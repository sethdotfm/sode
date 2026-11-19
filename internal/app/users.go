package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"github.com/sethdotfm/sode/internal/config"
)

// Users and page locks. Off until the config has users and a lock on. policy.go holds the
// message->need table; this file is the access state (who a connection signed in as) and
// the lock screen messages.

var errPageLocked = errors.New("locked; unlock with your PIN first")

type gate int

const (
	gateMain gate = iota
	gateLog
	gatePanel
)

func (l gate) on(k config.Locks) bool {
	switch l {
	case gateLog:
		return k.Log
	case gatePanel:
		return k.Panel
	}
	return k.Main
}

var pageGates = map[string]gate{"main": gateMain, "log": gateLog, "panel": gatePanel}

// connUser: page and sess guarded by App.mu; user is atomic.
type connUser struct {
	page string // "main" (default), "log" or "panel", as the page declared
	// sess: the token (from hi or issued at sign-in) and the Lock, shared with connections
	// on the same token. While locked the user stays set so only their PIN resumes it;
	// every access check treats a locked connection as signed in to nothing.
	sess *session
	user atomic.Pointer[config.User] // who it signed in as, with that user's PIN hash then
	// profile: last profile message sent (under mu); "" = none, so no clear is sent.
	profile string
	// rights: last myRights message sent (under mu); "" = none, so no clear is sent.
	rights string
	// offline: last panelsOffline message sent (under mu); "" = none.
	offline string
	recheck recheck // under mu (recheck.go)
	step    *stepIn // under mu; nil when not stepped in (stepin.go)
}

// userID is the signed-in user's id, or "".
func (c *client) userID() string {
	if u := c.u.user.Load(); u != nil {
		return u.ID
	}
	return ""
}

func (c *client) who() string {
	if c.api != nil {
		return c.api.Actor.String()
	}
	if u := c.u.user.Load(); u != nil {
		return u.Name
	}
	return c.ip
}

// userSameMu: signed in as a user who still exists with the same PIN, locked or not.
// Caller holds mu.
func (a *App) userSameMu(c *client, cfg config.Config) bool {
	was := c.u.user.Load()
	if was == nil {
		return false
	}
	u, ok := cfg.FindUserByID(was.ID)
	return ok && u.Pin == was.Pin
}

// userOKMu: signed in and not under the Lock. A user without a PIN opens no page lock, so under
// their page's lock they count as nobody. Caller holds mu.
func (a *App) userOKMu(c *client, cfg config.Config) bool {
	if c.u.lockedMu() || !a.userSameMu(c, cfg) {
		return false
	}
	return c.u.user.Load().HasPin() || !pageGates[cmp.Or(c.u.page, "main")].on(cfg.Locks)
}

// heldMu: the session's Lock is on and its user can still resume it. Caller holds mu.
func (a *App) heldMu(c *client, cfg config.Config) bool {
	return c.u.lockedMu() && a.userSameMu(c, cfg)
}

// endDeadHeldMu: a Lock with nobody left to resume it (the user was removed or changed
// their PIN) ends, so the connection signs out. Run where a config change applies. Caller
// holds mu. It sends nothing: the callers that change users or PINs (applyConfig,
// UserSetPin) sync every connection after, and other commits cannot change either.
func (a *App) endDeadHeldMu() {
	cfg := a.r().cfg
	for cl := range a.clients {
		if cl.u.lockedMu() && !a.userSameMu(cl, cfg) {
			cl.u.user.Store(nil)
			cl.u.sess = nil
		}
	}
}

// renameSignedInMu gives each signed-in connection its user's live name, so History after a
// rename shows the new one. Caller holds mu.
func (a *App) renameSignedInMu() {
	cfg := a.r().cfg
	for cl := range a.clients {
		was := cl.u.user.Load()
		if was == nil {
			continue
		}
		if u, ok := cfg.FindUserByID(was.ID); ok && u.Pin == was.Pin && u.Name != was.Name {
			cl.u.user.Store(&u)
		}
	}
}

// Caller holds mu.
func (a *App) pageAllowedMu(c *client, g gate) bool {
	cfg := a.r().cfg
	if a.heldMu(c, cfg) {
		return false
	}
	return !g.on(cfg.Locks) || a.userOKMu(c, cfg)
}

type lockStateMsg struct {
	T        string `json:"t"`
	Page     string `json:"page"`
	Enforced bool   `json:"enforced"` // this page type has a lock on
	Locked   bool   `json:"locked"`
	// Held: the Lock is the signed-in session's, held by the server. Carries no name: only
	// that user's PIN resumes it.
	Held bool   `json:"held,omitempty"`
	User string `json:"user,omitempty"`
	// Lens: PIN lengths in use; rides every lock state so a pad is never stale. Names and each
	// user's length ride the hello.
	Lens []int `json:"lens,omitempty"`
	// Hi: answer to the page's own hi; it is possible that an unsolicited state arrives
	// first, and it says nothing about the token.
	Hi bool `json:"hi,omitempty"`
	// Role: the Role this connection acts with ("" under a Lock); the server decides.
	Role string `json:"role,omitempty"`
	// Trusted: Role is Trusted user or higher; pages read it for the signed-in user.
	Trusted    bool `json:"trusted,omitempty"`
	Persistent bool `json:"persistent,omitempty"`
	// NoAutoLock: to the signed-in user only, so the page's idle timer skips them.
	NoAutoLock bool `json:"noAutoLock,omitempty"`
	// PinNeeded: to the signed-in user only, so the page asks them to set a PIN.
	PinNeeded bool `json:"pinNeeded,omitempty"`
	// StepIn: this connection is stepped in as User; StepGuest: Step out returns to Guest.
	StepIn    bool `json:"stepIn,omitempty"`
	StepGuest bool `json:"stepGuest,omitempty"`
}

// Caller holds mu.
func (a *App) lockStateMu(c *client) lockStateMsg {
	cfg := a.r().cfg
	page := c.u.page
	if page == "" {
		page = "main"
	}
	m := lockStateMsg{T: "lockState", Page: page, Enforced: pageGates[page].on(cfg.Locks)}
	m.Held = a.heldMu(c, cfg)
	if a.userOKMu(c, cfg) {
		u, _ := cfg.FindUserByID(c.u.user.Load().ID) // live flags, not the sign-in copy
		m.User, m.Persistent, m.NoAutoLock, m.PinNeeded = u.Name, u.Persistent, u.NoAutoLock, u.PinNeeded
	}
	m.Role = a.roleMu(c)
	m.Trusted = config.RoleAtLeast(m.Role, config.RoleTrusted)
	m.Locked = m.Held || m.Enforced && m.User == ""
	m.Lens = cfg.PinLens()
	if st := c.u.step; st != nil {
		m.StepIn, m.StepGuest = true, st.orig == nil
	}
	return m
}

// Queues c's lock state and stops the sent-log feed if c can no longer read it. Caller
// holds mu.
func (a *App) syncAccessMu(c *client) { a.pushAccessMu(c, false) }

// pushAccessMu: hi marks the answer to a hi.
func (a *App) pushAccessMu(c *client, hi bool) {
	if !a.pageAllowedMu(c, gateLog) {
		c.sent = false
	}
	m := a.lockStateMu(c)
	m.Hi = hi
	b, _ := json.Marshal(m)
	a.deliver(c, b)
	a.pushProfileMu(c) // only to the user it belongs to
	a.pushRightsMu(c)
	a.pushPanelsOfflineMu(c)
}

func (a *App) syncAccess(c *client) {
	a.mu.Lock()
	a.syncAccessMu(c)
	a.mu.Unlock()
}

// Hi: a valid token brings the connection up as that user; a bad one is ignored. An
// already signed-in connection is left alone.
func (a *App) Hi(c *client, page, token string) {
	if _, ok := pageGates[page]; !ok {
		page = "main"
	}
	a.mu.Lock()
	cfg := a.r().cfg
	c.u.page = page
	u, ok := a.tokens.user(token, cfg)
	if ok && heldOnly(u, page, cfg.Locks) && !a.tokens.locked(token) {
		ok = false
	}
	held := ""
	if ok && c.u.user.Load() == nil {
		c.u.user.Store(&u)
		a.adoptMu(c, token, a.tokens.locked(token))
		// A browser back after the idle limit meets the Lock even before the next sweep.
		if a.overdueMu(c, cfg, a.now()) {
			held, _ = a.holdMu(c, true)
		}
	}
	a.pushAccessMu(c, true)
	a.mu.Unlock()
	if held != "" {
		a.tokens.save()
	}
	if ok {
		a.tokens.touch(cfg, token) // a used token's age starts over
	}
}

// dummyPinHash gives a wrong name the same PBKDF2 cost as a right one, so timing
// cannot reveal which names exist.
var dummyPinHash = sync.OnceValue(func() string {
	h, _ := config.HashPin("0000")
	return h
})

func burnPinCheck(pin string) { config.CheckPin(dummyPinHash(), pin) }

// pinCandidate is the one user a PIN can sign in, or nil. Pure: it reads only cfg. It is the
// holder of a Lock (an empty name, or the holder's own), else the named user.
func pinCandidate(cfg config.Config, holder *config.User, name string) *config.User {
	var cand config.User
	ok := false
	if holder != nil {
		if named, found := cfg.FindUserByName(name); name == "" || found && named.ID == holder.ID {
			cand, ok = cfg.FindUserByID(holder.ID) // the live hash, not the sign-in copy
		}
	} else if name != "" {
		cand, ok = cfg.FindUserByName(name)
	}
	if !ok {
		return nil
	}
	return &cand
}

// pinMatches: no candidate burns a check so it costs what a right one does.
func pinMatches(cand *config.User, pin string) bool {
	if cand == nil {
		burnPinCheck(pin)
		return false
	}
	if !cand.HasPin() { // public in the hello (len 0), so there is nothing to hide by hashing
		return pin == ""
	}
	// Hashed whatever the length: PIN lengths are public, so skipping it would tell a known name
	// from an unknown one.
	return config.CheckPin(cand.Pin, pin) && cand.Len == utf8.RuneCountInString(pin)
}

// UserUnlockAs compares the PIN with the named user's hash only. A held Lock resumes with its
// own user's PIN, and a name there must be that user's. Every try counts toward that name's wait.
func (a *App) UserUnlockAs(c *client, name, pin string) error {
	cfg := a.r().cfg
	a.mu.Lock()
	if c.u.step != nil {
		a.mu.Unlock()
		return errors.New("you are stepped in: step out first")
	}
	var holder *config.User
	if c.u.lockedMu() {
		holder = c.u.user.Load()
	}
	a.mu.Unlock()
	hit := pinCandidate(cfg, holder, name)
	counts := hit
	if holder != nil {
		counts = holder // a held Lock's PIN pad is its holder's, whatever name is typed
	}
	err := a.tryUserPin(counts, c.ip, func() bool { return pinMatches(hit, pin) })
	if err != nil {
		return err
	}
	a.mu.Lock()
	cfg = a.r().cfg // the PIN check can wait minutes: it is possible that the user changed since
	if u, ok := cfg.FindUserByID(hit.ID); !ok || u.Pin != hit.Pin {
		a.mu.Unlock()
		return errors.New("that PIN changed; try again")
	}
	held := c.u.user.Load()
	if c.u.lockedMu() && held != nil {
		if held.ID != hit.ID {
			// Locked by another tab while the PIN was being checked: only the holder resumes it.
			a.mu.Unlock()
			return errors.New("wrong PIN")
		}
		tok, drop := a.holdMu(c, false)
		a.mu.Unlock()
		a.finishHold(c, false, tok, drop)
		return nil
	}
	defer a.mu.Unlock()
	if !hit.HasPin() && pageGates[cmp.Or(c.u.page, "main")].on(cfg.Locks) {
		return fmt.Errorf("%s has no PIN, and this page's lock needs a user with a PIN", hit.Name)
	}
	// A token-X connection signing in as Y leaves X's session, or Y's Lock would end X's.
	c.u.sess = &session{seen: a.now()}
	c.u.recheck = recheck{}
	c.u.user.Store(hit)
	a.syncAccessMu(c)
	return nil
}

// SignOutAll needs an Administrator.
func (a *App) SignOutAll(c *client, name string) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, config.RoleAdministrator); err != nil {
		return err
	}
	who, ok := a.r().cfg.FindUserByName(name)
	if !ok {
		return errors.New("no such user")
	}
	a.tokens.dropUser(who.ID)
	a.endSessionKeys()
	a.mu.Lock()
	for cl := range a.clients {
		if st := cl.u.step; st != nil && st.orig != nil && st.orig.ID == who.ID {
			st.ended = true
		}
		if st := cl.u.step; st != nil && cl.userID() == who.ID {
			a.stepOutMu(cl, "signOut")
		}
	}
	for cl := range a.clients {
		if u := cl.u.user.Load(); u != nil && u.ID == who.ID {
			a.signOutMu(cl)
		}
	}
	a.mu.Unlock()
	return nil
}

// userNameRefusal is the clear refusal for a Name the config would reject (Validate has the
// same rules, for files).
func userNameRefusal(name string) error {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return errors.New("every user needs a name")
	case n > config.MaxUserName:
		return fmt.Errorf("the name %q is too long: at most %d characters", name, config.MaxUserName)
	case strings.IndexFunc(name, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0:
		return fmt.Errorf("the name %q has a character that cannot be shown", name)
	}
	return nil
}

// panelsFollowUsers keeps panel sharing (by name) in step with a saved Users list, and drops a
// removed user's watches. Owners are ids, so a rename leaves them.
func (a *App) panelsFollowUsers(before, after []config.User) {
	renames := map[string]string{}
	removedIDs, removedNames := map[string]bool{}, map[string]bool{}
	now := config.Config{Users: after}
	for _, u := range before {
		nu, ok := now.FindUserByID(u.ID)
		switch {
		case !ok:
			removedIDs[u.ID], removedNames[strings.ToLower(u.Name)] = true, true
		case nu.Name != u.Name:
			renames[strings.ToLower(u.Name)] = nu.Name
		}
	}
	if len(renames) == 0 && len(removedIDs) == 0 {
		return
	}
	a.mon.dropUsers(removedIDs)
	n, err := a.panels.followUsers(renames, removedNames)
	if err != nil {
		log.Printf("panels: user change reached %d panels, then: %v", n, err)
		return
	}
	if n > 0 {
		log.Printf("panels: user change updated %d panels", n)
	}
}

type userResult struct {
	T     string `json:"t"`
	OK    bool   `json:"ok"`
	ID    int    `json:"id"`
	Err   string `json:"err,omitempty"`
	Token string `json:"token,omitempty"`
	Tab   bool   `json:"tab,omitempty"` // Token is a sign-in without a page lock's: for this tab only
}

// Sign-ins run inline, so a later lock is handled after.
func (a *App) serveUserUnlock(c *client, m *inMsg) {
	res := userResult{T: "userSignInResult", OK: true, ID: m.ID}
	err := a.UserUnlockAs(c, m.User, m.Pin)
	if err != nil {
		res.OK, res.Err = false, err.Error()
		a.command(c, "userSignIn", "refused", "", err)
	} else {
		a.command(c, "userSignIn", "ok", "", nil)
		if res.Token = a.issueToken(c, m.Tab); res.Token != "" {
			if us, _ := a.r().cfg.FindUserByID(c.userID()); !us.Persistent {
				res.Tab = true
			}
			a.mu.Lock()
			a.syncAccessMu(c) // after the token, so the page's state is final
			a.mu.Unlock()
		}
	}
	a.reply(c, res)
}

func (a *App) serveUserSetPin(c *client, m *inMsg) {
	set := a.UserSetPin
	if m.Remove {
		set = func(c *client, old, _ string) error { return a.RemoveOwnPin(c, old) }
	}
	err := set(c, m.Old, m.New)
	if err != nil {
		a.command(c, "userSetPin", "refused", "", err)
	} else {
		a.command(c, "userSetPin", "ok", "", nil)
	}
	a.reply(c, opResultFor("userSetPin", &m.ID, err))
}

// Len: whether a PIN is set (0 = none); never a hash.
type usersListMsg struct {
	T     string     `json:"t"` // "usersListResult"
	ID    int        `json:"id"`
	OK    bool       `json:"ok"`
	Users []userView `json:"users"`
	// Me is the id of the user asking, the default new owner of a removed user's panels.
	Me string `json:"me"`
	// Rights and API clients: Administrator only, never before the PIN.
	APIClients  []config.APIClient `json:"apiClients"`
	GuestRights config.Rights      `json:"guestRights"`
	GuestScopes []string           `json:"guestScopes"`
	// Versions of the one Save's fields that never ride the hello (hiddenVersions).
	Versions map[string]string `json:"versions"`
}

type userView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Len        int    `json:"len"`
	Role       string `json:"role"`
	Persistent bool   `json:"persistent,omitempty"`
	NoAutoLock bool   `json:"noAutoLock,omitempty"`
	PinNeeded  bool   `json:"pinNeeded,omitempty"`
	// Rights has no omitempty: an unset list is "all", an empty one "none".
	Rights config.Rights `json:"rights"`
	// PanelClients are the panel clients this user's panels have, for the Rights editor to list.
	PanelClients []panelClientNote `json:"panelClients"`
	// Presets are the user's Me presets, so the Rights editor can turn their names into ids.
	Presets []presetNote `json:"presets"`
	// Panels: how many panels the user owns, so removing them asks for a new owner.
	Panels int `json:"panels,omitempty"`
	// Clients: the API clients the user owns, which go with them (dropOwnerless).
	Clients []string `json:"clients,omitempty"`
}

type presetNote struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (a *App) usersList(c *client, id int) usersListMsg {
	cfg := a.r().cfg.Clone()
	m := usersListMsg{T: "usersListResult", ID: id, OK: true, Me: c.userID(), Users: []userView{},
		APIClients: cfg.APIClients, GuestRights: cfg.GuestRights, GuestScopes: cfg.GuestAPIScopes, Versions: hiddenVersions(cfg)}
	if m.APIClients == nil {
		m.APIClients = []config.APIClient{}
	}
	for i := range m.APIClients {
		m.APIClients[i].OwnerName = cfg.UserName(m.APIClients[i].Owner)
	}
	for _, u := range cfg.Users {
		pc := a.panelClientsOf(cfg, u)
		if pc == nil {
			pc = []panelClientNote{}
		}
		ps := []presetNote{}
		for _, p := range u.Profile.Presets {
			ps = append(ps, presetNote{p.ID, p.Name})
		}
		var own []string
		for _, cl := range cfg.APIClients {
			if cl.Owner == u.ID {
				own = append(own, cl.Name)
			}
		}
		m.Users = append(m.Users, userView{ID: u.ID, Name: u.Name, Len: u.Len, Role: u.Role, Persistent: u.Persistent,
			NoAutoLock: u.NoAutoLock, PinNeeded: u.PinNeeded, Rights: u.Rights, PanelClients: pc, Presets: ps, Panels: a.panels.ownedBy(u.ID),
			Clients: own})
	}
	return m
}

// UserSetPin sets or changes c's own PIN. A user with a PIN gives it as old, a try in the users'
// pool; a user without one gives none. Other connections of the user relock; c stays signed in.
func (a *App) UserSetPin(c *client, oldPin, newPin string) error {
	return a.setOwnPin(c, oldPin, newPin, false)
}

// RemoveOwnPin: only a Standard user can have no PIN (config.NoPinRefusal). Their personal keys
// end with the hash. Their own API clients go in the same commit (dropOwnerless).
func (a *App) RemoveOwnPin(c *client, oldPin string) error {
	return a.setOwnPin(c, oldPin, "", true)
}

func (a *App) setOwnPin(c *client, oldPin, newPin string, remove bool) error {
	who := c.u.user.Load()
	if who == nil {
		return errors.New("unlock with your PIN first")
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	a.mu.Lock()
	cur := a.r().cfg
	ok := a.userOKMu(c, cur)
	a.mu.Unlock()
	if !ok {
		return errors.New("unlock with your PIN first")
	}
	u, _ := cur.FindUserByID(who.ID)
	switch {
	case remove && !u.HasPin():
		return errors.New("you have no PIN to remove")
	case remove && u.Role != config.RoleStandard:
		return fmt.Errorf("only a Standard user can remove their PIN; you are %s", config.RoleLabel(u.Role))
	case !remove:
		if err := config.CheckUserPinRules(newPin, cur.MaxPinLen); err != nil {
			return err
		}
	}
	if u.HasPin() {
		if err := a.tryUserPin(&u, c.ip, func() bool { return config.CheckPin(u.Pin, oldPin) }); err != nil {
			return err
		}
		if !remove && oldPin == newPin {
			return errors.New("the new PIN is the same as the old one")
		}
	}
	h := ""
	if !remove {
		var err error
		if h, err = config.HashPin(newPin); err != nil {
			return err
		}
	}
	page := a.readHello() // the hello's PIN length changes; history read stays outside mu
	err := a.commit(func(cfg *config.Config) error {
		for i := range cfg.Users {
			if cfg.Users[i].ID != who.ID {
				continue
			}
			if cfg.Users[i].Pin != u.Pin {
				return errors.New("your PIN changed; unlock again")
			}
			cfg.Users[i].Pin, cfg.Users[i].Len, cfg.Users[i].PinNeeded = h, len(newPin), false
			return config.NoPinRefusal(cfg.Users[i])
		}
		return errors.New("unlock with your PIN first")
	}, func(_, nr *runtime) {
		if nu, ok := nr.cfg.FindUserByID(who.ID); ok && c.userID() == who.ID {
			c.u.user.Store(&nu)
		}
		if u.Len != len(newPin) { // the hello carries each user's PIN length
			hello := a.hello(page)
			for cl := range a.clients {
				a.deliver(cl, hello)
			}
		}
		a.syncAccessAllMu() // the user's other connections relock
	})
	if err != nil {
		return err
	}
	a.catchUpSessions() // c's old tokens died with the old hash
	return nil
}

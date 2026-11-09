package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"log"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/sethdotfm/sode/internal/config"
)

// Users and page locks. Off until the config has users and a lock on. policy.go holds the
// message->need table; this file is the access state (who a connection signed in as) and
// the lock screen messages.

var errPageLocked = errors.New("locked; unlock with your PIN first")

var errNeedAdminPin = errors.New("Set an Advanced Settings PIN first: users and locks are administered with it")

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
	u, ok := cfg.FindUser(was.Name)
	return ok && u.Pin == was.Pin
}

// userOKMu: signed in and not under the Lock. Caller holds mu.
func (a *App) userOKMu(c *client, cfg config.Config) bool {
	return !c.u.lockedMu() && a.userSameMu(c, cfg)
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
	// Mode: sign-in mode, so the sign-in screen knows whether to ask for a username.
	Mode string `json:"mode"`
	// Lens: PIN lengths in use; rides every lock state so a pad is never stale.
	Lens []int `json:"lens,omitempty"`
	// Names: for "by Ana" in history; not sent to locked pages.
	Names []string `json:"names,omitempty"`
	// Hi: answer to the page's own hi; an unsolicited state may arrive first and says
	// nothing about the token.
	Hi bool `json:"hi,omitempty"`
	// Trusted: low-risk settings without the Advanced Settings PIN; the server decides.
	Trusted    bool `json:"trusted,omitempty"`
	Persistent bool `json:"persistent,omitempty"`
	// NoAutoLock: to the signed-in user only, so the page's idle timer skips them.
	NoAutoLock bool `json:"noAutoLock,omitempty"`
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
	m.Mode = cfg.SignInMode
	if a.userOKMu(c, cfg) {
		u, _ := cfg.FindUser(c.u.user.Load().Name) // live flags, not the sign-in copy
		m.User, m.Trusted, m.Persistent, m.NoAutoLock = u.Name, u.Trusted, u.Persistent, u.NoAutoLock
	}
	m.Locked = m.Held || m.Enforced && m.User == ""
	m.Lens = cfg.PinLens()
	if !m.Locked {
		for _, u := range cfg.Users {
			m.Names = append(m.Names, u.Name)
		}
	}
	return m
}

// Queues c's lock state and stops the sent-log feed if c may no longer read it. Caller
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
	if ok && c.u.user.Load() == nil {
		c.u.user.Store(&u)
		a.adoptMu(c, token, a.tokens.locked(token))
	}
	a.pushAccessMu(c, true)
	a.mu.Unlock()
	if ok {
		a.tokens.touch(cfg, token) // a used token's age starts over
	}
}

// UserUnlock signs in by PIN alone (PIN only mode).
func (a *App) UserUnlock(c *client, pin string) error { return a.UserUnlockAs(c, "", pin) }

// dummyPinHash gives a wrong username the same PBKDF2 cost as a right one, so timing
// can't reveal which usernames exist.
var dummyPinHash = sync.OnceValue(func() string {
	h, _ := config.HashPin("0000")
	return h
})

func burnPinCheck(pin string) { config.CheckPin(dummyPinHash(), pin) }

// matchPin finds the user a PIN signs in, or nil. Pure: it reads only cfg. Candidates are
// the holder of a Lock (only their PIN counts), the user a username names, or in PIN only
// mode everyone; exactly one must match, so a PIN several users hold (left over from
// Username and PIN mode) signs nobody in. A username that names nobody, or someone other
// than the holder, burns a check so it costs what a right one does.
func matchPin(cfg config.Config, holder *config.User, username, pin string) *config.User {
	byName := cfg.SignInMode == config.SignInUsername
	var cands []config.User
	switch {
	case holder != nil:
		if named, ok := cfg.FindUsername(username); byName && username != "" && (!ok || named.Name != holder.Name) {
			break
		}
		if u, ok := cfg.FindUser(holder.Name); ok { // the live hash, not the sign-in copy
			cands = []config.User{u}
		}
	case byName:
		if u, ok := cfg.FindUsername(username); ok {
			cands = []config.User{u}
		}
	default:
		cands = cfg.Users
	}
	if len(cands) == 0 {
		burnPinCheck(pin)
		return nil
	}
	n := utf8.RuneCountInString(pin)
	var found []config.User
	for _, u := range cands {
		// Username mode hashes whatever the length: PIN lengths are public, so skipping it
		// would tell a known username from an unknown one.
		if !byName && u.Len != n {
			continue
		}
		if config.CheckPin(u.Pin, pin) && u.Len == n {
			found = append(found, u)
		}
	}
	if len(found) != 1 {
		return nil
	}
	return &found[0]
}

// UserUnlockAs: PIN only mode ignores username and finds the user by PIN. Username and PIN
// mode compares the PIN with the named user's hash only. A held Lock resumes with its own
// user's PIN whatever the mode, and a username there must be that user's. Every try
// counts in the users' pool, like an Advanced Settings PIN.
func (a *App) UserUnlockAs(c *client, username, pin string) error {
	cfg := a.r().cfg
	a.mu.Lock()
	var holder *config.User
	if c.u.lockedMu() {
		holder = c.u.user.Load()
	}
	a.mu.Unlock()
	var hit *config.User
	err := a.tryPin(&a.userPin, c.ip, func() bool {
		hit = matchPin(cfg, holder, username, pin)
		return hit != nil
	})
	if err != nil {
		return err
	}
	a.mu.Lock()
	cfg = a.r().cfg // the PIN check can wait minutes: the user may have changed since
	if u, ok := cfg.FindUser(hit.Name); !ok || u.Pin != hit.Pin {
		a.mu.Unlock()
		return errors.New("that PIN changed; try again")
	}
	held := c.u.user.Load()
	if c.u.lockedMu() && held != nil {
		if held.Name != hit.Name {
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
	// A token-X connection signing in as Y leaves X's session, or Y's Lock would end X's.
	c.u.sess = &session{}
	c.u.user.Store(hit)
	a.syncAccessMu(c)
	return nil
}

// SignOutAll needs the Advanced Settings PIN (a trusted user is not enough).
func (a *App) SignOutAll(c *client, name string) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, false); err != nil {
		return err
	}
	if _, ok := a.r().cfg.FindUser(name); !ok {
		return errors.New("no such user")
	}
	a.tokens.dropUser(name)
	a.endSessionKeys()
	a.mu.Lock()
	for cl := range a.clients {
		if u := cl.u.user.Load(); u != nil && strings.EqualFold(u.Name, name) {
			a.signOutMu(cl)
		}
	}
	a.mu.Unlock()
	return nil
}

// backToPinRefusal: salted hashes hide whether saved users share a PIN, so going back to PIN only
// needs a new PIN typed for every user, all different. Compares plaintext within the request.
func backToPinRefusal(users []userEdit) error {
	var missing []string
	byPin := map[string][]string{}
	for _, e := range users {
		name := strings.TrimSpace(e.Name)
		if e.Pin == "" {
			missing = append(missing, name)
		} else {
			byPin[e.Pin] = append(byPin[e.Pin], name)
		}
	}
	if len(missing) > 0 {
		return errors.New("PIN only needs every PIN different, and Sode can't tell whether saved PINs match: give a new PIN to " + strings.Join(missing, ", "))
	}
	var clash []string
	for _, e := range users {
		if names := byPin[e.Pin]; len(names) > 1 {
			clash = append(clash, strings.TrimSpace(e.Name))
		}
	}
	if len(clash) > 0 {
		return errors.New("PIN only needs every PIN different, and these users got the same new PIN: " + strings.Join(clash, ", "))
	}
	return nil
}

// userEdit: Was is the old name (rename keeps the PIN), Pin a new PIN ("" keeps).
type userEdit struct {
	Name string `json:"name"`
	Was  string `json:"was"`
	// Username: nil keeps the user's current one.
	Username   *string `json:"username"`
	Pin        string  `json:"pin"`
	Trusted    bool    `json:"trusted"`
	Persistent bool    `json:"persistent"`
	// Rights and NoAutoLock: nil keeps the user's current value, so a page that doesn't
	// know them can't erase them.
	Rights     *config.Rights `json:"rights"`
	NoAutoLock *bool          `json:"noAutoLock"`
}

type usersReq struct {
	Users  []userEdit `json:"users"`
	MaxPin int        `json:"maxPinLen"` // 0 keeps the current one
	// SignInMode: "" keeps the current one. Saved here so it needs
	// the Advanced Settings PIN.
	SignInMode string `json:"signInMode"`
	// API clients, Guest rights and Guest scopes: nil keeps the current ones. They ride this
	// save so they need the Advanced Settings PIN.
	APIClients  *[]config.APIClient `json:"apiClients"`
	GuestRights *config.Rights      `json:"guestRights"`
	GuestScopes *[]string           `json:"guestScopes"`
	// Locks and AutoLock are read only to refuse them: a stale page must not look saved.
	Locks    json.RawMessage `json:"locks"`
	AutoLock json.RawMessage `json:"autoLockMinutes"`
}

// SaveUsers needs the Advanced Settings PIN only (not the main lock). PINs unique across users.
func (a *App) SaveUsers(c *client, req usersReq) error {
	if present(req.Locks) || present(req.AutoLock) {
		return errors.New("the lock settings moved to Settings > Access: reload this page")
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.r().cfg.SettingsPin == "" {
		return errNeedAdminPin
	}
	if err := a.guardSave(c, false); err != nil {
		return err
	}
	before := a.r().cfg
	err := a.applyConfig(func(cur *config.Config) error {
		if req.MaxPin != 0 {
			cur.MaxPinLen = req.MaxPin
		}
		maxLen := cur.MaxPinLen
		wasUsername := cur.SignInMode == config.SignInUsername
		if req.SignInMode != "" {
			cur.SignInMode = req.SignInMode
		}
		if len(req.Users) == 0 && cur.Locks.Any() {
			return errors.New("a screen lock is on: turn it off in Settings > Access (and save) before removing the last user")
		}
		old := map[string]config.User{}
		for _, u := range cur.Users {
			old[strings.ToLower(u.Name)] = u
		}
		var out []config.User
		inherited := map[string]bool{} // rows keep a hash only from their own, one row each
		fresh := map[string]string{}   // new PINs by user name
		kept := map[string]bool{}      // profiles carried over, one row each
		for _, e := range req.Users {
			u := config.User{Name: strings.TrimSpace(e.Name), Trusted: e.Trusted, Persistent: e.Persistent}
			if u.Name == "" {
				return errors.New("every user needs a name")
			}
			// The profile follows Was (rename keeps it); removed users lose theirs; a new
			// row starts empty.
			was := cmp.Or(e.Was, e.Name)
			pk := strings.ToLower(strings.TrimSpace(was))
			if o, ok := old[pk]; ok && !kept[pk] && (e.Was != "" || e.Pin == "") {
				kept[pk] = true
				u.Profile = o.Profile.Clone()
				u.Username = o.Username
				u.Rights, u.NoAutoLock = o.Rights.Clone(), o.NoAutoLock
			}
			if e.Username != nil {
				u.Username = strings.TrimSpace(*e.Username)
			}
			if e.Rights != nil {
				u.Rights = e.Rights.Clone()
			}
			if e.NoAutoLock != nil {
				u.NoAutoLock = *e.NoAutoLock
			}
			if e.Pin != "" {
				if err := config.CheckUserPinRules(e.Pin, maxLen); err != nil {
					return err
				}
				h, err := config.HashPin(e.Pin)
				if err != nil {
					return err
				}
				u.Pin, u.Len = h, len(e.Pin)
				fresh[u.Name] = e.Pin
			} else {
				key := pk
				o, ok := old[key]
				if !ok {
					return errors.New("give " + u.Name + " a PIN")
				}
				if inherited[key] {
					return errors.New("two rows keep the same user's PIN; give one of them a new PIN")
				}
				inherited[key] = true
				u.Pin, u.Len = o.Pin, o.Len
			}
			out = append(out, u)
		}
		if wasUsername && cur.SignInMode == config.SignInPin {
			if err := backToPinRefusal(req.Users); err != nil {
				return err
			}
		}
		for name, pin := range fresh {
			if cur.SignInMode == config.SignInUsername {
				break // the username says whose PIN it is
			}
			for _, u := range out {
				if u.Name == name || u.Len != len(pin) {
					continue
				}
				if p2, isNew := fresh[u.Name]; isNew && p2 == pin || !isNew && config.CheckPin(u.Pin, pin) {
					return errors.New("that PIN can't be used")
				}
			}
		}
		cur.Users = out
		if req.APIClients != nil {
			cur.APIClients = slices.Clone(*req.APIClients)
			for i := range cur.APIClients {
				cur.APIClients[i].Rights = cur.APIClients[i].Rights.Clone()
			}
		}
		if req.GuestRights != nil {
			cur.GuestRights = req.GuestRights.Clone()
		}
		if req.GuestScopes != nil {
			cur.GuestAPIScopes = slices.Clone(*req.GuestScopes)
		}
		return nil
	})
	if err == nil {
		a.auditRights(c, before, a.r().cfg)
		a.panelsFollowUsers(before.Users, req.Users)
	}
	return err
}

// panelsFollowUsers keeps panel owner and sharing in step with a saved Users list. Rows keep
// a user by Was (or Name), like profiles above.
func (a *App) panelsFollowUsers(before []config.User, rows []userEdit) {
	renames := map[string]string{}
	kept := map[string]bool{}
	for _, e := range rows {
		name := strings.TrimSpace(e.Name)
		pk := strings.ToLower(strings.TrimSpace(cmp.Or(e.Was, e.Name)))
		kept[pk] = true
		if e.Was != "" && strings.TrimSpace(e.Was) != name {
			renames[pk] = name
		}
	}
	removed := map[string]bool{}
	for _, u := range before {
		if k := strings.ToLower(u.Name); !kept[k] {
			removed[k] = true
		}
	}
	if len(renames) == 0 && len(removed) == 0 {
		return
	}
	n, err := a.panels.followUsers(renames, removed)
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

// Sign-ins run inline like the Advanced Settings PIN entry, so a later lock is handled after.
func (a *App) serveUserUnlock(c *client, m *inMsg) {
	res := userResult{T: "userUnlockResult", OK: true, ID: m.ID}
	err := a.UserUnlockAs(c, m.User, m.Pin)
	if err != nil {
		res.OK, res.Err = false, err.Error()
		a.command(c, "userUnlock", "refused", "", err)
	} else {
		a.command(c, "userUnlock", "ok", "", nil)
		if res.Token = a.issueToken(c, m.Tab); res.Token != "" {
			if us, _ := a.r().cfg.FindUser(c.who()); !us.Persistent {
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
	err := a.UserSetPin(c, m.Old, m.New)
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
	Mode  string     `json:"signInMode"`
	// Rights and API clients: needAdmin only, never before the PIN.
	APIClients  []config.APIClient `json:"apiClients"`
	GuestRights config.Rights      `json:"guestRights"`
	GuestScopes []string           `json:"guestScopes"`
}

type userView struct {
	Name       string `json:"name"`
	Username   string `json:"username,omitempty"`
	Len        int    `json:"len"`
	Trusted    bool   `json:"trusted,omitempty"`
	Persistent bool   `json:"persistent,omitempty"`
	NoAutoLock bool   `json:"noAutoLock,omitempty"`
	// Rights has no omitempty: an unset list is "all", an empty one "none".
	Rights config.Rights `json:"rights"`
}

func (a *App) usersList(id int) usersListMsg {
	cfg := a.r().cfg.Clone()
	m := usersListMsg{T: "usersListResult", ID: id, OK: true, Users: []userView{}, Mode: cfg.SignInMode,
		APIClients: cfg.APIClients, GuestRights: cfg.GuestRights, GuestScopes: cfg.GuestAPIScopes}
	if m.APIClients == nil {
		m.APIClients = []config.APIClient{}
	}
	for _, u := range cfg.Users {
		m.Users = append(m.Users, userView{u.Name, u.Username, u.Len, u.Trusted, u.Persistent, u.NoAutoLock, u.Rights})
	}
	return m
}

// UserSetPin needs the user's own PIN, not the Advanced Settings PIN; the old PIN counts as a try
// (same pool). Other connections of the user relock; c stays signed in.
func (a *App) UserSetPin(c *client, oldPin, newPin string) error {
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
	u, _ := cur.FindUser(who.Name)
	if err := config.CheckUserPinRules(newPin, cur.MaxPinLen); err != nil {
		return err
	}
	if err := a.tryPin(&a.userPin, c.ip, func() bool { return config.CheckPin(u.Pin, oldPin) }); err != nil {
		return err
	}
	if oldPin == newPin {
		return errors.New("the new PIN is the same as the old one")
	}
	h, err := config.HashPin(newPin)
	if err != nil {
		return err
	}
	err = a.commit(func(cfg *config.Config) error {
		for i := range cfg.Users {
			if !strings.EqualFold(cfg.Users[i].Name, who.Name) {
				continue
			}
			if cfg.Users[i].Pin != u.Pin {
				return errors.New("your PIN changed; unlock again")
			}
			for j, o := range cfg.Users {
				if cfg.SignInMode == config.SignInPin && j != i && o.Len == len(newPin) && config.CheckPin(o.Pin, newPin) {
					return errors.New("that PIN can't be used")
				}
			}
			cfg.Users[i].Pin, cfg.Users[i].Len = h, len(newPin)
			return nil
		}
		return errors.New("unlock with your PIN first")
	}, func(_, nr *runtime) {
		if nu, ok := nr.cfg.FindUser(who.Name); ok {
			c.u.user.Store(&nu)
		}
		a.syncAccessAllMu() // the user's other connections relock
	})
	if err != nil {
		return err
	}
	a.catchUpSessions() // c's old tokens died with the old hash
	return nil
}

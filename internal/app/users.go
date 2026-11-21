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
	// inbox: an inbox message went out (under mu), so losing the user sends an empty one.
	inbox   bool
	recheck recheck // under mu (recheck.go)
	step    *stepIn // under mu; nil when not stepped in (stepin.go)
	// cards: this page shows Approval cards (approvalCards); approvers: the last approvers
	// message sent. Both under mu (approvalrequest.go).
	cards     bool
	approvers string
}

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

// guestOpenMu: guests can use c's page, since its page lock is off. Caller holds mu.
func guestOpenMu(c *client, cfg config.Config) bool {
	return !pageGates[cmp.Or(c.u.page, "main")].on(cfg.Locks)
}

// actsAs: u on c's page. Where guests can use the page, signing in never gives less than a guest has
// (config.ActsWithGuest). open is guestOpenMu.
func actsAs(cfg config.Config, u config.User, open bool) (string, config.Rights) {
	if open {
		return cfg.ActsWithGuest(u)
	}
	return cfg.ActsAs(u)
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

// History after a rename shows the new name. Caller holds mu.
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
	// Lens: PIN lengths in use, in every lock state, so a pad is never stale.
	Lens []int `json:"lens,omitempty"`
	// Hi: answer to the page's own hi; it is possible that an unsolicited state arrives
	// first, and it says nothing about the token.
	Hi bool `json:"hi,omitempty"`
	// "" under a Lock.
	Role       string `json:"role,omitempty"`
	Trusted    bool   `json:"trusted,omitempty"`
	Persistent bool   `json:"persistent,omitempty"`
	// NoAutoLock: to the signed-in user only, so the page's idle timer skips them.
	NoAutoLock bool `json:"noAutoLock,omitempty"`
	// PinNeeded: to the signed-in user only, so the page asks them to set a PIN.
	PinNeeded bool `json:"pinNeeded,omitempty"`
	// StepGuest: Step out returns to Guest.
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

// Also stops the Communication Log feed if c cannot read it. Caller holds mu.
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
	a.cardsSyncMu()
	a.pushInboxMu(c, nil)
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

// pinCandidate: the holder of a Lock (an empty name, or the holder's own), else the named user. Pure.
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

// With no candidate it still hashes, so timing matches a real check.
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

// A held Lock resumes only with its holder's PIN. Every try counts toward that name's wait.
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
	// A held Lock's PIN pad is its holder's, whatever name is typed.
	if err := a.provePin(c, holder, hit, pin); err != nil {
		return err
	}
	a.mu.Lock()
	cfg = a.r().cfg
	if _, ok := a.livePinMu(hit); !ok {
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

// Keep in step with config Validate's name rules.
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
	a.inboxFollowUsers(removedIDs)
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

// Sign-ins run inline, so the hub handles a later lock after this one.
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

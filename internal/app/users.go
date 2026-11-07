package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/sethdotfm/sode/internal/config"
)

// Users and page locks. Off until the config has users and a lock on. policy.go holds the
// message->need table; this file is the access state (who a connection unlocked as) and
// the lock screen messages.

var errPageLocked = errors.New("locked; unlock with your PIN first")

var errNeedAdminPin = errors.New("Set a settings PIN first: users and locks are administered with it")

type gate int

const (
	gateMain gate = iota
	gateLog
)

func (l gate) on(k config.Locks) bool {
	switch l {
	case gateLog:
		return k.Log
	}
	return k.Main
}

var pageGates = map[string]gate{"main": gateMain, "log": gateLog}

// connUser: page and token guarded by App.mu; user is atomic.
type connUser struct {
	page string // "main" (default) or "log", as the page declared
	// token: session token held (from hi or issued on unlock); persistent users only; a
	// lock revokes it.
	token string
	user  atomic.Pointer[config.User] // who it unlocked as, with that user's PIN hash then
	// locked: the session's Lock (under mu). The user stays set so only their PIN resumes
	// it; every access check treats a locked connection as signed in to nothing.
	locked bool
	// profile: last profile message sent (under mu); "" = none, so no clear is sent.
	profile string
}

func (c *client) who() string {
	if u := c.u.user.Load(); u != nil {
		return u.Name
	}
	return c.ip
}

// userSameLocked: signed in as a user who still exists with the same PIN, locked or not.
// Caller holds mu.
func (a *App) userSameLocked(c *client, cfg config.Config) bool {
	was := c.u.user.Load()
	if was == nil {
		return false
	}
	u, ok := cfg.FindUser(was.Name)
	return ok && u.Pin == was.Pin
}

// userOKLocked: signed in and not under the Lock. Caller holds mu.
func (a *App) userOKLocked(c *client, cfg config.Config) bool {
	return !c.u.locked && a.userSameLocked(c, cfg)
}

// heldLocked: the session's Lock is on and its user can still resume it. Caller holds mu.
func (a *App) heldLocked(c *client, cfg config.Config) bool {
	return c.u.locked && a.userSameLocked(c, cfg)
}

// Caller holds mu.
func (a *App) pageAllowedLocked(c *client, g gate) bool {
	cfg := a.r().cfg
	if a.heldLocked(c, cfg) {
		return false
	}
	return !g.on(cfg.Locks) || a.userOKLocked(c, cfg)
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
	// Lens: PIN lengths in use; rides every lock state so a pad is never stale.
	Lens []int `json:"lens,omitempty"`
	// Names: for "by Ana" in history; not sent to locked pages.
	Names []string `json:"names,omitempty"`
	// Hi: answer to the page's own hi; an unsolicited state may arrive first and says
	// nothing about the token.
	Hi bool `json:"hi,omitempty"`
	// Trusted: low-risk settings without the settings PIN; the server decides.
	Trusted    bool `json:"trusted,omitempty"`
	Persistent bool `json:"persistent,omitempty"`
}

// Caller holds mu.
func (a *App) lockStateLocked(c *client) lockStateMsg {
	cfg := a.r().cfg
	page := c.u.page
	if page == "" {
		page = "main"
	}
	if c.u.locked && !a.userSameLocked(c, cfg) {
		// Nobody left to resume it: the user was removed or changed their PIN.
		c.u.user.Store(nil)
		c.u.locked = false
	}
	m := lockStateMsg{T: "lockState", Page: page, Enforced: pageGates[page].on(cfg.Locks)}
	m.Held = c.u.locked
	if a.userOKLocked(c, cfg) {
		u, _ := cfg.FindUser(c.u.user.Load().Name) // live flags, not the unlock-time copy
		m.User, m.Trusted, m.Persistent = u.Name, u.Trusted, u.Persistent
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
func (a *App) syncAccessLocked(c *client) { a.pushAccessLocked(c, false) }

// pushAccessLocked: hi marks the answer to a hi.
func (a *App) pushAccessLocked(c *client, hi bool) {
	if !a.pageAllowedLocked(c, gateLog) {
		c.sent = false
	}
	m := a.lockStateLocked(c)
	m.Hi = hi
	b, _ := json.Marshal(m)
	a.deliver(c, b)
	a.pushProfileLocked(c) // only to the user it belongs to
}

func (a *App) syncAccess(c *client) {
	a.mu.Lock()
	a.syncAccessLocked(c)
	a.mu.Unlock()
}

// Hi: a valid token brings the connection up as that user; a bad one is ignored. An
// already unlocked connection is left alone.
func (a *App) Hi(c *client, page, token string) {
	if _, ok := pageGates[page]; !ok {
		page = "main"
	}
	a.mu.Lock()
	cfg := a.r().cfg
	c.u.page = page
	u, ok := a.tokens.user(token, cfg)
	if ok && !u.Persistent && pageGates[page].on(cfg.Locks) {
		ok = false // a no-lock login's tab token means nothing once the page is locked
	}
	if ok && c.u.user.Load() == nil {
		c.u.user.Store(&u)
		c.u.token = token
		c.u.locked = a.tokens.locked(token)
	}
	a.pushAccessLocked(c, true)
	a.mu.Unlock()
	if ok {
		a.tokens.touch(cfg, token) // a used token's age starts over
	}
}

// UserUnlock: PINs are unique across users. Every try counts in the users' pool, like a
// settings PIN.
func (a *App) UserUnlock(c *client, pin string) error {
	users := a.r().cfg.Users
	a.mu.Lock()
	var owner *config.User // a held Lock: only this user's PIN counts
	if c.u.locked {
		owner = c.u.user.Load()
	}
	a.mu.Unlock()
	n := utf8.RuneCountInString(pin)
	var hit *config.User
	err := a.tryPin(&a.userPin, c.ip, func() bool {
		for i := range users {
			if owner != nil && users[i].Name != owner.Name {
				continue
			}
			if users[i].Len == n && config.CheckPin(users[i].Pin, pin) {
				hit = &users[i]
				return true
			}
		}
		return false
	})
	if err != nil {
		return err
	}
	a.mu.Lock()
	if u, ok := a.r().cfg.FindUser(hit.Name); !ok || u.Pin != hit.Pin {
		a.mu.Unlock()
		return errors.New("that PIN changed; try again")
	}
	if c.u.locked && c.u.user.Load() != nil && c.u.user.Load().Name == hit.Name {
		// Resume: same user, same session. Every connection sharing the token resumes.
		tok := c.u.token
		c.u.locked = false
		if tok != "" {
			for cl := range a.clients {
				if cl != c && cl.u.token == tok {
					cl.u.locked = false
					a.syncAccessLocked(cl)
				}
			}
			a.tokens.setLocked(tok, false)
		}
		a.syncAccessLocked(c)
		a.mu.Unlock()
		if tok != "" {
			a.tokens.persist()
		}
		return nil
	}
	defer a.mu.Unlock()
	// A token-X connection unlocking as Y must drop X's token, or Y's lock would revoke
	// X's.
	c.u.token = ""
	c.u.locked = false
	c.u.user.Store(hit)
	a.syncAccessLocked(c)
	return nil
}

// SignOutAll needs the settings PIN (a trusted user is not enough).
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
	a.mu.Lock()
	for cl := range a.clients {
		if u := cl.u.user.Load(); u != nil && strings.EqualFold(u.Name, name) {
			cl.u.user.Store(nil)
			cl.u.token = ""
			cl.u.locked = false
			a.syncAccessLocked(cl)
		}
	}
	a.mu.Unlock()
	return nil
}

// issueSession does disk I/O: never under mu. "" for a non-persistent user.
func (a *App) issueSession(c *client) string { return a.issueSessionTab(c, false) }

// tab: a no-lock login's token for a non-persistent user, marked tab in the store, kept in
// sessionStorage.
func (a *App) issueSessionTab(c *client, tab bool) string {
	who := c.u.user.Load()
	cfg := a.r().cfg
	if who == nil {
		return ""
	}
	a.mu.Lock()
	held := c.u.locked
	a.mu.Unlock()
	if held {
		return "" // a locked session already has its token, or none to give
	}
	u, ok := cfg.FindUser(who.Name)
	if !ok || u.Pin != who.Pin {
		return ""
	}
	if !u.Persistent {
		a.mu.Lock()
		page := c.u.page
		a.mu.Unlock()
		if !tab || pageGates[page].on(cfg.Locks) {
			return ""
		}
	} else {
		tab = false // a persistent user's token is the ordinary one
	}
	t := a.tokens.issue(u, cfg, tab)
	a.mu.Lock()
	// Locked meanwhile, or a racing call already issued one: forgotten after mu goes (that
	// writes the file).
	stale := c.u.user.Load() != who || a.sessionGoodLocked(c)
	if !stale {
		c.u.token = t
	}
	a.mu.Unlock()
	if stale {
		a.tokens.forget(t)
		return ""
	}
	return t
}

// Caller holds mu.
func (a *App) sessionGoodLocked(c *client) bool {
	if c.u.token == "" {
		return false
	}
	_, ok := a.tokens.user(c.u.token, a.r().cfg)
	return ok
}

type sessionMsg struct {
	T     string `json:"t"`
	Token string `json:"token"`
}

// Renews open connections' tokens so a never-reloading page outlasts tokenMaxAge. Off mu.
func (a *App) touchSessions() {
	a.mu.Lock()
	cfg := a.r().cfg
	var held []string
	for cl := range a.clients {
		if cl.u.token != "" && a.userSameLocked(cl, cfg) {
			held = append(held, cl.u.token)
		}
	}
	a.mu.Unlock()
	a.tokens.touch(cfg, held...)
}

// Issues tokens to persistent-user connections holding none or a dead one (tokens are
// otherwise issued only at PIN entry, e.g. after Never log out is ticked or a PIN change).
// Off mu: writes the token file.
func (a *App) catchUpSessions() {
	a.mu.Lock()
	cfg := a.r().cfg
	var todo []*client
	for cl := range a.clients {
		if who := cl.u.user.Load(); who != nil && a.userOKLocked(cl, cfg) && !a.sessionGoodLocked(cl) {
			if u, ok := cfg.FindUser(who.Name); ok && u.Persistent {
				todo = append(todo, cl)
			}
		}
	}
	a.mu.Unlock()
	for _, cl := range todo {
		if t := a.issueSession(cl); t != "" {
			a.reply(cl, sessionMsg{T: "session", Token: t})
		}
	}
}

// UserLock raises the signed-in user's Lock and keeps the session (ADR 0001). The flag
// rides the token, so a reload comes back locked; other connections sharing the token
// lock too. With nobody signed in it does nothing: that Lock is page-only.
func (a *App) UserLock(c *client) {
	a.setHeld(c, true)
}

// setHeld sets or clears the Lock of c's session and of every connection sharing its token.
func (a *App) setHeld(c *client, on bool) {
	a.mu.Lock()
	if !a.userSameLocked(c, a.r().cfg) {
		a.syncAccessLocked(c)
		a.mu.Unlock()
		return
	}
	tok := c.u.token
	c.u.locked = on
	if tok != "" {
		for cl := range a.clients {
			if cl != c && cl.u.token == tok {
				cl.u.locked = on
				a.syncAccessLocked(cl)
			}
		}
		a.tokens.setLocked(tok, on)
	}
	a.syncAccessLocked(c)
	a.mu.Unlock()
	if tok != "" {
		a.tokens.persist()
	}
}

// SignOut ends c's session: the token dies and other connections sharing it sign out too.
func (a *App) SignOut(c *client) {
	a.mu.Lock()
	tok := c.u.token
	c.u.user.Store(nil)
	c.u.token = ""
	c.u.locked = false
	if tok != "" {
		for cl := range a.clients {
			if cl != c && cl.u.token == tok {
				cl.u.user.Store(nil)
				cl.u.token = ""
				cl.u.locked = false
				a.syncAccessLocked(cl)
			}
		}
	}
	a.syncAccessLocked(c)
	a.mu.Unlock()
	a.tokens.forget(tok)
}

// userEdit: Was is the old name (rename keeps the PIN), Pin a new PIN ("" keeps).
type userEdit struct {
	Name       string `json:"name"`
	Was        string `json:"was"`
	Pin        string `json:"pin"`
	Trusted    bool   `json:"trusted"`
	Persistent bool   `json:"persistent"`
}

type usersReq struct {
	Users  []userEdit `json:"users"`
	MaxPin int        `json:"maxPinLen"` // 0 keeps the current one
	// Locks and AutoLock are read only to refuse them: a stale page must not look saved.
	Locks    json.RawMessage `json:"locks"`
	AutoLock json.RawMessage `json:"autoLockMinutes"`
}

// SaveUsers needs the settings PIN only (not the main lock). PINs unique across users.
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
	return a.applyConfig(func(cur *config.Config) error {
		if req.MaxPin != 0 {
			cur.MaxPinLen = req.MaxPin
		}
		maxLen := cur.MaxPinLen
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
				u.TakePin(o)
			}
			out = append(out, u)
		}
		for name, pin := range fresh {
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
		return nil
	})
}

type userResult struct {
	T     string `json:"t"`
	OK    bool   `json:"ok"`
	ID    int    `json:"id"`
	Err   string `json:"err,omitempty"`
	Token string `json:"token,omitempty"`
	Tab   bool   `json:"tab,omitempty"` // Token is a no-lock login's: for this tab only
}

// Unlocks run inline like the settings unlock, so a later lock is handled after.
func (a *App) serveUserUnlock(c *client, m *inMsg) {
	res := userResult{T: "userUnlockResult", OK: true, ID: m.ID}
	err := a.UserUnlock(c, m.Pin)
	if err != nil {
		res.OK, res.Err = false, err.Error()
		a.command(c, "userUnlock", "refused", "", err)
	} else {
		a.command(c, "userUnlock", "ok", "", nil)
		if res.Token = a.issueSessionTab(c, m.Tab); res.Token != "" {
			if us, _ := a.r().cfg.FindUser(c.who()); !us.Persistent {
				res.Tab = true
			}
			a.mu.Lock()
			a.syncAccessLocked(c) // after the token, so the page's state is final
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
}

type userView struct {
	Name       string `json:"name"`
	Len        int    `json:"len"`
	Trusted    bool   `json:"trusted,omitempty"`
	Persistent bool   `json:"persistent,omitempty"`
}

func (a *App) usersList(id int) usersListMsg {
	m := usersListMsg{T: "usersListResult", ID: id, OK: true, Users: []userView{}}
	for _, u := range a.r().cfg.Users {
		m.Users = append(m.Users, userView{u.Name, u.Len, u.Trusted, u.Persistent})
	}
	return m
}

// UserSetPin needs the user's own PIN, not the settings PIN; the old PIN counts as a try
// (same pool). Other connections of the user relock; c stays unlocked.
func (a *App) UserSetPin(c *client, oldPin, newPin string) error {
	who := c.u.user.Load()
	if who == nil {
		return errors.New("unlock with your PIN first")
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	a.mu.Lock()
	cur := a.r().cfg
	ok := a.userOKLocked(c, cur)
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
				if j != i && o.Len == len(newPin) && config.CheckPin(o.Pin, newPin) {
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
		a.syncAccessAllLocked() // the user's other connections relock
	})
	if err != nil {
		return err
	}
	a.catchUpSessions() // c's old tokens died with the old hash
	return nil
}

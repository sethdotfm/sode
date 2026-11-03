package app

import (
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/sethdotfm/sode/internal/config"
)

// Users and page locks (#151). Everything here is off until the config has
// users and a lock turned on. What each message needs is one table
// (policy.go), checked before anything runs; this file is the access state
// itself: who a connection has unlocked as, and the lock screen's messages.

// errPageLocked is the refusal for an action behind a page lock.
var errPageLocked = errors.New("locked; unlock with your PIN first")

// errNeedAdminPin: users and locks are administered with the settings PIN.
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

// pageGates is the lock that guards each page type the browser can declare.
var pageGates = map[string]gate{"main": gateMain, "log": gateLog}

// connUser is a connection's access state: page and token are guarded by
// App.mu; user is atomic so who can be read anywhere.
type connUser struct {
	page  string // "main" (default) or "log", as the page declared
	token string // the admin token it presented
	// suspended: this connection lets its token rest (it locked itself).
	// Per connection, under mu; the token stays valid and AdminUnlock ends it.
	suspended bool
	user      atomic.Pointer[config.User] // who it unlocked as, with that user's PIN hash then
}

// who names the connection for history and logs: the user, else the address.
func (c *client) who() string {
	if u := c.u.user.Load(); u != nil {
		return u.Name
	}
	return c.ip
}

// userOKLocked: c unlocked as a user who still exists with the same PIN (a
// changed PIN or a removal relocks). Caller holds mu.
func (a *App) userOKLocked(c *client, cfg config.Config) bool {
	was := c.u.user.Load()
	if was == nil {
		return false
	}
	u, ok := cfg.FindUser(was.Name)
	return ok && u.Pin == was.Pin
}

// pageAllowedLocked reports whether c may do what lock g guards: the lock is
// off, or c unlocked as a user, or c carries a valid admin token. Caller
// holds mu.
func (a *App) pageAllowedLocked(c *client, g gate) bool {
	cfg := a.r().cfg
	if !g.on(cfg.Locks) {
		return true
	}
	return a.userOKLocked(c, cfg) || a.tokenOKLocked(c, cfg)
}

// tokenOKLocked: c holds a valid admin token and hasn't suspended it. Caller
// holds mu.
func (a *App) tokenOKLocked(c *client, cfg config.Config) bool {
	return !c.u.suspended && a.tokens.ok(c.u.token, cfg.SettingsPin)
}

func (a *App) pageAllowed(c *client, g gate) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pageAllowedLocked(c, g)
}

// lockStateMsg is what a page needs to show or hide its lock screen.
type lockStateMsg struct {
	T        string `json:"t"`
	Page     string `json:"page"`
	Enforced bool   `json:"enforced"` // this page type has a lock on
	Locked   bool   `json:"locked"`
	User     string `json:"user,omitempty"`
	Exempt   bool   `json:"exempt,omitempty"` // an admin token lifts the lock
	// Token: the connection holds a valid admin token, even a suspended one.
	Token bool `json:"token,omitempty"`
}

// lockStateLocked is c's lock state, as its page declared itself. Caller
// holds mu.
func (a *App) lockStateLocked(c *client) lockStateMsg {
	cfg := a.r().cfg
	page := c.u.page
	if page == "" {
		page = "main"
	}
	m := lockStateMsg{T: "lockState", Page: page, Enforced: pageGates[page].on(cfg.Locks)}
	m.Exempt = a.tokenOKLocked(c, cfg)
	m.Token = a.tokens.ok(c.u.token, cfg.SettingsPin)
	if a.userOKLocked(c, cfg) {
		m.User = c.u.user.Load().Name
	}
	m.Locked = m.Enforced && m.User == "" && !m.Exempt
	return m
}

// syncAccessLocked brings c up to date after anything that can change what it
// may do (a lock, an unlock, a config change): it queues c's lock state and
// stops the sent-log feed if c may no longer read it. Caller holds mu.
func (a *App) syncAccessLocked(c *client) {
	if !a.pageAllowedLocked(c, gateLog) {
		c.sent = false
	}
	b, _ := json.Marshal(a.lockStateLocked(c))
	a.deliver(c, b)
}

func (a *App) syncAccess(c *client) {
	a.mu.Lock()
	a.syncAccessLocked(c)
	a.mu.Unlock()
}

// Hi records what a page says about itself: its type and its admin token.
func (a *App) Hi(c *client, page, token string) {
	if _, ok := pageGates[page]; !ok {
		page = "main"
	}
	a.mu.Lock()
	c.u.page, c.u.token = page, token
	a.syncAccessLocked(c)
	a.mu.Unlock()
}

// UserUnlock unlocks c as the user whose PIN is pin. PINs are unique across
// users, so no name is asked for. Every try counts, like a settings PIN, in
// the users' own pool; the page only sends lengths some user has, so a
// shorter or longer one is simply wrong.
func (a *App) UserUnlock(c *client, pin string) error {
	users := a.r().cfg.Users
	n := utf8.RuneCountInString(pin)
	var hit *config.User
	err := a.tryPin(&a.userPin, c.ip, func() bool {
		for i := range users {
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
	defer a.mu.Unlock()
	// Still the same user with the same PIN?
	if u, ok := a.r().cfg.FindUser(hit.Name); !ok || u.Pin != hit.Pin {
		return errors.New("that PIN changed; try again")
	}
	c.u.user.Store(hit)
	a.syncAccessLocked(c)
	return nil
}

// UserLock forgets c's unlock; forget also drops its admin token for good.
// suspend only lets this connection's token rest, so an admin browser can lock
// itself: the token stays valid, and a user PIN or the admin password (which
// ends the suspension) opens it again.
func (a *App) UserLock(c *client, forget, suspend bool) {
	a.mu.Lock()
	tok := c.u.token
	c.u.user.Store(nil)
	if forget {
		c.u.token = ""
	}
	if suspend && !forget {
		c.u.suspended = true
	}
	a.syncAccessLocked(c)
	a.mu.Unlock()
	if forget {
		a.tokens.forget(tok)
	}
}

// AdminUnlock checks the settings PIN (the admin password) from the lock
// screen and, if right, issues a token that lifts the page-type locks for the
// browser holding it, until the settings PIN changes or is removed.
func (a *App) AdminUnlock(c *client, pin string) (string, error) {
	hash, err := a.checkAdmin(c, pin)
	if err != nil {
		return "", err
	}
	if hash == "" {
		return "", errors.New("no admin password is set; set a settings PIN first")
	}
	t := a.tokens.issue(hash)
	a.mu.Lock()
	c.u.token, c.u.suspended = t, false
	a.syncAccessLocked(c)
	a.mu.Unlock()
	return t, nil
}

// userEdit is one row of the users form: Was is the name it had (for a
// rename that keeps the PIN), Pin a new PIN ("" keeps the old).
type userEdit struct {
	Name string `json:"name"`
	Was  string `json:"was"`
	Pin  string `json:"pin"`
}

type usersReq struct {
	Users    []userEdit   `json:"users"`
	Locks    config.Locks `json:"locks"`
	AutoLock int          `json:"autoLockMinutes"`
	MaxPin   int          `json:"maxPinLen"` // 0 keeps the current one
}

// SaveUsers replaces the user directory, locks and auto-lock time. It is a
// setting like any other, so the settings PIN guards it (and nothing else:
// the main lock does not, see policy.go). PINs must be unique across users
// (that is how a PIN alone says who).
func (a *App) SaveUsers(c *client, req usersReq) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.r().cfg.SettingsPin == "" {
		return errNeedAdminPin
	}
	if err := a.guardSave(c); err != nil {
		return err
	}
	return a.applyConfig(func(cur *config.Config) error {
		if req.MaxPin != 0 {
			cur.MaxPinLen = req.MaxPin
		}
		maxLen := cur.MaxPinLen
		old := map[string]config.User{}
		for _, u := range cur.Users {
			old[strings.ToLower(u.Name)] = u
		}
		var out []config.User
		inherited := map[string]bool{} // rows keep a hash only from their own, one row each
		fresh := map[string]string{}   // new PINs by user name
		for _, e := range req.Users {
			u := config.User{Name: strings.TrimSpace(e.Name)}
			if u.Name == "" {
				return errors.New("every user needs a name")
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
				was := e.Was
				if was == "" {
					was = e.Name
				}
				key := strings.ToLower(strings.TrimSpace(was))
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
		// Unique PINs: a new one against every other user's, same length only.
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
		cur.Users, cur.Locks, cur.AutoLockMinutes = out, req.Locks, req.AutoLock
		return nil
	})
}

// userResult answers userUnlock and adminUnlock.
type userResult struct {
	T     string `json:"t"`
	OK    bool   `json:"ok"`
	ID    int    `json:"id"`
	Err   string `json:"err,omitempty"`
	Token string `json:"token,omitempty"`
}

// serveUsers handles the access messages from the read loop. Unlocks run in
// line, like the settings unlock, so a lock sent after one is handled after
// it.
func (a *App) serveUsers(c *client, m *inMsg) {
	switch m.T {
	case "hi":
		a.Hi(c, m.Page, m.Token)
	case "userUnlock":
		res := userResult{T: "userUnlockResult", OK: true, ID: m.ID}
		err := a.UserUnlock(c, m.Pin)
		if err != nil {
			res.OK, res.Err = false, err.Error()
			a.command(c, "userUnlock", "refused", "", err)
		} else {
			a.command(c, "userUnlock", "ok", "", nil)
		}
		a.reply(c, res)
	case "userLock":
		a.UserLock(c, m.Forget, m.Suspend)
		a.command(c, "userLock", "ok", "", nil)
	case "adminUnlock":
		res := userResult{T: "adminUnlockResult", OK: true, ID: m.ID}
		tok, err := a.AdminUnlock(c, m.Pin)
		if err != nil {
			res.OK, res.Err = false, err.Error()
		}
		res.Token = tok
		a.command(c, "adminUnlock", outcome(err), "", err)
		a.reply(c, res)
	case "userSetPin":
		res := opResult{T: "userSetPinResult", OK: true, ID: &m.ID}
		if err := a.UserSetPin(c, m.Old, m.New); err != nil {
			res.OK, res.Err = false, err.Error()
			a.command(c, "userSetPin", "refused", "", err)
		} else {
			a.command(c, "userSetPin", "ok", "", nil)
		}
		a.reply(c, res)
	case "users":
		if m.UsersReq == nil {
			return
		}
		go func(r usersReq, id int) {
			err := a.SaveUsers(c, r)
			// Names only, never a PIN.
			a.replyOp(c, "users", id, 0, "", err)
		}(*m.UsersReq, m.ID)
	}
}

// UserSetPin changes the PIN of the user c is unlocked as, after checking
// old. It needs no settings PIN: the user's own PIN is the credential. The
// old PIN is a try like an unlock (same pool, backed off). Other connections
// of that user relock, as for any changed PIN; c stays unlocked.
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
	_, nr, err := a.update(func(cfg *config.Config) error {
		for i := range cfg.Users {
			if !strings.EqualFold(cfg.Users[i].Name, who.Name) {
				continue
			}
			if cfg.Users[i].Pin != u.Pin {
				return errors.New("your PIN changed; unlock again")
			}
			// Unique: no other user may have this PIN (same length only).
			for j, o := range cfg.Users {
				if j != i && o.Len == len(newPin) && config.CheckPin(o.Pin, newPin) {
					return errors.New("that PIN can't be used")
				}
			}
			cfg.Users[i].Pin, cfg.Users[i].Len = h, len(newPin)
			return nil
		}
		return errors.New("unlock with your PIN first")
	})
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.rt.Store(nr)
	if nu, ok := nr.cfg.FindUser(who.Name); ok {
		c.u.user.Store(&nu)
	}
	for cl := range a.clients {
		a.syncAccessLocked(cl) // the user's other connections relock
	}
	a.mu.Unlock()
	return nil
}

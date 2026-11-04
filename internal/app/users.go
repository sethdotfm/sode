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
	page string // "main" (default) or "log", as the page declared
	// token: the session token this connection holds (presented in hi, or
	// issued when a persistent user unlocked it). Only ever set for a
	// persistent user; a lock revokes it.
	token string
	user  atomic.Pointer[config.User] // who it unlocked as, with that user's PIN hash then
	// profile: the last profile message sent (under mu); "" = none, so a
	// connection that never had one is never sent a clear.
	profile string
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
// off, or c unlocked as a user (a session token only ever brings a connection
// up as that user, see Hi). Caller holds mu.
func (a *App) pageAllowedLocked(c *client, g gate) bool {
	cfg := a.r().cfg
	return !g.on(cfg.Locks) || a.userOKLocked(c, cfg)
}

// lockStateMsg is what a page needs to show or hide its lock screen.
type lockStateMsg struct {
	T        string `json:"t"`
	Page     string `json:"page"`
	Enforced bool   `json:"enforced"` // this page type has a lock on
	Locked   bool   `json:"locked"`
	User     string `json:"user,omitempty"`
	// Lens: the PIN lengths some user has, so the lock pad knows when a PIN
	// can be complete. It rides every lock state (a PIN change sends one to
	// everyone) so a pad never works from a stale list.
	Lens []int `json:"lens,omitempty"`
	// Names: the users' names, for "by Ana" in history. Only a page that may
	// see the main screen gets them (not a locked one).
	Names []string `json:"names,omitempty"`
	// Hi: this is the answer to the page's own `hi`. Only that one says
	// whether its session token was honored; an unsolicited state (a config
	// save by someone else) may arrive first and says nothing about it.
	Hi bool `json:"hi,omitempty"`
	// Trusted: the user may save low-risk settings without the settings PIN;
	// the page only hides its PIN prompt for it, the server decides.
	Trusted bool `json:"trusted,omitempty"`
	// Persistent: the user never logs out (no idle lock in the browser).
	Persistent bool `json:"persistent,omitempty"`
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
	if a.userOKLocked(c, cfg) {
		u, _ := cfg.FindUser(c.u.user.Load().Name) // live flags, not the unlock-time copy
		m.User, m.Trusted, m.Persistent = u.Name, u.Trusted, u.Persistent
	}
	m.Locked = m.Enforced && m.User == ""
	m.Lens = cfg.PinLens()
	if !m.Locked {
		for _, u := range cfg.Users {
			m.Names = append(m.Names, u.Name)
		}
	}
	return m
}

// syncAccessLocked brings c up to date after anything that can change what it
// may do (a lock, an unlock, a config change): it queues c's lock state and
// stops the sent-log feed if c may no longer read it. Caller holds mu.
func (a *App) syncAccessLocked(c *client) { a.pushAccessLocked(c, false) }

// pushAccessLocked is syncAccessLocked; hi marks the answer to a `hi`.
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

// Hi records what a page says about itself: its type and, for a persistent
// user's browser, its session token. A valid token brings the connection up
// unlocked as that user without a PIN; a wrong or dead one is ignored (the
// normal lock pad follows). A connection already unlocked is left as it is.
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
	}
	a.pushAccessLocked(c, true)
	a.mu.Unlock()
	if ok {
		a.tokens.touch(cfg, token) // a used token's age starts over
	}
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
	// A connection that came up by token as X and unlocks as Y must not keep
	// X's token: Y's later lock would revoke X's and relock X's tabs.
	c.u.token = ""
	c.u.user.Store(hit)
	a.syncAccessLocked(c)
	return nil
}

// SignOutAll ends every session of the named user: their tokens are
// deleted and their connections relock. The settings PIN guards it (a
// trusted user is not enough).
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
			a.syncAccessLocked(cl)
		}
	}
	a.mu.Unlock()
	return nil
}

// issueSession gives c's persistent user a session token (the page keeps it
// and presents it in hi). Disk I/O, so never under mu. It returns "" for a
// user who is not persistent.
func (a *App) issueSession(c *client) string { return a.issueSessionTab(c, false) }

// issueSessionTab is issueSession; tab asks for a no-lock login's token for a
// user who is not persistent (only where c's page has no lock on). It is
// marked tab in the store and the page keeps it in sessionStorage.
func (a *App) issueSessionTab(c *client, tab bool) string {
	who := c.u.user.Load()
	cfg := a.r().cfg
	if who == nil {
		return ""
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
	// Locked while the file was written, or already given a good token by a
	// racing call (an unlock and a settings save can overlap): this one is
	// not wanted. Forgotten after mu goes, since that writes the file.
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

// sessionGoodLocked: c holds a session token that is still honored. Caller
// holds mu.
func (a *App) sessionGoodLocked(c *client) bool {
	if c.u.token == "" {
		return false
	}
	_, ok := a.tokens.user(c.u.token, a.r().cfg)
	return ok
}

// sessionMsg hands a browser a session token outside an unlock's reply.
type sessionMsg struct {
	T     string `json:"t"`
	Token string `json:"token"`
}

// touchSessions renews the tokens of the connections that are open now, so a
// persistent user whose page never reloads keeps their session past
// tokenMaxAge. Run from periodic, off mu.
func (a *App) touchSessions() {
	a.mu.Lock()
	cfg := a.r().cfg
	var held []string
	for cl := range a.clients {
		if cl.u.token != "" && a.userOKLocked(cl, cfg) {
			held = append(held, cl.u.token)
		}
	}
	a.mu.Unlock()
	a.tokens.touch(cfg, held...)
}

// catchUpSessions gives a session token to every connection unlocked as a
// persistent user that holds none (or holds a dead one). A token is otherwise
// issued only when the PIN is typed, so a user who was already logged in when
// Never log out was ticked, or who just changed their own PIN (which ends
// their old tokens), would be asked for it again at the next reload. Run
// after config changes, off mu: issuing writes the token file.
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

// UserLock forgets c's unlock and revokes its session token, so a reload
// does not undo a lock. Other connections of the same browser (same token)
// lock too; other browsers' tokens are theirs.
func (a *App) UserLock(c *client) {
	a.mu.Lock()
	tok := c.u.token
	c.u.user.Store(nil)
	c.u.token = ""
	if tok != "" {
		for cl := range a.clients {
			if cl != c && cl.u.token == tok {
				cl.u.user.Store(nil)
				cl.u.token = ""
				a.syncAccessLocked(cl)
			}
		}
	}
	a.syncAccessLocked(c)
	a.mu.Unlock()
	a.tokens.forget(tok)
}

// userEdit is one row of the users form: Was is the name it had (for a
// rename that keeps the PIN), Pin a new PIN ("" keeps the old).
type userEdit struct {
	Name string `json:"name"`
	Was  string `json:"was"`
	Pin  string `json:"pin"`
	// Trusted and Persistent are this row's flags (see config.User).
	Trusted    bool `json:"trusted"`
	Persistent bool `json:"persistent"`
}

type usersReq struct {
	Users  []userEdit `json:"users"`
	MaxPin int        `json:"maxPinLen"` // 0 keeps the current one
}

// SaveUsers replaces the user directory and the longest PIN (the locks and
// the idle time are Settings > Access fields). It is a
// setting like any other, so the settings PIN guards it (and nothing else:
// the main lock does not, see policy.go). PINs must be unique across users
// (that is how a PIN alone says who).
func (a *App) SaveUsers(c *client, req usersReq) error {
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
			// The profile follows the row's Was (a rename keeps it, with or
			// without a new PIN); a user no longer listed loses theirs.
			// A new row (no Was, with a PIN) starts empty even if a user of
			// that name was removed in the same save.
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
		if len(out) == 0 && cur.Locks.Any() {
			// The locks are Settings > Access fields now, not part of this save.
			return errors.New("a screen lock is on: turn it off in Settings > Access (and save) before removing the last user")
		}
		cur.Users = out
		return nil
	})
}

// userResult answers userUnlock. Token is set for a persistent user.
type userResult struct {
	T     string `json:"t"`
	OK    bool   `json:"ok"`
	ID    int    `json:"id"`
	Err   string `json:"err,omitempty"`
	Token string `json:"token,omitempty"`
	Tab   bool   `json:"tab,omitempty"` // Token is a no-lock login's: for this tab only
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
	case "userLock":
		a.UserLock(c)
		a.command(c, "userLock", "ok", "", nil)
	case "userSignOutAll":
		go func(name string, id int) {
			a.replyOp(c, "userSignOutAll", id, 0, "", a.SignOutAll(c, name))
		}(m.Name, m.ID)
	case "userSetPin":
		res := opResult{T: "userSetPinResult", OK: true, ID: &m.ID}
		if err := a.UserSetPin(c, m.Old, m.New); err != nil {
			res.OK, res.Err = false, err.Error()
			a.command(c, "userSetPin", "refused", "", err)
		} else {
			a.command(c, "userSetPin", "ok", "", nil)
		}
		a.reply(c, res)
	case "usersList":
		a.reply(c, a.usersList(m.ID))
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

// usersListMsg is the user directory for the Users editor: what the form
// shows, never a hash. Len is only whether a PIN is set (0 = none).
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

// usersList answers the settings PIN holder (policy: needAdmin).
func (a *App) usersList(id int) usersListMsg {
	m := usersListMsg{T: "usersListResult", ID: id, OK: true, Users: []userView{}}
	for _, u := range a.r().cfg.Users {
		m.Users = append(m.Users, userView{u.Name, u.Len, u.Trusted, u.Persistent})
	}
	return m
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
	a.catchUpSessions() // c's old tokens died with the old hash
	return nil
}

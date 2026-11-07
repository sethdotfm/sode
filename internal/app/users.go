package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/sethdotfm/sode/internal/config"
)

// Users and page locks. Off until the config has users and a lock on. policy.go holds the
// message->need table; this file is the access state (who a connection unlocked as) and
// the lock screen messages.

var errPageLocked = errors.New("locked; unlock with your PIN first")

var errNeedAdminPin = errors.New("Set an Advanced Settings PIN first: users and locks are administered with it")

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
	return !c.u.locked && a.userSameMu(c, cfg)
}

// heldMu: the session's Lock is on and its user can still resume it. Caller holds mu.
func (a *App) heldMu(c *client, cfg config.Config) bool {
	return c.u.locked && a.userSameMu(c, cfg)
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
}

// Caller holds mu.
func (a *App) lockStateMu(c *client) lockStateMsg {
	cfg := a.r().cfg
	page := c.u.page
	if page == "" {
		page = "main"
	}
	if c.u.locked && !a.userSameMu(c, cfg) {
		// Nobody left to resume it: the user was removed or changed their PIN.
		c.u.user.Store(nil)
		c.u.locked = false
	}
	m := lockStateMsg{T: "lockState", Page: page, Enforced: pageGates[page].on(cfg.Locks)}
	m.Held = c.u.locked
	m.Mode = cfg.SignInMode
	if a.userOKMu(c, cfg) {
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
}

func (a *App) syncAccess(c *client) {
	a.mu.Lock()
	a.syncAccessMu(c)
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
	if ok && !u.Persistent && pageGates[page].on(cfg.Locks) && !a.tokens.locked(token) {
		ok = false // under a PIN lock a tab token only carries a held Lock, never a sign-in
	}
	if ok && c.u.user.Load() == nil {
		c.u.user.Store(&u)
		c.u.token = token
		c.u.locked = a.tokens.locked(token)
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

// UserUnlockAs: PIN only mode ignores username and finds the user by PIN, which no two
// users share; a PIN several users hold (left over from Username and PIN mode) signs
// nobody in. Username and PIN mode compares the PIN with the named user's hash only. A
// held Lock resumes with its own user's PIN whatever the mode, and a username there must
// be that user's. Every try counts in the users' pool, like an Advanced Settings PIN.
func (a *App) UserUnlockAs(c *client, username, pin string) error {
	cfg := a.r().cfg
	users := cfg.Users
	a.mu.Lock()
	var owner *config.User // a held Lock: only this user's PIN counts
	if c.u.locked {
		owner = c.u.user.Load()
	}
	a.mu.Unlock()
	n := utf8.RuneCountInString(pin)
	var hit *config.User
	err := a.tryPin(&a.userPin, c.ip, func() bool {
		if cfg.SignInMode == config.SignInUsername {
			named, ok := cfg.FindUsername(username)
			switch {
			case owner != nil && username != "" && (!ok || named.Name != owner.Name):
				burnPinCheck(pin)
				return false
			case owner != nil:
				named, ok = *owner, true
			case !ok:
				burnPinCheck(pin)
				return false
			}
			for i := range users {
				if users[i].Name != named.Name {
					continue
				}
				// The hash runs whatever the length: PIN lengths are public, so skipping it
				// would tell a known username from an unknown one.
				if config.CheckPin(users[i].Pin, pin) && users[i].Len == n {
					hit = &users[i]
					return true
				}
			}
			return false
		}
		var found []*config.User
		for i := range users {
			if owner != nil && users[i].Name != owner.Name {
				continue
			}
			if users[i].Len == n && config.CheckPin(users[i].Pin, pin) {
				found = append(found, &users[i])
			}
		}
		if len(found) != 1 {
			return false
		}
		hit = found[0]
		return true
	})
	if err != nil {
		return err
	}
	a.mu.Lock()
	if u, ok := a.r().cfg.FindUser(hit.Name); !ok || u.Pin != hit.Pin {
		a.mu.Unlock()
		return errors.New("that PIN changed; try again")
	}
	held := c.u.user.Load()
	if c.u.locked && held != nil && held.Name != hit.Name {
		// Locked by another tab while the PIN was being checked: only the holder resumes it.
		a.mu.Unlock()
		return errors.New("wrong PIN")
	}
	if c.u.locked && held != nil {
		// Resume: same user, same session. Every connection sharing the token resumes.
		tok := c.u.token
		// A tab token under a PIN lock exists only to carry the Lock across a reload.
		drop := tok != "" && !hit.Persistent && pageGates[c.u.page].on(a.r().cfg.Locks)
		c.u.locked = false
		if drop {
			c.u.token = ""
		}
		if tok != "" {
			for cl := range a.clients {
				if cl != c && cl.u.token == tok {
					cl.u.locked = false
					if drop {
						cl.u.token = ""
					}
					a.syncAccessMu(cl)
				}
			}
			a.tokens.setLocked(tok, false)
		}
		a.syncAccessMu(c)
		a.mu.Unlock()
		switch {
		case drop:
			a.tokens.forget(tok)
		case tok != "":
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
	a.mu.Lock()
	for cl := range a.clients {
		if u := cl.u.user.Load(); u != nil && strings.EqualFold(u.Name, name) {
			cl.u.user.Store(nil)
			cl.u.token = ""
			cl.u.locked = false
			a.syncAccessMu(cl)
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
}

type usersReq struct {
	Users  []userEdit `json:"users"`
	MaxPin int        `json:"maxPinLen"` // 0 keeps the current one
	// SignInMode: "" keeps the current one. Saved here so it needs the Advanced Settings PIN.
	SignInMode string `json:"signInMode"`
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
	return a.applyConfig(func(cur *config.Config) error {
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
			}
			if e.Username != nil {
				u.Username = strings.TrimSpace(*e.Username)
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
	err := a.UserUnlockAs(c, m.User, m.Pin)
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
}

type userView struct {
	Name       string `json:"name"`
	Username   string `json:"username,omitempty"`
	Len        int    `json:"len"`
	Trusted    bool   `json:"trusted,omitempty"`
	Persistent bool   `json:"persistent,omitempty"`
}

func (a *App) usersList(id int) usersListMsg {
	cfg := a.r().cfg
	m := usersListMsg{T: "usersListResult", ID: id, OK: true, Users: []userView{}, Mode: cfg.SignInMode}
	for _, u := range cfg.Users {
		m.Users = append(m.Users, userView{u.Name, u.Username, u.Len, u.Trusted, u.Persistent})
	}
	return m
}

// UserSetPin needs the user's own PIN, not the Advanced Settings PIN; the old PIN counts as a try
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

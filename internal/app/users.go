package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/sethdotfm/sode/internal/config"
)

// Users and page locks (#151). Everything here is off until the config has
// users and a lock turned on. Enforcement is on the server: a connection that
// hasn't unlocked as a user (or carries an admin token) is refused the
// operations behind a lock, whatever its page does.
//
// Which lock guards what: the main lock guards batch, refreshNames,
// clearHistory, config, filterSets and users; the grid lock additionally
// guards batch from a page that declared itself "grid"; the log lock guards
// the sent log (feed and requests) and the uptime graph. The page type is
// declared by the browser (hi), so it can only add the grid lock, never skip
// the main one: until Grid pages exist, a main-locked Sode also refuses takes
// from "grid".

// errNeedUser is the refusal for an operation behind a lock.
var errNeedUser = errors.New("locked; unlock with your PIN first")

// errNeedAdminPin: users and locks are administered with the settings PIN.
var errNeedAdminPin = errors.New("Set a settings PIN first: users and locks are administered with it")

type gate int

const (
	gateMain gate = iota
	gateGrid
	gateLog
)

func (l gate) on(k config.Locks) bool {
	switch l {
	case gateGrid:
		return k.Grid
	case gateLog:
		return k.Log
	}
	return k.Main
}

// connUser is a connection's access state. Guarded by App.mu except who.
type connUser struct {
	page  string // "main" (default), "grid" or "log", as the page declared
	user  string // the user it unlocked as
	uhash string // that user's PIN hash then; a changed PIN or removal relocks
	token string // the admin token it presented
	name  atomic.Pointer[string]
}

// who names the connection for history and logs: the user, else the address.
func (c *client) who() string {
	if n := c.u.name.Load(); n != nil && *n != "" {
		return *n
	}
	return c.ip
}

// userState is the admin tokens (in memory, and in a file next to the
// config when there is one).
type userState struct {
	pin    pinState // wrong user PINs, apart from the settings PIN's count (guarded by App.mu)
	mu     sync.Mutex
	loaded bool
	toks   map[string]tokenRec // sha256 of the token, hex
}

type tokenRec struct {
	Pin string    `json:"pin"` // the settings PIN hash it was issued against
	At  time.Time `json:"at"`
}

const maxTokens = 100

func tokenKey(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

func (a *App) tokenPath() string {
	if a.cfgPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(a.cfgPath), "tokens.json")
}

// loadTokens reads the token file once. Caller holds usr.mu.
func (a *App) loadTokens() {
	if a.usr.loaded {
		return
	}
	a.usr.loaded = true
	a.usr.toks = map[string]tokenRec{}
	if p := a.tokenPath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if err := json.Unmarshal(b, &a.usr.toks); err != nil {
				log.Printf("users: %s is damaged, tokens forgotten: %v", p, err)
				a.usr.toks = map[string]tokenRec{}
			}
		}
	}
}

// saveTokens writes the token file. Caller holds usr.mu.
func (a *App) saveTokens() {
	p := a.tokenPath()
	if p == "" {
		return
	}
	b, _ := json.Marshal(a.usr.toks)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err == nil {
		err = os.Rename(tmp, p)
		if err == nil {
			return
		}
	}
	log.Printf("users: tokens not saved to %s", p)
}

// tokenOK reports whether t is an admin token issued against the settings PIN
// in force. A token outlives nothing: changing or removing the PIN makes every
// one fail here, and pruneTokens drops them from the file.
func (a *App) tokenOK(t, settingsPin string) bool {
	if t == "" || settingsPin == "" {
		return false
	}
	a.usr.mu.Lock()
	defer a.usr.mu.Unlock()
	// Loaded by pruneTokens at startup and on every config apply: no file
	// I/O here, since callers hold App.mu.
	r, ok := a.usr.toks[tokenKey(t)]
	return ok && r.Pin == settingsPin
}

// pruneTokens drops every token not issued against the current settings PIN.
func (a *App) pruneTokens() {
	pin := a.r().cfg.SettingsPin
	a.usr.mu.Lock()
	defer a.usr.mu.Unlock()
	a.loadTokens()
	n := 0
	for k, r := range a.usr.toks {
		if pin == "" || r.Pin != pin {
			delete(a.usr.toks, k)
			n++
		}
	}
	if n > 0 {
		a.saveTokens()
	}
}

// issueToken makes a token for the settings PIN pin.
func (a *App) issueToken(pin string) string {
	b := make([]byte, 32)
	rand.Read(b)
	t := base64.RawURLEncoding.EncodeToString(b)
	a.usr.mu.Lock()
	defer a.usr.mu.Unlock()
	a.loadTokens()
	if len(a.usr.toks) >= maxTokens {
		var oldest string
		for k, r := range a.usr.toks {
			if oldest == "" || r.At.Before(a.usr.toks[oldest].At) {
				oldest = k
			}
		}
		delete(a.usr.toks, oldest)
	}
	a.usr.toks[tokenKey(t)] = tokenRec{Pin: pin, At: time.Now()}
	a.saveTokens()
	return t
}

func (a *App) forgetToken(t string) {
	if t == "" {
		return
	}
	a.usr.mu.Lock()
	defer a.usr.mu.Unlock()
	a.loadTokens()
	if _, ok := a.usr.toks[tokenKey(t)]; ok {
		delete(a.usr.toks, tokenKey(t))
		a.saveTokens()
	}
}

// userOKLocked: c unlocked as a user who still exists with the same PIN.
// Caller holds mu.
func (a *App) userOKLocked(c *client, cfg config.Config) bool {
	if c.u.user == "" {
		return false
	}
	u, ok := cfg.FindUser(c.u.user)
	return ok && u.Pin == c.u.uhash
}

// refusedLocked reports whether c may not do what lock g guards. Caller holds mu.
func (a *App) refusedLocked(c *client, g gate) bool {
	cfg := a.r().cfg
	if !g.on(cfg.Locks) {
		return false
	}
	return !a.userOKLocked(c, cfg) && !a.tokenOK(c.u.token, cfg.SettingsPin)
}

func (a *App) refused(c *client, g gate) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.refusedLocked(c, g)
}

// deny tells c it is locked (so its page shows the lock screen) and logs it.
func (a *App) deny(c *client, what string) {
	a.command(c, what, "locked", "", nil)
	a.sendLockState(c)
}

// pageGate is the lock that guards a page of this type.
func pageGate(page string) gate {
	switch page {
	case "grid":
		return gateGrid
	case "log":
		return gateLog
	}
	return gateMain
}

// batchRefused: a take needs the main lock's unlock, and from a page that said
// it is a grid page also the grid lock's. The page type is the browser's word,
// so claiming "grid" can only add a lock, never skip the main one.
func (a *App) batchRefused(c *client) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.refusedLocked(c, gateMain) || (c.u.page == "grid" && a.refusedLocked(c, gateGrid))
}

// lockStateMsg is what a page needs to show or hide its lock screen.
type lockStateMsg struct {
	T        string `json:"t"`
	Page     string `json:"page"`
	Enforced bool   `json:"enforced"` // this page type has a lock on
	Locked   bool   `json:"locked"`
	User     string `json:"user,omitempty"`
	Exempt   bool   `json:"exempt,omitempty"` // an admin token lifts the lock
}

// lockStateLocked is c's lock state; it also stops the sent-log feed for a
// connection that no longer may see it. Caller holds mu.
func (a *App) lockStateLocked(c *client) []byte {
	cfg := a.r().cfg
	page := c.u.page
	if page == "" {
		page = "main"
	}
	g := pageGate(page)
	m := lockStateMsg{T: "lockState", Page: page, Enforced: g.on(cfg.Locks)}
	m.Exempt = a.tokenOK(c.u.token, cfg.SettingsPin)
	if a.userOKLocked(c, cfg) {
		m.User = c.u.user
	}
	m.Locked = m.Enforced && m.User == "" && !m.Exempt
	if a.refusedLocked(c, gateLog) {
		c.sent = false
	}
	b, _ := json.Marshal(m)
	return b
}

// deliverLock queues c's lock state. Caller holds mu.
func (a *App) deliverLock(c *client) { a.deliver(c, a.lockStateLocked(c)) }

func (a *App) sendLockState(c *client) {
	a.mu.Lock()
	a.deliverLock(c)
	a.mu.Unlock()
}

// Hi records what a page says about itself: its type and its admin token.
func (a *App) Hi(c *client, page, token string) {
	switch page {
	case "main", "grid", "log":
	default:
		page = "main"
	}
	a.mu.Lock()
	c.u.page, c.u.token = page, token
	a.deliverLock(c)
	a.mu.Unlock()
}

// UserUnlock unlocks c as the user whose PIN is pin. PINs are unique across
// users, so no name is asked for. A PIN whose length matches no user's is not
// a try (it is the page sending as it types), answered partial; one that does
// is checked against the users of that length and counted like a settings PIN.
func (a *App) UserUnlock(c *client, pin string) (partial bool, err error) {
	cfg := a.r().cfg
	var cands []config.User
	n := utf8.RuneCountInString(pin)
	for _, u := range cfg.Users {
		if u.Len == n {
			cands = append(cands, u)
		}
	}
	if len(cands) == 0 {
		return true, nil
	}
	if err := a.pinStart(true); err != nil {
		return false, err
	}
	var hit *config.User
	for i := range cands {
		if config.CheckPin(cands[i].Pin, pin) {
			hit = &cands[i]
			break
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.pinEnd(true, hit != nil, c.ip); err != nil {
		return false, err
	}
	// Still the same user with the same PIN?
	if u, ok := a.r().cfg.FindUser(hit.Name); !ok || u.Pin != hit.Pin {
		return false, errors.New("that PIN changed; try again")
	}
	c.u.user, c.u.uhash = hit.Name, hit.Pin
	name := hit.Name
	c.u.name.Store(&name)
	a.deliverLock(c)
	return false, nil
}

// UserLock forgets c's unlock; forget also drops its admin token for good.
func (a *App) UserLock(c *client, forget bool) {
	a.mu.Lock()
	tok := c.u.token
	c.u.user, c.u.uhash = "", ""
	c.u.name.Store(nil)
	if forget {
		c.u.token = ""
	}
	a.deliverLock(c)
	a.mu.Unlock()
	if forget {
		a.forgetToken(tok)
	}
}

// AdminUnlock checks the settings PIN (the admin password) from the lock
// screen and, if right, issues a token that lifts the page-type locks for the
// browser holding it, until the settings PIN changes or is removed.
func (a *App) AdminUnlock(c *client, pin string) (string, error) {
	hash := a.r().cfg.SettingsPin
	if hash == "" {
		return "", errors.New("no admin password is set; set a settings PIN first")
	}
	if err := a.pinStart(false); err != nil {
		return "", err
	}
	ok := config.CheckPin(hash, pin)
	a.mu.Lock()
	err := a.pinEnd(false, ok, c.ip)
	cur := a.r().cfg.SettingsPin
	a.mu.Unlock()
	if err != nil {
		return "", err
	}
	if cur != hash {
		return "", errLocked
	}
	t := a.issueToken(hash)
	a.mu.Lock()
	c.u.token = t
	a.deliverLock(c)
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
}

// SaveUsers replaces the user directory, locks and auto-lock time. It is a
// setting like any other, so the settings PIN guards it, and so does the main
// lock. PINs must be unique across users (that is how a PIN alone says who).
func (a *App) SaveUsers(c *client, req usersReq) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.r().cfg.SettingsPin == "" {
		return errNeedAdminPin
	}
	if a.locked(c) {
		return errLocked
	}
	if a.refused(c, gateMain) {
		a.sendLockState(c)
		return errNeedUser
	}
	return a.applyConfig(func(cur *config.Config) error {
		old := map[string]config.User{}
		for _, u := range cur.Users {
			old[strings.ToLower(u.Name)] = u
		}
		var out []config.User
		inherited := map[string]bool{} // rows keep a hash only from their own, one row each
		fresh := map[string]string{}   // new PINs by user name
		for _, e := range req.Users {
			u := config.User{Name: strings.TrimSpace(e.Name)}
			if e.Pin != "" {
				if err := config.CheckUserPinRules(e.Pin); err != nil {
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
	T       string `json:"t"`
	OK      bool   `json:"ok"`
	ID      int    `json:"id"`
	Partial bool   `json:"partial,omitempty"` // the PIN so far matches no length: keep typing
	Err     string `json:"err,omitempty"`
	Token   string `json:"token,omitempty"`
}

// serveUsers handles the access messages from the read loop. Unlocks run in
// line, like the settings unlock, so a lock sent after one is handled after it.
func (a *App) serveUsers(c *client, t, page, token, pin string, id int, forget bool, req *usersReq) {
	switch t {
	case "hi":
		a.Hi(c, page, token)
	case "userUnlock":
		res := userResult{T: "userUnlockResult", OK: true, ID: id}
		partial, err := a.UserUnlock(c, pin)
		switch {
		case partial:
			res.OK, res.Partial = false, true
		case err != nil:
			res.OK, res.Err = false, err.Error()
			a.command(c, "userUnlock", "refused", "", err)
		default:
			a.command(c, "userUnlock", "ok", "", nil)
		}
		a.reply(c, res)
	case "userLock":
		a.UserLock(c, forget)
		a.command(c, "userLock", "ok", "", nil)
	case "adminUnlock":
		res := userResult{T: "adminUnlockResult", OK: true, ID: id}
		tok, err := a.AdminUnlock(c, pin)
		if err != nil {
			res.OK, res.Err = false, err.Error()
		}
		res.Token = tok
		a.command(c, "adminUnlock", outcome(err), "", err)
		a.reply(c, res)
	case "users":
		if req == nil {
			return
		}
		go func(r usersReq) {
			res := opResult{T: "usersResult", OK: true, ID: &id}
			err := a.SaveUsers(c, r)
			// Names only, never a PIN.
			a.command(c, "users", outcome(err), "", err)
			if err != nil {
				res.OK, res.Err = false, err.Error()
				res.Locked = locked(err)
			} else {
				log.Printf("config: users saved by %s", c.who())
			}
			a.reply(c, res)
		}(*req)
	}
}

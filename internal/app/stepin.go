package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Connection state only: issueTokenFor gives it no token, so a reload, page close or dropped connection ends it
// (docs/agents/users-locks.md).

// stepInIdle is fixed: No auto-lock on either user does not make it longer.
const stepInIdle = 5 * time.Minute

// stepIn is what Step out returns to. Under App.mu.
type stepIn struct {
	orig *config.User // nil: the browser's Guest state
	sess *session     // the original session, which the original's other tabs share
	seen time.Time    // last activity of this tab while stepped in
	// ended: the original sign-in ended meanwhile with no token to show it (SignOutAll).
	ended bool
}

type stepEndedMsg struct {
	T string `json:"t"` // "stepEnded"
	// Why: "out", "idle", "lock", "save" or "signOut" (Sign out everywhere of the stepped-in user).
	Why  string `json:"why"`
	Note string `json:"note,omitempty"` // a status text when the original sign-in ended meanwhile
}

// serveStepIn writes the Log row: by the stepped-in user, or by the browser for a failed try.
func (a *App) serveStepIn(c *client, m *inMsg) {
	before := c.who()
	err := a.StepIn(c, m.User, m.Pin)
	if err != nil {
		a.commandBy(before, "stepIn", "refused", "as "+m.User, err)
	} else {
		a.commandBy(c.who(), "stepIn", "ok", "browser of "+a.browserOf(c), nil)
	}
	a.reply(c, opResultFor("stepIn", &m.ID, err))
}

// stepEndWords: how a step-in ended, for its Step out row. Keys are stepEndedMsg.Why, and
// "closed" for a reload, page close or dropped connection.
var stepEndWords = map[string]string{
	"out":     "Step out",
	"idle":    "idle",
	"lock":    "a Lock",
	"save":    "a save changed the user",
	"signOut": "Sign out everywhere",
	"closed":  "the page closed or the connection dropped",
}

// The row goes before c returns to the original.
func (a *App) stepOutRowMu(c *client, why string) {
	a.commandBy(c.who(), "stepOut", "ok", stepEndWords[why]+", browser of "+a.browserOfMu(c), nil)
}

// StepIn checks the PIN in that name's pool, so wrong PINs count toward its wait.
func (a *App) StepIn(c *client, name, pin string) error {
	a.mu.Lock()
	err := a.stepInRefusalMu(c, name)
	cfg := a.r().cfg
	a.mu.Unlock()
	if err != nil {
		return err
	}
	hit := pinCandidate(cfg, nil, name)
	if hit != nil && !hit.HasPin() {
		return errors.New(hit.Name + " has no PIN: only a user with a PIN can step in")
	}
	if err := a.provePin(c, nil, hit, pin); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg = a.r().cfg
	live, ok := a.livePinMu(hit)
	if !ok {
		return errors.New("that PIN changed; try again")
	}
	if err := a.stepInRefusalMu(c, live.Name); err != nil {
		return err
	}
	now := a.now()
	st := &stepIn{sess: c.sessMu(), seen: now}
	if a.userOKMu(c, cfg) {
		st.orig = c.u.user.Load()
	}
	c.u.step = st
	a.feedOriginalMu(c, now)
	c.u.user.Store(&live)
	c.u.sess = &session{seen: now}
	// The step-in PIN counts as a PIN recheck; the original's recheck ends.
	c.u.recheck = recheck{user: live.ID, hash: live.Pin, until: now.Add(a.recheckFor)}
	a.syncAccessMu(c)
	return nil
}

// Never on the Lock screen, never twice, never as the user already signed in.
func (a *App) stepInRefusalMu(c *client, name string) error {
	cfg := a.r().cfg
	switch {
	case c.u.step != nil:
		return errors.New("you are stepped in already: step out first")
	case cmp.Or(c.u.page, "main") != "main":
		return errors.New("step in on the main page or in Settings")
	case c.u.lockedMu(), gateMain.on(cfg.Locks) && !a.userOKMu(c, cfg):
		return errors.New("dismiss the Lock first")
	}
	if u, ok := cfg.FindUserByName(name); ok && a.userOKMu(c, cfg) && u.ID == c.userID() {
		return errors.New("you are signed in as " + u.Name + " already")
	}
	return nil
}

// Step-in activity counts for the original session too.
func (a *App) feedOriginalMu(c *client, now time.Time) {
	st := c.u.step
	st.seen = now
	if st.orig != nil {
		st.sess.seen = now
		a.tokens.seenAt(st.sess.token, now)
	}
}

// StepOut is the Step out button; a step-out with no step-in sends the lock state only.
func (a *App) StepOut(c *client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c.u.step == nil {
		a.syncAccessMu(c)
		return
	}
	a.stepOutMu(c, "out")
}

// stepOutMu returns c to the original. If the original sign-in ended meanwhile, c lands signed
// out (on the PIN pad under a page lock) with a status text. Caller holds mu.
func (a *App) stepOutMu(c *client, why string) {
	st := c.u.step
	if st == nil {
		return
	}
	a.stepOutRowMu(c, why)
	c.u.step = nil
	c.u.recheck = recheck{}
	cfg := a.r().cfg
	note := ""
	switch {
	case st.orig == nil:
		c.u.user.Store(nil)
		c.u.sess = nil
	case a.originalLiveMu(st, cfg):
		u, _ := cfg.FindUserByID(st.orig.ID) // the live name, after a rename meanwhile
		c.u.user.Store(&u)
		c.u.sess = st.sess
	default:
		c.u.user.Store(nil)
		c.u.sess = nil
		note = "Your sign-in ended while you were stepped in."
	}
	b, _ := json.Marshal(stepEndedMsg{T: "stepEnded", Why: why, Note: note})
	a.deliver(c, b)
	a.syncAccessMu(c)
}

// For History; "" when not stepped in.
func (a *App) browserOf(c *client) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.browserOfMu(c)
}

func (a *App) browserOfMu(c *client) string {
	st := c.u.step
	switch {
	case st == nil:
		return ""
	case st.orig == nil:
		return c.ip
	}
	if u, ok := a.r().cfg.FindUserByID(st.orig.ID); ok {
		return u.Name // the live name, after a rename meanwhile
	}
	return st.orig.Name
}

func (a *App) originalLiveMu(st *stepIn, cfg config.Config) bool {
	u, ok := cfg.FindUserByID(st.orig.ID)
	if !ok || u.Pin != st.orig.Pin || st.ended {
		return false
	}
	if st.sess.token == "" {
		return true
	}
	_, ok = a.tokens.user(st.sess.token, cfg)
	return ok
}

// The step-ins that return to a Locked session end, so they land on that Lock.
func (a *App) stepOutsOnLockMu(s *session) {
	for cl := range a.clients {
		if cl.u.step != nil && cl.u.step.sess == s {
			a.stepOutMu(cl, "lock")
		}
	}
}

func (a *App) stepOutsOnSaveMu(old, cfg config.Config) {
	for cl := range a.clients {
		if cl.u.step != nil && stepInEnds(old, cfg, cl.userID()) {
			a.stepOutMu(cl, "save")
		}
	}
}

// stepInEnds: the change from old to cfg ends the step-ins of user id. Keep in step with the
// Interruptions line (stepInStops).
func stepInEnds(old, cfg config.Config, id string) bool {
	was, _ := old.FindUserByID(id)
	u, ok := cfg.FindUserByID(id)
	return !ok || u.Name != was.Name || u.Pin != was.Pin ||
		!config.RoleAtLeast(cmp.Or(u.Role, config.RoleStandard), cmp.Or(was.Role, config.RoleStandard))
}

// stepInStops: the Interruptions line "Step-ins of <user>" for each stepped-in user the commit
// from live to next changes. Caller holds cfgMu, not mu.
func (a *App) stepInStops(live, next config.Config) []interruption {
	n := map[string]int{}
	a.mu.Lock()
	for cl := range a.clients {
		if cl.u.step != nil && stepInEnds(live, next, cl.userID()) {
			n[cl.userID()]++
		}
	}
	a.mu.Unlock()
	var out []interruption
	for _, u := range live.Users {
		if k := n[u.ID]; k > 0 {
			out = append(out, interruption{StepIns: k, Ends: true,
				Text: fmt.Sprintf("Step-ins of %s: this save ends %s.", u.Name, howMany(k, "step-in", "step-ins"))})
		}
	}
	return out
}

// A shorter auto-lock of the original user wins; then the original session locks if it is idle too. The caller runs the
// returned Lock after it releases mu.
func (a *App) stepIdleMu(c *client, cfg config.Config, now time.Time) (heldJob, bool) {
	st := c.u.step
	lim := stepInIdle
	if st.orig != nil {
		if u, ok := cfg.FindUserByID(st.orig.ID); ok {
			if l := idleLimit(cfg, u); l > 0 && l < lim {
				lim = l
			}
		}
	}
	if now.Sub(st.seen) < lim {
		return heldJob{}, false
	}
	a.stepOutMu(c, "idle")
	if !a.overdueMu(c, cfg, now) {
		return heldJob{}, false
	}
	tok, _ := a.holdMu(c, true)
	return heldJob{c, tok}, true
}

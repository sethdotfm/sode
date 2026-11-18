package app

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Critical actions and the PIN recheck (GLOSSARY.md). The policy table classifies every
// message (TestPolicyClassifiesEveryMessageCritical). critAlways is checked in admit;
// critByFields saves call recheckFields under cfgMu with the config they would write.
type crit uint8

const (
	critUnset crit = iota // a test failure
	critNever
	critAlways
	critByFields
)

const recheckFor = time.Minute

// recheck: one connection's PIN recheck, under App.mu. hash pins it to the PIN that was
// typed, so a PIN change ends it. Lock, sign-out and a new sign-in clear it.
type recheck struct {
	user, hash string
	until      time.Time
}

// recheckErr: what needs the recheck; none for an action that is always Critical.
type recheckErr struct{ what []string }

func (e recheckErr) Error() string {
	if len(e.what) == 0 {
		return "this Critical action needs PIN recheck: type your PIN again"
	}
	return "a change to " + strings.Join(e.what, ", ") + " needs PIN recheck: type your PIN again"
}

func isRecheckErr(err error) bool {
	var r recheckErr
	return errors.As(err, &r)
}

// recheckedMu: c can do a Critical action now. With no users there is no PIN to check.
// Caller holds mu.
func (a *App) recheckedMu(c *client) bool {
	cfg := a.r().cfg
	if len(cfg.Users) == 0 {
		return true
	}
	r := c.u.recheck
	if r.user == "" || !a.now().Before(r.until) || !a.userOKMu(c, cfg) || c.userID() != r.user {
		return false
	}
	u, ok := cfg.FindUserByID(r.user)
	return ok && u.Pin == r.hash
}

func (a *App) servePinRecheck(c *client, m *inMsg) {
	a.mu.Lock()
	cfg := a.r().cfg
	var u config.User
	ok := a.userOKMu(c, cfg)
	if ok {
		u, ok = cfg.FindUserByID(c.userID())
	}
	a.mu.Unlock()
	var err error
	switch {
	case !ok || !u.HasPin():
		err = errors.New("sign in as a user with a PIN first")
	default:
		err = a.tryUserPin(&u, c.ip, func() bool { return config.CheckPin(u.Pin, m.Pin) })
	}
	if err == nil {
		a.mu.Lock()
		// The PIN check can wait: it is possible that the sign-in or the PIN changed since.
		if live, ok := a.r().cfg.FindUserByID(u.ID); ok && live.Pin == u.Pin && a.userOKMu(c, a.r().cfg) && c.userID() == u.ID {
			c.u.recheck = recheck{user: u.ID, hash: u.Pin, until: a.now().Add(a.recheckFor)}
		} else {
			err = errors.New("your sign-in changed; try again")
		}
		a.mu.Unlock()
	}
	a.command(c, "pinRecheck", outcome(err), "", err)
	a.reply(c, opResultFor("pinRecheck", &m.ID, err))
}

// critical wraps a critByFields save's mutate: a change to a Critical field needs c's recheck.
func (a *App) critical(c *client, mutate func(*config.Config) error) func(*config.Config) error {
	return func(cur *config.Config) error {
		live := cur.Clone()
		if err := mutate(cur); err != nil {
			return err
		}
		next := cur.Clone()
		next.Normalize()
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.recheckedMu(c) {
			return nil
		}
		var actor config.User
		if u := c.u.user.Load(); u != nil {
			actor, _ = live.FindUserByID(u.ID)
		}
		if what := criticalEdits(live, next, actor); len(what) > 0 {
			return recheckErr{what: what}
		}
		return nil
	}
}

// presetFields: every field an Access preset writes is Critical. Later access fields join here.
var presetFields = []struct {
	what    string
	changed func(a, b config.Config) bool
}{
	{"the page locks", func(a, b config.Config) bool {
		return a.Locks.Main != b.Locks.Main || a.Locks.Log != b.Locks.Log || a.Locks.Panel != b.Locks.Panel
	}},
	{"auto-lock", func(a, b config.Config) bool { return a.AutoLockMinutes != b.AutoLockMinutes }},
	{"Guest role", func(a, b config.Config) bool { return a.GuestRole != b.GuestRole }},
	{"Wait after wrong PINs", func(a, b config.Config) bool { return a.PinWaits != b.PinWaits }},
	{"the keyless API", func(a, b config.Config) bool { return a.API.GuestEnabled != b.API.GuestEnabled }},
	{"keyless scopes", func(a, b config.Config) bool { return !sameSet(a.GuestAPIScopes, b.GuestAPIScopes) }},
	{"Public panels allowed", func(a, b config.Config) bool { return a.PublicPanels != b.PublicPanels }},
	{"the new-panel Authentication mode", func(a, b config.Config) bool { return a.NewPanelActs != b.NewPanelActs }},
}

// sameSet: pages send scopes in their own order.
func sameSet(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}

func sameRights(a, b config.Rights) bool { return a.Within(b) && b.Within(a) }

// criticalEdits names the Critical fields next changes from live. actor is the user who
// saves; Rights within the actor's own are not Critical.
func criticalEdits(live, next config.Config, actor config.User) []string {
	var out []string
	add := func(on bool, what string) {
		if on && !slices.Contains(out, what) {
			out = append(out, what)
		}
	}
	add(config.Changed(live, next, "router"), "the router connection")
	for _, f := range presetFields {
		add(f.changed(live, next), f.what)
	}
	_, mine := live.ActsAs(actor)
	wider := func(was, now config.Rights) bool { return !sameRights(was, now) && !now.Within(mine) }
	for _, u := range next.Users {
		o, ok := live.FindUserByID(u.ID)
		if !ok {
			add(true, "the user list")
			continue
		}
		add(o.Role != u.Role, "a Role")
		add(o.Pin != u.Pin && u.ID != actor.ID, "another user's PIN")
		add(o.Persistent != u.Persistent, "Persistent")
		add(o.NoAutoLock != u.NoAutoLock, "No auto-lock")
		add(wider(o.Rights, u.Rights), "Rights wider than yours")
	}
	for _, u := range live.Users {
		_, ok := next.FindUserByID(u.ID)
		add(!ok, "the user list")
	}
	add(wider(live.GuestRights, next.GuestRights), "Guest Rights wider than yours")
	for _, cl := range next.APIClients {
		i := slices.IndexFunc(live.APIClients, func(o config.APIClient) bool { return o.Name == cl.Name })
		was := config.Rights{Sources: &[]config.Ref{}} // a new client is an edit from nothing
		if i >= 0 {
			was = live.APIClients[i].Rights
		}
		add(wider(was, cl.Rights), "an API client wider than your Rights")
	}
	return out
}

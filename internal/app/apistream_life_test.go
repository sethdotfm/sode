package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// life* helpers: an open stream follows its owner's rights, scopes and the API switches.

func lifeCommit(t *testing.T, a *App, mut func(*config.Config)) {
	t.Helper()
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.commit(func(c *config.Config) error { mut(c); return nil }, nil); err != nil {
		t.Fatal(err)
	}
}

func lifeTally(t *testing.T, a *App, topics string) *strmConn {
	t.Helper()
	s := strmDial(t, strmServer(t, a), botKey(t, a), topics)
	s.next() // hello
	for range strings.Split(topics, ",") {
		s.next()
	}
	return s
}

func TestStreamFollowsRightsChange(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	s := lifeTally(t, a, "tally")
	a.broadcast(deltaMsg{T: "tally", U: []routeChange{{7, 1, 5}}})
	strmWantType(t, s.next(), "tally", "tally")

	lifeCommit(t, a, func(c *config.Config) { c.APIClients[0].Rights = viewRights(config.VisHidden) })
	a.broadcast(deltaMsg{T: "tally", U: []routeChange{{7, 1, 5}}})
	s.quiet(300 * time.Millisecond)
	a.broadcast(deltaMsg{T: "tally", U: []routeChange{{2, 1, 1}}})
	strmWantType(t, s.next(), "tally", "tally")
}

func lifeEnded(t *testing.T, a *App, s *strmConn, want string) {
	t.Helper()
	rvkClosed(t, s)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range logged(t, a) {
			if e.Why == "streamDisconnect" && strings.Contains(e.Note, want) {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no streamDisconnect row mentioning %q: %+v", want, logged(t, a))
}

func TestStreamEndsWhenScopeLost(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	defer startLog(t, a)()
	s := lifeTally(t, a, "tally")
	lifeCommit(t, a, func(c *config.Config) { c.APIClients[0].Scopes = []string{config.ScopeNames} })
	lifeEnded(t, a, s, "scopes changed")
}

func TestStreamEndsWhenLogScopeRemoved(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) {
		c.APIClients[0].Scopes = append(c.APIClients[0].Scopes, config.ScopeLog)
	})
	defer startLog(t, a)()
	s := lifeTally(t, a, "status")
	lifeCommit(t, a, func(c *config.Config) {
		c.APIClients[0].Scopes = []string{config.ScopeNames, config.ScopeTally, config.ScopeStatus}
	})
	lifeEnded(t, a, s, "scopes changed")
}

func TestStreamEndsWhenAPIOff(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	defer startLog(t, a)()
	s := lifeTally(t, a, "tally")
	lifeCommit(t, a, func(c *config.Config) { c.API.Enabled = false })
	lifeEnded(t, a, s, "the API is off")
}

func TestStreamEndsWhenGuestOff(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) {
		c.API.GuestEnabled = true
		c.GuestAPIScopes = []string{config.ScopeTally}
	})
	defer startLog(t, a)()
	s := strmDial(t, strmServer(t, a), "", "tally")
	s.next()
	s.next()
	lifeCommit(t, a, func(c *config.Config) { c.API.GuestEnabled = false })
	lifeEnded(t, a, s, "the Guest API is off")
}

func TestStreamSlowDropSaysWhy(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	defer startLog(t, a)()
	s := lifeTally(t, a, "tally")
	a.mu.Lock()
	for range 5000 { // one lock hold: the writer cannot keep up with the queue
		a.broadcast(deltaMsg{T: "tally", U: []routeChange{{2, 1, 1}}})
	}
	a.mu.Unlock()
	lifeEnded(t, a, s, "too slow")
}

func lifeWantOver(t *testing.T, a *App, key string) {
	t.Helper()
	_, resp, _, cancel, err := strmDialRaw(strmServer(t, a), key, "tally")
	defer cancel()
	if err == nil || resp == nil || resp.StatusCode != 429 {
		t.Fatalf("want 429 over the cap, got %v %v", resp, err)
	}
}

func TestStreamCapsPerKey(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	srv := strmServer(t, a)
	old := apiStreamMaxPerAddr
	apiStreamMaxPerAddr = 100
	defer func() { apiStreamMaxPerAddr = old }()
	key := botKey(t, a)
	for range apiStreamMaxPerKey {
		strmDial(t, srv, key, "tally")
	}
	lifeWantOver(t, a, key)
	strmDial(t, srv, botKey(t, a), "tally") // another key is not held back
}

func TestStreamCapsPerAddress(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	srv := strmServer(t, a)
	for range apiStreamMaxPerAddr {
		strmDial(t, srv, botKey(t, a), "tally")
	}
	lifeWantOver(t, a, botKey(t, a))
}

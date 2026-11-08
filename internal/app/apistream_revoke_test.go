package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// rvk* helpers: a stream must not outlive the key it was opened with.

var rvkBot = keyOwner{Kind: actorClient, Name: "PanelsBot"}

// rvkOpen attaches a stream with a fresh key and returns it with the key's id.
func rvkOpen(t *testing.T, a *App, exp time.Time) (*strmConn, string) {
	t.Helper()
	secret, rec, err := a.keys.createCapped(rvkBot, "Panel 43", exp, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	s := strmDial(t, strmServer(t, a), secret, "status")
	s.next() // hello
	strmWantType(t, s.next(), "snapshot", "status")
	return s, rec.ID
}

func rvkClosed(t *testing.T, s *strmConn) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-s.in:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("stream still open after its key ended")
		}
	}
}

func rvkDisconnectNote(t *testing.T, a *App, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range logged(t, a) {
			if e.Why == "streamDisconnect" && strings.Contains(e.Note, want) {
				if !strings.Contains(e.By, "Panel 43") {
					t.Fatalf("row lost the key name: %+v", e)
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no streamDisconnect row mentioning %q: %+v", want, logged(t, a))
}

func TestStreamClosesWhenKeyRevoked(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	defer startLog(t, a)()
	s, id := rvkOpen(t, a, time.Time{})
	other := strmDial(t, strmServer(t, a), apiTestKey(t, a, rvkBot, "Panel 44", time.Time{}), "status")
	other.next()
	other.next()

	if _, err := a.RevokeKeys(adminClient(t, a), func(r keyRec) bool { return r.ID == id }); err != nil {
		t.Fatal(err)
	}
	rvkClosed(t, s)
	rvkDisconnectNote(t, a, "key revoked")
	other.quiet(200 * time.Millisecond) // another key's stream stays up
}

func TestStreamClosesWhenKeyExpires(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	defer startLog(t, a)()
	s, _ := rvkOpen(t, a, time.Now().Add(500*time.Millisecond))
	time.Sleep(600 * time.Millisecond)
	a.expireKeys(time.Now())
	rvkClosed(t, s)
	rvkDisconnectNote(t, a, "key expired")
}

func TestStreamClosesWhenOwnerRemoved(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	defer startLog(t, a)()
	s, _ := rvkOpen(t, a, time.Time{})
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error {
		var keep []config.APIClient
		for _, cl := range c.APIClients {
			if cl.Name != "PanelsBot" {
				keep = append(keep, cl)
			}
		}
		c.APIClients = keep
		return nil
	}, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	rvkClosed(t, s)
	rvkDisconnectNote(t, a, "owner changed or was removed")
}

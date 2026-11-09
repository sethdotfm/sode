package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A Session-mode panel takes through /api/v1/takes with the user's session key. The server,
// not the page, enforces Rights and Protected, and History names the user as the actor.

func sessionTaker(t *testing.T, mut func(*config.Config)) (*App, *fakeRouter, string) {
	t.Helper()
	a, f := apiTestApp(t, mut)
	shortTimers(a)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	return a, f, sessionKeyOf(t, a, c, next)
}

func TestPanelTakeWithSessionKeyNamesTheUser(t *testing.T) {
	a, f, key := sessionTaker(t, nil)
	f.seed(2, 1, 0)
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"result":"connected"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 1 || es[0].Actor == nil || es[0].Actor.Name != "Ana" || es[0].Client != "Ana (Session)" {
		t.Fatalf("entries %+v", es)
	}
}

func TestPanelTakeIsRefusedByTheServerOutsideRights(t *testing.T) {
	a, f, key := sessionTaker(t, func(c *config.Config) {
		c.Users[0].Rights = config.Rights{Dsts: ints(2)}
	})
	n := f.sends()
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":5,"levels":[1]}]}`, nil)
	wantErr(t, rec, 403, "outside_rights")
	if f.sends() != n || a.hist.Len() != 0 {
		t.Fatalf("a refused take sent %d messages and wrote %d entries", f.sends()-n, a.hist.Len())
	}
}

func TestPanelTakeOfProtectedDestinationFails(t *testing.T) {
	a, f, key := sessionTaker(t, nil)
	n := f.sends()
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":3,"levels":[1]}]}`, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"result":"failed"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if f.sends() != n {
		t.Fatalf("a Protected destination was sent to the router")
	}
}

package app

import (
	"net/http"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Without the "Use tie lines" right, a route across routers gets 403 tie_lines_off, with ties or without, and
// nothing goes to a router. A route on one router still goes.
func TestAPITieLinesOffRefusesRouteAcrossRouters(t *testing.T) {
	off := false
	a, fa, fb, key := tieApp(t, func(c *config.Config) { c.APIClients[0].Rights = config.Rights{TieLines: &off} })
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	for _, body := range []string{tieRoutes(uidA, [2]int{4, 2}), ends(uidA, "b2b2b2b2", 4, 2)} {
		wantErr(t, apiPost(a, "/api/v1/takes", key, body, nil), http.StatusForbidden, "tie_lines_off")
	}
	if got := log(); len(got) != 0 {
		t.Fatalf("connects %v, want none", got)
	}
	if es, _ := a.hist.Page(-1, 5); len(es) != 0 {
		t.Fatalf("entries %+v, want none", es)
	}
	if rec := apiPost(a, "/api/v1/takes", key, ends("b2b2b2b2", "b2b2b2b2", 5, 2), nil); rec.Code != http.StatusOK {
		t.Fatalf("a route on one router: %d %s", rec.Code, rec.Body)
	}
}

// The ends come first: a caller outside both learns of the ends.
func TestAPITieLinesOffAfterOutsideRights(t *testing.T) {
	off := false
	a, _, _, key := tieApp(t, func(c *config.Config) { c.APIClients[0].Rights = config.Rights{Dsts: nums(3), TieLines: &off} })
	wantErr(t, apiPost(a, "/api/v1/takes", key, ends(a.r().cfg.UID, "b2b2b2b2", 4, 2), nil), http.StatusForbidden, "outside_rights")
}

// The Access preset writes the Guest's right. A keyless Guest can send a route across routers under Open access.
// It cannot under Standard access or Secured access.
func TestAPIGuestTieLinesFollowAccessPreset(t *testing.T) {
	for key, want := range map[string]int{"open": http.StatusOK, "standard": http.StatusForbidden, "secured": http.StatusForbidden} {
		t.Run(key, func(t *testing.T) {
			p, err := config.AccessPresetByName(key)
			if err != nil {
				t.Fatal(err)
			}
			a, _, _, _ := tieApp(t, func(c *config.Config) {
				p.Apply(c)
				// Only the Guest's right differs: the keyless API and its take scope are on for every preset here.
				c.Locks = config.Locks{}
				c.API.GuestEnabled = true
				c.GuestAPIScopes = append(c.GuestAPIScopes, config.ScopeTake)
			})
			rec := apiPost(a, "/api/v1/takes", "", tieRoutes(a.r().cfg.UID, [2]int{4, 2}), nil)
			if rec.Code != want {
				t.Fatalf("%d %s, want %d", rec.Code, rec.Body, want)
			}
			if want == http.StatusForbidden {
				wantErr(t, rec, want, "tie_lines_off")
			}
		})
	}
}

// A user's key acts with the user's own right.
func TestAPIUserKeyTieLinesOff(t *testing.T) {
	off := false
	a, _, _, _ := tieApp(t, func(c *config.Config) {
		for i := range c.Users {
			c.Users[i].Rights.TieLines = &off
		}
	})
	rec := apiPost(a, "/api/v1/takes", userKey(t, a, "Ada"), tieRoutes(a.r().cfg.UID, [2]int{4, 2}), nil)
	wantErr(t, rec, http.StatusForbidden, "tie_lines_off")
}

package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestPanelTakeOnALevelOutsideRightsIsRefusedByTheServer(t *testing.T) {
	a, f, key := sessionTaker(t, func(c *config.Config) {
		c.Users[0].Rights = config.Rights{Levels: ints(2)}
	})
	n := f.sends()
	wantErr(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, nil), 403, "outside_rights")
	if f.sends() != n {
		t.Fatalf("a refused take sent %d messages", f.sends()-n)
	}
}

// The page mirrors the Rights before arming, so GET /panels/{id} tells it the caller's own.
func TestPanelReadCarriesTheCallersRights(t *testing.T) {
	a, _, key := sessionTaker(t, func(c *config.Config) {
		c.Users[0].Rights = config.Rights{Dsts: ints(2), Visibility: config.VisHidden}
	})
	id := makePanel(t, a, key, `{"name":"R"}`)
	rec := apiDo(a, "GET", "/api/v1/panels/"+id, key, nil)
	r, _ := apiBody(t, rec)["rights"].(map[string]any)
	if r == nil || r["visibility"] != "hidden" || len(r["dsts"].([]any)) != 1 || r["sources"] != nil {
		t.Fatalf("rights %v in %s", r, rec.Body.String())
	}
}

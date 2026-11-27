package app

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The level subset of a panel names its router. A plain list from a request is the one router's. A cell with levels
// and no source gets the one router. A bad shape is refused.
func TestPanelLevelsNameTheirRouter(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := userKey(t, a, "Ana")
	uid := a.r().cfg.Routers[0].UID
	levelsOf := func(id string) (any, []any) {
		t.Helper()
		p := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, key, nil))
		settings, _ := p["settings"].(map[string]any)
		cells := p["pages"].([]any)[0].(map[string]any)["cells"].([]any)
		return settings["levels"], cells
	}

	cells := `[{"r":1,"c":1,"kind":"destination","n":2,"levels":[1]},{"r":1,"c":2,"kind":"source","levels":[2]}]`
	id := makePanel(t, a, key, `{"name":"L","settings":{"levels":[2,1]},"cells":`+cells+`}`)
	got, cs := levelsOf(id)
	if want := map[string]any{uid: []any{2.0, 1.0}}; !reflect.DeepEqual(got, want) {
		t.Errorf("a plain list stored as %v, want %v", got, want)
	}
	if c := cs[0].(map[string]any); c["router"] != uid || c["levels"] == nil {
		t.Errorf("a Destination cell with a router lost its levels: %v", c)
	}
	if c := cs[1].(map[string]any); c["router"] != uid || c["levels"] == nil || c["n"] != nil {
		t.Errorf("a cell with levels and no source did not get the one router: %v", c)
	}

	other := "b2b2b2b2"
	body := fmt.Sprintf(`{"name":"M","settings":{"levels":{"":[3],%q:[1],"c3c3c3c3":[]}}}`, other)
	got, _ = levelsOf(makePanel(t, a, key, body))
	if want := map[string]any{uid: []any{3.0}, other: []any{1.0}}; !reflect.DeepEqual(got, want) {
		t.Errorf("levels by router stored as %v, want %v", got, want)
	}
	got, _ = levelsOf(makePanel(t, a, key, `{"name":"N","settings":{"levels":[]}}`))
	if got != nil {
		t.Errorf("an empty list is every level, stored as %v", got)
	}

	for _, bad := range []string{`"1"`, `["x"]`, `[0]`, `{"a":"x"}`} {
		rec := apiSend(a, "POST", "/api/v1/panels", key, `{"name":"Bad","settings":{"levels":`+bad+`}}`)
		wantErr(t, rec, 400, "bad_request")
	}
}

// A panel client's plain level list is the one router's, so it is within a user's levels on that router.
func TestPanelClientLevelsWithinTheUsers(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) {
		c.Users[0].Rights = config.Rights{Levels: config.RouterLevels{"": {1, 2}}}
	})
	key := userKey(t, a, a.r().cfg.Users[0].Name)
	if rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`,"clientRights":{"levels":[2]}}`); rec.Code != 201 {
		t.Fatalf("levels within: %d %s", rec.Code, rec.Body)
	}
	rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`,"clientRights":{"levels":[3]}}`)
	wantErr(t, rec, 403, "administrator_only")
}

// GET /tally names the router of its levels, so a panel page finds its Rights levels and level subset.
func TestTallyNamesItsRouter(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	if got := viewGet(t, a, botKey(t, a), "/api/v1/tally")["router"]; got != a.r().cfg.Routers[0].UID {
		t.Errorf("router %v, want %s", got, a.r().cfg.Routers[0].UID)
	}
}

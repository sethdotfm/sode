package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// An Everyone Checkpoint set takes the Advanced Settings PIN in the body like an Everyone
// template does: a wrong PIN is 403 trusted_required, no PIN set opens nothing.
func TestCheckpointSetEveryoneTakesThePin(t *testing.T) {
	a := panelsApp(t)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	p1 := makePanel(t, a, ana, `{"name":"P1"}`)
	withPin := func(scope, pin string) string {
		return `{"name":"All","scope":"` + scope + `","panels":["` + p1 + `"],"pin":"` + pin + `"}`
	}

	// No PIN set: a pin in the body opens nothing.
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, withPin("everyone", "9999")), 403, "trusted_required")

	h, err := config.HashPin("9999")
	if err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	cfg.SettingsPin = h
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, withPin("everyone", "1111")), 403, "trusted_required")
	rec := apiSend(a, "POST", "/api/v1/panel-sets", ana, withPin("everyone", "9999"))
	if rec.Code != 201 {
		t.Fatalf("right PIN: %d %s", rec.Code, rec.Body.String())
	}
	all := apiBody(t, rec)["id"].(string)
	if _, stored := apiBody(t, rec)["pin"]; stored {
		t.Fatal("the pin came back")
	}
	if rec := apiSend(a, "POST", "/api/v1/panel-sets", ana, withPin("me", "")); rec.Code != 201 {
		t.Fatalf("a Me set needs no PIN: %d", rec.Code)
	}

	// Bo is not Trusted and did not make it: the PIN lets him change or delete it, nothing else does.
	wantErr(t, apiSend(a, "PUT", "/api/v1/panel-sets/"+all, bo, `{"name":"Renamed"}`), 403, "not_owner")
	wantErr(t, apiSend(a, "PUT", "/api/v1/panel-sets/"+all, bo, `{"name":"Renamed","pin":"1111"}`), 403, "trusted_required")
	if rec := apiSend(a, "PUT", "/api/v1/panel-sets/"+all, bo, `{"name":"Renamed","pin":"9999"}`); rec.Code != 200 || apiBody(t, rec)["name"] != "Renamed" {
		t.Fatalf("rename with the PIN: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panel-sets/"+all, bo, ``), 403, "not_owner")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panel-sets/"+all, bo, `{"pin":"1111"}`), 403, "trusted_required")
	if rec := apiSend(a, "DELETE", "/api/v1/panel-sets/"+all, bo, `{"pin":"9999"}`); rec.Code != 204 {
		t.Fatalf("delete with the PIN: %d", rec.Code)
	}

	// The PIN does not open someone else's Me set.
	mine := apiBody(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, withPin("me", "")))["id"].(string)
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panel-sets/"+mine, bo, `{"pin":"9999"}`), 404, "not_found")
}

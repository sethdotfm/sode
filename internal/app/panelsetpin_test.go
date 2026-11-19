package app

import (
	"testing"
)

func TestCheckpointSetEveryoneNeedsATrustedUser(t *testing.T) {
	a := panelsApp(t)
	ana, bo, cy := userKey(t, a, "Ana"), userKey(t, a, "Bo"), userKey(t, a, "Cy")
	p1 := makePanel(t, a, ana, `{"name":"P1"}`)
	withPin := func(scope, pin string) string {
		return `{"name":"All","scope":"` + scope + `","panels":["` + p1 + `"],"pin":"` + pin + `"}`
	}

	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, withPin("everyone", "1111")), 403, "trusted_required")
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, withPin("everyone", "9999")), 403, "trusted_required")
	if rec := apiSend(a, "POST", "/api/v1/panel-sets", ana, withPin("me", "")); rec.Code != 201 {
		t.Fatalf("a Me set needs no Trusted user: %d", rec.Code)
	}
	rec := apiSend(a, "POST", "/api/v1/panel-sets", cy, withPin("everyone", ""))
	if rec.Code != 201 {
		t.Fatalf("Trusted user: %d %s", rec.Code, rec.Body.String())
	}
	all := apiBody(t, rec)["id"].(string)
	if _, stored := apiBody(t, rec)["pin"]; stored {
		t.Fatal("the pin came back")
	}

	wantErr(t, apiSend(a, "PUT", "/api/v1/panel-sets/"+all, bo, `{"name":"Renamed"}`), 403, "not_owner")
	wantErr(t, apiSend(a, "PUT", "/api/v1/panel-sets/"+all, bo, `{"name":"Renamed","pin":"9999"}`), 403, "not_owner")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panel-sets/"+all, bo, `{"pin":"9999"}`), 403, "not_owner")
	if rec := apiSend(a, "PUT", "/api/v1/panel-sets/"+all, cy, `{"name":"Renamed"}`); rec.Code != 200 || apiBody(t, rec)["name"] != "Renamed" {
		t.Fatalf("Trusted rename: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "DELETE", "/api/v1/panel-sets/"+all, cy, ``); rec.Code != 204 {
		t.Fatalf("Trusted delete: %d", rec.Code)
	}

	mine := apiBody(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, withPin("me", "")))["id"].(string)
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panel-sets/"+mine, bo, `{"pin":"9999"}`), 404, "not_found")
}

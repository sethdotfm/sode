package app

import (
	"fmt"
	"strings"
	"testing"
)

func reloadPath(id string) string { return "/api/v1/panels/" + id + "/instances/reload" }

func TestReloadSendsAReloadLineToOneInstanceAndLogsIt(t *testing.T) {
	a := panelsApp(t)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A","slug":"studio-a"}`)
	lobby := assignHold(t, a, id, cy, "Lobby")
	hall := assignHold(t, a, id, cy, "Hall")
	if rec := apiSend(a, "POST", reloadPath(id), cy, `{"instance":"lobby"}`); rec.Code != 200 {
		t.Fatalf("reload: %d %s", rec.Code, rec.Body.String())
	}
	if msg := assignLineOf(t, lobby); msg["t"] != "reload" {
		t.Fatalf("message %v", msg)
	}
	silent(t, hall)
	row := lastRow(t, a, "reload")
	if !strings.Contains(row.By, "Cy") || !strings.Contains(row.Note, "studio-a") || !strings.Contains(row.Note, "Lobby") || !strings.Contains(row.Note, "at 127.0.0.1") {
		t.Fatalf("row %+v", row)
	}
}

func TestReloadAKeylessPageAndEveryPageOfAPanel(t *testing.T) {
	a := panelsApp(t)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A","slug":"studio-a"}`)
	page, _ := keylessHold(t, a, "studio-a", "?instance=Wall")
	row := keylessRow(t, a, id, cy, "Wall")
	if rec := apiSend(a, "POST", reloadPath(id), cy, fmt.Sprintf(`{"keyless":%q}`, row["keyless"])); rec.Code != 200 {
		t.Fatalf("reload keyless: %d %s", rec.Code, rec.Body.String())
	}
	wantLine(t, page, "reload")

	page, _ = keylessHold(t, a, "studio-a", "?instance=Wall")
	keylessRow(t, a, id, cy, "Wall")
	lobby := assignHold(t, a, id, cy, "Lobby")
	rec := apiSend(a, "POST", reloadPath(id), cy, `{"all":true}`)
	if rec.Code != 200 || apiBody(t, rec)["sent"] != float64(2) {
		t.Fatalf("reload all: %d %s", rec.Code, rec.Body.String())
	}
	wantLine(t, page, "reload")
	wantLine(t, lobby, "reload")
	lastRow(t, a, "reload", 3)
}

func TestReloadRefusals(t *testing.T) {
	a := panelsApp(t)
	cy, bo := userKey(t, a, "Cy"), userKey(t, a, "Bo")
	id := makePanel(t, a, cy, `{"name":"Studio A","share":{"mode":"everyone"}}`)
	lobby := assignHold(t, a, id, cy, "Lobby")
	wantErr(t, apiSend(a, "POST", reloadPath(id), bo, `{"instance":"Lobby"}`), 403, "trusted_required")
	silent(t, lobby)
	wantErr(t, apiSend(a, "POST", reloadPath(id), cy, `{"instance":"Gone"}`), 409, "offline")
	wantErr(t, apiSend(a, "POST", reloadPath(id), cy, `{"keyless":"0011223344556677"}`), 409, "offline")
	empty := makePanel(t, a, cy, `{"name":"Empty"}`)
	wantErr(t, apiSend(a, "POST", reloadPath(empty), cy, `{"all":true}`), 409, "offline")
	wantErr(t, apiSend(a, "POST", reloadPath(id), cy, `{}`), 400, "bad_request")
}

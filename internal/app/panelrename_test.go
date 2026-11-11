package app

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

// REST and stream seam for renaming an instance from the panels page: the rename line goes down the instance's
// held stream, its key is renamed, and the page reopens under the new name.

func renameBody(instance, name, pin string) string {
	return fmt.Sprintf(`{"instance":%q,"name":%q,"pin":%q}`, instance, name, pin)
}

func renamePath(id string) string { return "/api/v1/panels/" + id + "/instances/rename" }

func TestRenameSendsTheNameDownTheStreamAndRenamesTheKey(t *testing.T) {
	a := panelsApp(t)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A","acts":"api","slug":"studio-a"}`)
	lobbyKey := instanceKey(t, a, id, cy, "Lobby")
	lobby := assignHold(t, a, id, lobbyKey, "Lobby")
	hall := assignHold(t, a, id, instanceKey(t, a, id, cy, "Hall"), "Hall")
	if rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/watch?instance=Lobby", cy, ""); rec.Code != 204 {
		t.Fatalf("watch: %d", rec.Code)
	}

	rec := apiSend(a, "POST", renamePath(id), cy, renameBody("lobby", " Front desk ", ""))
	if rec.Code != 200 || apiBody(t, rec)["instance"] != "Front desk" {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	msg := assignLineOf(t, lobby)
	if msg["t"] != "rename" || msg["instance"] != "Front desk" || msg["key"] != nil {
		t.Fatalf("message %v", msg)
	}
	select {
	case m := <-hall:
		t.Fatalf("another instance heard it: %v", m)
	case <-time.After(300 * time.Millisecond):
	}

	// The key keeps working under its new name, so the Link with key list agrees with the instances.
	keys := apiSend(a, "GET", "/api/v1/panels/"+id+"/keys", cy, "").Body.String()
	if !strings.Contains(keys, `"name":"Front desk"`) || strings.Contains(keys, `"name":"Lobby"`) {
		t.Fatalf("keys: %s", keys)
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+id, lobbyKey, ""); rec.Code != 200 {
		t.Fatalf("the key ended: %d", rec.Code)
	}
	// The old name goes from the list without a lost notice, and its watch follows the new name.
	for deadline := time.Now().Add(5 * time.Second); instanceRows(t, a, id, cy)["Lobby"] != nil; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the old name is still listed")
		}
	}
	if w := fmt.Sprint(panelOf(t, a, id, cy)["watchingInstances"]); w != "[front desk]" {
		t.Fatalf("watching %s", w)
	}
	assignHold(t, a, id, lobbyKey, url.QueryEscape("Front desk"))
	if instanceRows(t, a, id, cy)["Front desk"]["stream"] != true {
		t.Fatal("the new name is not listed")
	}

	row := lastRow(t, a, "rename")
	if !strings.Contains(row.By, "Cy") || !strings.Contains(row.Note, "Lobby") || !strings.Contains(row.Note, "Front desk") || !strings.Contains(row.Note, "studio-a") {
		t.Fatalf("row %+v", row)
	}
}

// A Session-mode panel has no keys: the monitor's names are all there is to rename and clash with.
func TestRenameOnASessionPanelAndACaseOnlyChange(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Desk"}`)
	lobby := assignHold(t, a, id, cy, "lobby")
	if rec := apiSend(a, "POST", renamePath(id), cy, renameBody("lobby", "Lobby", "")); rec.Code != 200 {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if msg := assignLineOf(t, lobby); msg["instance"] != "Lobby" {
		t.Fatalf("message %v", msg)
	}
}

func TestRenameRights(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	bo := userKey(t, a, "Bo")
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, ana, `{"name":"Studio A","share":{"mode":"everyone"}}`)
	lobby := assignHold(t, a, id, ana, "Lobby")

	// Neither owner nor Trusted, and no PIN or a wrong one.
	wantErr(t, apiSend(a, "POST", renamePath(id), bo, renameBody("Lobby", "Hall", "")), 403, "trusted_required")
	wantErr(t, apiSend(a, "POST", renamePath(id), bo, renameBody("Lobby", "Hall", "1111")), 403, "trusted_required")
	// A panel's own key is no user.
	client := makePanel(t, a, cy, `{"name":"Wall","acts":"api"}`)
	if rec := apiSend(a, "POST", renamePath(id), instanceKey(t, a, client, cy, "Desk"), renameBody("Lobby", "Hall", "2468")); rec.Code != 403 {
		t.Fatalf("a panel key renamed: %d %s", rec.Code, rec.Body.String())
	}

	// The owner, then the PIN, then a Trusted user. Each rename ends the stream, so the page's reopen is held again.
	steps := []struct{ key, from, to, pin string }{{ana, "Lobby", "Hall", ""}, {bo, "Hall", "Porch", "2468"}, {cy, "Porch", "Gate", ""}}
	for _, s := range steps {
		if rec := apiSend(a, "POST", renamePath(id), s.key, renameBody(s.from, s.to, s.pin)); rec.Code != 200 {
			t.Fatalf("%s to %s: %d %s", s.from, s.to, rec.Code, rec.Body.String())
		}
		if msg := assignLineOf(t, lobby); msg["instance"] != s.to {
			t.Fatalf("message %v", msg)
		}
		lobby = assignHold(t, a, id, ana, s.to)
	}
}

func TestRenameClashAnswersASuggestion(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
	assignHold(t, a, id, instanceKey(t, a, id, cy, "Lobby"), "Lobby")
	held := instanceKey(t, a, id, cy, "Hall")
	keepalive(a, id, cy, "Hall 2") // an instance with no key still holds its name

	rec := apiSend(a, "POST", renamePath(id), cy, renameBody("Lobby", "HALL", ""))
	wantErr(t, rec, 409, "name_taken")
	if got := suggestOf(t, rec); got != "HALL 3" {
		t.Fatalf("suggest %q", got)
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+id, held, ""); rec.Code != 200 {
		t.Fatalf("the held key ended: %d", rec.Code)
	}
	keys := apiSend(a, "GET", "/api/v1/panels/"+id+"/keys", cy, "").Body.String()
	if !strings.Contains(keys, `"name":"Lobby"`) {
		t.Fatalf("keys: %s", keys)
	}
}

func TestRenameRefusals(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
	assignHold(t, a, id, cy, "Lobby")
	keepalive(a, id, cy, "Hall") // a keepalive alone has no stream to tell
	instanceKey(t, a, id, cy, "Porch")
	stop := monHold(t, a, id, cy, "Porch")
	stop()
	monWaitConns(t, a, id, 1)

	cases := []struct {
		name, panel, body string
		status            int
		code              string
	}{
		{"offline", id, renameBody("Porch", "Gate", ""), 409, "conflict"},
		{"no stream", id, renameBody("Hall", "Gate", ""), 409, "conflict"},
		{"unknown instance", id, renameBody("Nobody", "Gate", ""), 409, "conflict"},
		{"no instance", id, renameBody("", "Gate", ""), 400, "bad_request"},
		{"no name", id, renameBody("Lobby", "  ", ""), 400, "bad_request"},
		{"same name", id, renameBody("Lobby", "Lobby", ""), 400, "bad_request"},
		{"unknown panel", "p-00000000", renameBody("Lobby", "Gate", ""), 404, "not_found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantErr(t, apiSend(a, "POST", renamePath(c.panel), cy, c.body), c.status, c.code)
		})
	}
	// An offline instance's key is not renamed alone.
	if instanceRows(t, a, id, cy)["Porch"] == nil {
		t.Fatal("Porch is not listed")
	}
	if keys := apiSend(a, "GET", "/api/v1/panels/"+id+"/keys", cy, "").Body.String(); strings.Contains(keys, "Gate") {
		t.Fatalf("keys: %s", keys)
	}
}

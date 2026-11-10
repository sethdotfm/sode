package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// REST and stream seam for Assign: a held instance stream, a trusted user or the PIN, a target that acts as an API client.

const assignTarget = `{"name":"Studio B","acts":"api","clientRights":{"sources":[1,2],"dsts":[2]},"settings":{"mode":"take","fixedDests":[2]}}`

// assignHold holds an instance stream like the panel page and hands back every line after the first.
func assignHold(t *testing.T, a *App, id, key, device string) <-chan map[string]any {
	t.Helper()
	srv := strmServer(t, a)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/panels/"+id+"/instances/stream?device="+device, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("stream: %v %v", resp, err)
	}
	rd := bufio.NewReader(resp.Body)
	if _, err := rd.ReadString('\n'); err != nil {
		t.Fatalf("first line: %v", err)
	}
	out := make(chan map[string]any, 8)
	go func() {
		defer close(out)
		for {
			line, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			var m map[string]any
			if json.Unmarshal([]byte(line), &m) == nil && m["t"] != nil {
				out <- m
			}
		}
	}()
	return out
}

func instanceKey(t *testing.T, a *App, panel, key, name string) string {
	t.Helper()
	rec := apiSend(a, "POST", "/api/v1/panels/"+panel+"/key", key, fmt.Sprintf(`{"instance":%q}`, name))
	if rec.Code != 201 {
		t.Fatalf("key: %d %s", rec.Code, rec.Body.String())
	}
	return apiBody(t, rec)["key"].(string)
}

func assignBody(from, instance, pin string) string {
	return fmt.Sprintf(`{"from":%q,"instance":%q,"pin":%q}`, from, instance, pin)
}

func TestAssignSendsAKeyForTheTargetDownOnlyThatInstancesStream(t *testing.T) {
	a := panelsApp(t)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api","slug":"studio-a"}`)
	to := makePanel(t, a, cy, assignTarget)
	lobbyKey := instanceKey(t, a, from, cy, "Lobby")
	hall := assignHold(t, a, from, instanceKey(t, a, from, cy, "Hall"), "Hall")
	lobby := assignHold(t, a, from, lobbyKey, "Lobby")
	before := panelOf(t, a, to, cy)

	rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "lobby", ""))
	if rec.Code != 200 {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sode_") {
		t.Fatalf("the answer carries the key: %s", rec.Body.String())
	}
	var msg map[string]any
	select {
	case msg = <-lobby:
	case <-time.After(5 * time.Second):
		t.Fatal("the instance heard nothing")
	}
	if msg["t"] != "assign" || msg["panel"] != to || msg["instance"] != "Lobby" {
		t.Fatalf("message %v", msg)
	}
	select {
	case m := <-hall:
		t.Fatalf("another instance heard it: %v", m)
	case <-time.After(300 * time.Millisecond):
	}

	// The key is the target's client, so it carries the target's rights and reaches only it.
	key, _ := msg["key"].(string)
	got := panelOf(t, a, to, key)
	if fmt.Sprint(got["rights"].(map[string]any)["dsts"]) != "[2]" {
		t.Fatalf("rights %v", got["rights"])
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+from, key, ""); rec.Code != 404 {
		t.Fatalf("the new key reaches the old panel: %d", rec.Code)
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+to+"/keys", cy, ""); !strings.Contains(rec.Body.String(), `"name":"Lobby"`) {
		t.Fatalf("keys: %s", rec.Body.String())
	}
	// The old key stays, and the target panel itself is not written.
	if rec := apiSend(a, "GET", "/api/v1/panels/"+from, lobbyKey, ""); rec.Code != 200 {
		t.Fatalf("the old key ended: %d", rec.Code)
	}
	after := panelOf(t, a, to, cy)
	if after["rev"] != before["rev"] || fmt.Sprint(after["settings"].(map[string]any)["fixedDests"]) != "[2]" || after["acts"] != "api" {
		t.Fatalf("the target changed: %v -> %v", before, after)
	}

	// The move is deliberate: the source panel forgets the instance instead of losing it later.
	for deadline := time.Now().Add(5 * time.Second); strings.Contains(fmt.Sprint(panelOf(t, a, from, cy)["instances"]), "Lobby"); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the source panel still holds the instance")
		}
	}

	row := lastRow(t, a, "assign")
	if !strings.Contains(row.By, "Cy") || !strings.Contains(row.Note, "Lobby") || !strings.Contains(row.Note, "studio-a") || !strings.Contains(row.Note, "Studio B") {
		t.Fatalf("row %+v", row)
	}
	if strings.Contains(row.Note, key) {
		t.Fatal("the row carries the key")
	}
}

func TestAssignNeedsATrustedUserOrThePin(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	from := makePanel(t, a, ana, `{"name":"Studio A"}`)
	cy := userKey(t, a, "Cy")
	to := makePanel(t, a, cy, assignTarget)
	assignHold(t, a, from, ana, "Lobby")

	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", ana, assignBody(from, "Lobby", "")), 403, "trusted_required")
	h, err := config.HashPin("9999")
	if err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	cfg.SettingsPin = h
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", ana, assignBody(from, "Lobby", "1111")), 403, "trusted_required")
	if rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", ana, assignBody(from, "Lobby", "9999")); rec.Code != 200 {
		t.Fatalf("with the PIN: %d %s", rec.Code, rec.Body.String())
	}
	// A panel's own key cannot assign, even with the PIN.
	client := instanceKey(t, a, to, cy, "Desk")
	if rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", client, assignBody(from, "Lobby", "9999")); rec.Code != 403 {
		t.Fatalf("a panel key assigned: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAssignRefusals(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A"}`)
	to := makePanel(t, a, cy, assignTarget)
	session := makePanel(t, a, cy, `{"name":"Desk"}`)
	assignHold(t, a, from, cy, "Lobby")
	keepalive(a, from, cy, "Hall") // a keepalive alone is no Sode page to send to

	cases := []struct {
		name, target, body string
		status             int
		code               string
	}{
		{"session target", session, assignBody(from, "Lobby", ""), 409, "conflict"},
		{"same panel", from, assignBody(from, "Lobby", ""), 409, "conflict"},
		{"no stream", to, assignBody(from, "Hall", ""), 409, "conflict"},
		{"unknown instance", to, assignBody(from, "Nobody", ""), 409, "conflict"},
		{"no instance", to, assignBody(from, "", ""), 400, "bad_request"},
		{"unknown from", to, assignBody("p-00000000", "Lobby", ""), 404, "not_found"},
		{"unknown target", "p-00000000", assignBody(from, "Lobby", ""), 404, "not_found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+c.target+"/assign", cy, c.body), c.status, c.code)
		})
	}

	// A name the target already holds is another device's: its key is not rotated away.
	held := instanceKey(t, a, to, cy, "Lobby")
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "Lobby", "")), 409, "conflict")
	if rec := apiSend(a, "GET", "/api/v1/panels/"+to, held, ""); rec.Code != 200 {
		t.Fatalf("the held key ended: %d", rec.Code)
	}
}

func TestPanelReadSaysWhichInstancesHoldAStream(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A"}`)
	assignHold(t, a, id, cy, "Lobby")
	keepalive(a, id, cy, "Hall")
	stream := map[string]bool{}
	for _, in := range panelOf(t, a, id, cy)["instances"].([]any) {
		m := in.(map[string]any)
		stream[m["device"].(string)] = m["stream"] == true
	}
	if !stream["Lobby"] || stream["Hall"] {
		t.Fatalf("stream %v", stream)
	}
}

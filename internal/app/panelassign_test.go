package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// Anyone who may use a panel can hold a stream under any device name, so the key goes to the newest stream alone.
func TestAssignSendsTheKeyToTheNewestStreamOfADeviceOnly(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
	to := makePanel(t, a, cy, assignTarget)
	lobbyKey := instanceKey(t, a, from, cy, "Lobby")
	older := assignHold(t, a, from, lobbyKey, "Lobby")
	newest := assignHold(t, a, from, lobbyKey, "Lobby")

	if rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "Lobby", "")); rec.Code != 200 {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	if msg := assignLineOf(t, newest); msg["key"] == nil {
		t.Fatalf("message %v", msg)
	}
	select {
	case m, ok := <-older:
		if ok {
			t.Fatalf("the older stream heard it: %v", m)
		}
	case <-time.After(300 * time.Millisecond):
	}
}

// A key whose line no stream wrote is revoked and the Assign fails: the stream closed with the line still queued,
// or its write failed.
func TestAssignFailsAndDropsTheKeyWhenNoStreamWroteTheLine(t *testing.T) {
	cases := []struct {
		name  string
		open  func(a *App, from string, cancel context.CancelFunc) (body string, drop func(), out <-chan lastLine)
		close bool // the stream closes with the line queued; else its write fails
	}{
		{"stream closed", openTestStream, true},
		{"write failed", openTestStream, false},
		{"keyless stream closed", openTestKeyless, true},
		{"keyless write failed", openTestKeyless, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := panelsApp(t)
			defer startLog(t, a)()
			cy := userKey(t, a, "Cy")
			from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
			to := makePanel(t, a, cy, assignTarget)
			_, cancel := context.WithCancel(context.Background())
			defer cancel()
			body, drop, out := c.open(a, from, cancel)
			defer drop()
			go func() {
				if c.close {
					for len(out) == 0 {
						time.Sleep(5 * time.Millisecond)
					}
					drop()
					return
				}
				l := <-out
				l.got <- false
			}()

			start := time.Now()
			wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, body), 409, "conflict")
			if time.Since(start) > 3*time.Second {
				t.Fatalf("the refusal took %v", time.Since(start))
			}
			if rec := apiSend(a, "GET", "/api/v1/panels/"+to+"/keys", cy, ""); strings.Contains(rec.Body.String(), `"name":"Lobby"`) {
				t.Fatalf("the key outlived the line: %s", rec.Body.String())
			}
			for _, r := range logged(t, a) {
				if r.Why == "assign" && r.Result == "ok" {
					t.Fatalf("row %+v", r)
				}
			}
		})
	}
}

func openTestStream(a *App, from string, cancel context.CancelFunc) (string, func(), <-chan lastLine) {
	done, out, _, err := a.mon.openStream(from, "Lobby", streamWho{addr: "192.0.2.7", signIn: "client"}, cancel)
	if err != nil {
		panic(err)
	}
	return assignBody(from, "Lobby", ""), sync.OnceFunc(done), out
}

func openTestKeyless(a *App, from string, cancel context.CancelFunc) (string, func(), <-chan lastLine) {
	done, out := a.mon.openKeyless(from, "192.0.2.7", "Lobby", false, cancel)
	return `{"keyless":"` + a.mon.keylessOf(Panel{ID: from})[0].Keyless + `"}`, sync.OnceFunc(done), out
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
		{"client key from a session target", session, `{"from":"` + from + `","instance":"Lobby","as":"client"}`, 409, "conflict"},
		{"sign-in on an api target", to, `{"from":"` + from + `","instance":"Lobby","as":"user"}`, 409, "conflict"},
		{"unknown choice", to, `{"from":"` + from + `","instance":"Lobby","as":"guest"}`, 400, "bad_request"},
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
}

func assignAs(from, instance, name string) string {
	return fmt.Sprintf(`{"from":%q,"instance":%q,"name":%q}`, from, instance, name)
}

func suggestOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	e, _ := apiBody(t, rec)["error"].(map[string]any)
	s, _ := e["suggest"].(string)
	return s
}

func TestAssignNameClashAnswersASuggestionAndTakesTheChosenName(t *testing.T) {
	a := panelsApp(t)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
	to := makePanel(t, a, cy, assignTarget)
	lobby := assignHold(t, a, from, instanceKey(t, a, from, cy, "Lobby"), "Lobby")
	held := instanceKey(t, a, to, cy, "lobby")
	instanceKey(t, a, to, cy, "Lobby 2")

	// A name the target already holds is another device's: its key is not rotated away.
	rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "Lobby", ""))
	wantErr(t, rec, 409, "name_taken")
	if got := suggestOf(t, rec); got != "Lobby 3" {
		t.Fatalf("suggest %q", got)
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+to, held, ""); rec.Code != 200 {
		t.Fatalf("the held key ended: %d", rec.Code)
	}
	// A chosen name that clashes too is answered the same way.
	rec = apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignAs(from, "Lobby", "LOBBY 2"))
	wantErr(t, rec, 409, "name_taken")
	if got := suggestOf(t, rec); got != "LOBBY 3" {
		t.Fatalf("suggest %q", got)
	}

	rec = apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignAs(from, "lobby", " Lobby desk "))
	if rec.Code != 200 || apiBody(t, rec)["instance"] != "Lobby desk" {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	var msg map[string]any
	select {
	case msg = <-lobby:
	case <-time.After(5 * time.Second):
		t.Fatal("the instance heard nothing")
	}
	if msg["instance"] != "Lobby desk" {
		t.Fatalf("message %v", msg)
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+to+"/keys", cy, ""); !strings.Contains(rec.Body.String(), `"name":"Lobby desk"`) {
		t.Fatalf("keys: %s", rec.Body.String())
	}
	row := lastRow(t, a, "assign")
	if !strings.Contains(row.Note, "instance Lobby") || !strings.Contains(row.Note, "as Lobby desk") {
		t.Fatalf("row %+v", row)
	}
}

// The clash counts every instance the target holds, as a rename does, not only its keys: here one open without a key
// of that name.
func TestAssignNameClashCountsTheTargetsOpenInstances(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
	to := makePanel(t, a, cy, assignTarget)
	assignHold(t, a, from, instanceKey(t, a, from, cy, "Lobby"), "Lobby")
	assignHold(t, a, to, instanceKey(t, a, to, cy, "Desk"), "lobby")

	rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "Lobby", ""))
	wantErr(t, rec, 409, "name_taken")
	if got := suggestOf(t, rec); got != "Lobby 2" {
		t.Fatalf("suggest %q", got)
	}
}

// The panel an instance left still holds its old key, so assigning it back is a name clash, and the old key stays.
func TestAssignBackOffersANewName(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
	to := makePanel(t, a, cy, assignTarget)
	lobbyKey := instanceKey(t, a, from, cy, "Lobby")
	lobby := assignHold(t, a, from, lobbyKey, "Lobby")
	if rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "Lobby", "")); rec.Code != 200 {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	msg := <-lobby
	assignHold(t, a, to, msg["key"].(string), "Lobby")

	rec := apiSend(a, "POST", "/api/v1/panels/"+from+"/assign", cy, assignBody(to, "Lobby", ""))
	wantErr(t, rec, 409, "name_taken")
	if got := suggestOf(t, rec); got != "Lobby 2" {
		t.Fatalf("suggest %q", got)
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+from, lobbyKey, ""); rec.Code != 200 {
		t.Fatalf("the old key ended: %d", rec.Code)
	}
	if rec := apiSend(a, "POST", "/api/v1/panels/"+from+"/assign", cy, assignAs(to, "Lobby", "Lobby 2")); rec.Code != 200 {
		t.Fatalf("assign back: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAssignRefusedAtTheInstanceCap(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A"}`)
	to := makePanel(t, a, cy, assignTarget)
	assignHold(t, a, from, cy, "Lobby")
	for i := 1; i < panelKeyMax; i++ {
		instanceKey(t, a, to, cy, fmt.Sprintf("Wall %d", i))
	}
	instanceKey(t, a, to, cy, "Lobby")
	// At the cap a clash offers no new name, since none would fit.
	for _, body := range []string{assignBody(from, "Lobby", ""), assignAs(from, "Lobby", "Lobby desk")} {
		rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, body)
		wantErr(t, rec, 409, "conflict")
		if !strings.Contains(rec.Body.String(), "20 instances") {
			t.Fatalf("no reason: %s", rec.Body.String())
		}
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+to+"/keys", cy, ""); strings.Contains(rec.Body.String(), "Lobby desk") {
		t.Fatalf("a key was made: %s", rec.Body.String())
	}
}

func TestSuggestInstanceName(t *testing.T) {
	long := strings.Repeat("x", panelInstanceMax)
	cases := []struct {
		name  string
		taken []string
		want  string
	}{
		{"Lobby", []string{"Lobby"}, "Lobby 2"},
		{"Lobby", []string{"lobby", "LOBBY 2", "Lobby 4"}, "Lobby 3"},
		{"Lobby 2", []string{"Lobby 2"}, "Lobby 3"},
		{"Lobby 9", []string{"Lobby 9", "Lobby 10"}, "Lobby 11"},
		{"Cam2", []string{"Cam2"}, "Cam2 2"},
		{long, []string{long}, long[:panelInstanceMax-2] + " 2"},
	}
	for _, c := range cases {
		if got := suggestInstance(c.name, c.taken); got != c.want {
			t.Errorf("suggestInstance(%q, %v) = %q, want %q", c.name, c.taken, got, c.want)
		}
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

func instanceRows(t *testing.T, a *App, id, key string) map[string]map[string]any {
	t.Helper()
	rows := map[string]map[string]any{}
	for _, in := range panelOf(t, a, id, key)["instances"].([]any) {
		m := in.(map[string]any)
		rows[m["device"].(string)] = m
	}
	return rows
}

func TestInstanceRowsSayWhoHoldsTheStreamFromWhereAndSince(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A","acts":"both","share":{"mode":"everyone"}}`)
	before := time.Now().Add(-time.Second)
	monHold(t, a, id, instanceKey(t, a, id, cy, "Wall"), "Wall")
	monHold(t, a, id, cy, "Desk")
	monHold(t, a, id, userKey(t, a, "Ana"), "Bench")
	keepalive(a, id, cy, "Hall")

	rows := instanceRows(t, a, id, cy)
	wall, desk, hall := rows["Wall"], rows["Desk"], rows["Hall"]
	if wall["signIn"] != "client" || wall["user"] != nil || wall["addr"] != "127.0.0.1" || wall["page"] != true || wall["trusted"] != nil {
		t.Fatalf("wall %v", wall)
	}
	// trusted is read live, so the Assign to list can tell which panels the user may open.
	if desk["signIn"] != "user" || desk["user"] != "Cy" || desk["trusted"] != true {
		t.Fatalf("desk %v", desk)
	}
	if bench := rows["Bench"]; bench["user"] != "Ana" || bench["trusted"] != nil {
		t.Fatalf("bench %v", bench)
	}
	since, err := time.Parse(time.RFC3339Nano, fmt.Sprint(wall["streamSince"]))
	if err != nil || since.Before(before) || since.After(time.Now()) {
		t.Fatalf("streamSince %v %v", wall["streamSince"], err)
	}
	// A keepalive alone is no Sode panel page.
	if hall["page"] == true || hall["streamSince"] != nil || hall["signIn"] != nil {
		t.Fatalf("hall %v", hall)
	}
}

func TestATimedOutInstanceIsListedButCannotBeAssigned(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A"}`)
	to := makePanel(t, a, cy, assignTarget)
	stop := monHold(t, a, from, cy, "Lobby")
	stop()
	monWaitConns(t, a, from, 0)

	lobby := instanceRows(t, a, from, cy)["Lobby"]
	if lobby["page"] != true || lobby["stream"] == true || lobby["streamSince"] != nil || lobby["addr"] != "127.0.0.1" || lobby["signIn"] != "user" {
		t.Fatalf("lobby %v", lobby)
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "Lobby", "")), 409, "conflict")
	if rec := apiSend(a, "GET", "/api/v1/panels/"+to+"/keys", cy, ""); strings.Contains(rec.Body.String(), "Lobby") {
		t.Fatalf("a key was made: %s", rec.Body.String())
	}
}

// assignLine waits for the instance's assign line.
func assignLineOf(t *testing.T, ch <-chan map[string]any) map[string]any {
	t.Helper()
	select {
	case m := <-ch:
		return m
	case <-time.After(5 * time.Second):
		t.Fatal("the instance heard nothing")
	}
	return nil
}

// A panel that acts as a user gets no key: the device opens it and shows its own sign-in.
func TestAssignToAUserModePanelMakesNoKey(t *testing.T) {
	a := panelsApp(t)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
	to := makePanel(t, a, cy, `{"name":"Desk","slug":"desk","settings":{"mode":"take","fixedDests":[2]}}`)
	lobby := assignHold(t, a, from, instanceKey(t, a, from, cy, "Lobby"), "Lobby")
	before, keys := panelOf(t, a, to, cy), len(a.keys.list())

	rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "Lobby", ""))
	if rec.Code != 200 {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	msg := assignLineOf(t, lobby)
	if msg["panel"] != to || msg["slug"] != "desk" || msg["instance"] != "Lobby" || msg["signIn"] != "user" || msg["key"] != nil {
		t.Fatalf("message %v", msg)
	}
	if n := len(a.keys.list()); n != keys {
		t.Fatalf("keys %d -> %d", keys, n)
	}
	after := panelOf(t, a, to, cy)
	if after["rev"] != before["rev"] || after["acts"] != "session" || fmt.Sprint(after["settings"].(map[string]any)["fixedDests"]) != "[2]" {
		t.Fatalf("the target changed: %v -> %v", before, after)
	}
	if row := lastRow(t, a, "assign"); !strings.Contains(row.Note, "Desk") || !strings.Contains(row.Note, "sign-in") {
		t.Fatalf("row %+v", row)
	}
}

// A panel that acts as both gives a key unless the Assign asks for the sign-in.
func TestAssignToABothPanelHonoursTheChoice(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
	to := makePanel(t, a, cy, `{"name":"Studio B","acts":"both"}`)
	lobby := assignHold(t, a, from, instanceKey(t, a, from, cy, "Lobby"), "Lobby")
	hall := assignHold(t, a, from, instanceKey(t, a, from, cy, "Hall"), "Hall")

	if rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "Lobby", "")); rec.Code != 200 {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	if m := assignLineOf(t, lobby); m["key"] == nil || m["signIn"] != nil {
		t.Fatalf("default %v", m)
	}
	keys := len(a.keys.list())
	if rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, `{"from":"`+from+`","instance":"Hall","as":"user"}`); rec.Code != 200 {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	if m := assignLineOf(t, hall); m["key"] != nil || m["signIn"] != "user" {
		t.Fatalf("sign-in %v", m)
	}
	if n := len(a.keys.list()); n != keys {
		t.Fatalf("keys %d -> %d", keys, n)
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+to+"/keys", cy, ""); strings.Contains(rec.Body.String(), "Hall") {
		t.Fatalf("keys: %s", rec.Body.String())
	}
}

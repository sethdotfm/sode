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
)

// Keyless instances: a panel page with no working key announces itself, is listed for Assign and gets nothing until one.

// keylessHold opens the announcement with no key and hands back every line after the first.
func keylessHold(t *testing.T, a *App, seg, query string) (<-chan map[string]any, context.CancelFunc) {
	t.Helper()
	srv := strmServer(t, a)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/panels/"+seg+"/instances/keyless"+query, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("keyless: %v %v", resp, err)
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
	return out, cancel
}

func keylessRows(t *testing.T, a *App, id, key string) []map[string]any {
	t.Helper()
	rows := []map[string]any{}
	for _, r := range panelOf(t, a, id, key)["keyless"].([]any) {
		rows = append(rows, r.(map[string]any))
	}
	return rows
}

// keylessRow waits for the panel to list one keyless row named device.
func keylessRow(t *testing.T, a *App, id, key, device string) map[string]any {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		for _, r := range keylessRows(t, a, id, key) {
			if r["device"] == device {
				return r
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no keyless row %q in %v", device, keylessRows(t, a, id, key))
		}
	}
}

func silent(t *testing.T, ch <-chan map[string]any) {
	t.Helper()
	select {
	case m := <-ch:
		t.Fatalf("the keyless page heard %v", m)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestKeylessPageIsListedByItsNameAndAddress(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A","slug":"studio-a"}`)
	lines, _ := keylessHold(t, a, "studio-a", "?instance=Lobby")
	r := keylessRow(t, a, id, cy, "Lobby")
	if r["addr"] != "127.0.0.1" || r["signIn"] != "none" || r["stream"] != true || r["streamSince"] == nil || r["keyless"] == "" {
		t.Fatalf("row %v", r)
	}
	// Listed only: keyless rows are not instances, so the panel's counts and alarms ignore them.
	if p := panelOf(t, a, id, cy); p["instancesKnown"].(float64) != 0 || len(p["instances"].([]any)) != 0 {
		t.Fatalf("counted as an instance: %v", p)
	}
	silent(t, lines)
}

func TestKeylessPageWithAnEndedKeyReadsKeyEnded(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A"}`)
	keylessHold(t, a, id, "?instance=Lobby&ended=1")
	if r := keylessRow(t, a, id, cy, "Lobby"); r["signIn"] != "ended" {
		t.Fatalf("row %v", r)
	}
}

func TestKeylessNameFallsBackToTheAddressThenItsLastName(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Studio A","acts":"api"}`)
	keylessHold(t, a, id, "")
	keylessRow(t, a, id, cy, "127.0.0.1")
	// A stream from the same address that carried a name: the nameless page is shown under it.
	monHold(t, a, id, instanceKey(t, a, id, cy, "Hall"), "Hall")
	keylessRow(t, a, id, cy, "Hall")
}

func TestKeylessAnswersAlikeForAPanelThatDoesNotExist(t *testing.T) {
	a := panelsApp(t)
	keylessHold(t, a, "no-such-panel", "?instance=Lobby")
	if rec := apiSend(a, "GET", "/api/v1/panels/"+strings.Repeat("x", 80)+"/instances/keyless", "", ""); rec.Code != 400 {
		t.Fatalf("a malformed address: %d", rec.Code)
	}
}

func TestKeylessEntriesAreCappedOldestFirstAndForgottenAfterADay(t *testing.T) {
	a, clk := monApp(t)
	for i := range keylessMax + 1 {
		done, _ := a.mon.openKeyless("wall", fmt.Sprintf("10.0.%d.%d", i/256, i%256), "", false, func() {})
		done()
		clk.advance(time.Second)
	}
	a.mon.mu.Lock()
	n := len(a.mon.keyless)
	_, first := a.mon.keyless[keylessKey("wall", "10.0.0.0", "")]
	_, last := a.mon.keyless[keylessKey("wall", "10.0.3.232", "")]
	a.mon.mu.Unlock()
	if n != keylessMax || first || !last {
		t.Fatalf("%d entries, first kept %v, last kept %v", n, first, last)
	}
	clk.advance(keylessKeep + time.Minute)
	a.monSweep()
	a.mon.mu.Lock()
	n = len(a.mon.keyless)
	a.mon.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d entries after a day", n)
	}
}

func keylessAssign(a *App, to, key, id, extra string) (int, map[string]any) {
	rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", key, `{"keyless":"`+id+`"`+extra+`}`)
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	return rec.Code, m
}

func TestKeylessPageGetsAKeyOnlyByAnAssign(t *testing.T) {
	a := panelsApp(t)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","slug":"studio-a"}`)
	to := makePanel(t, a, cy, assignTarget)
	lines, _ := keylessHold(t, a, "studio-a", "?instance=Lobby")
	r := keylessRow(t, a, from, cy, "Lobby")
	silent(t, lines)

	// A user who is not Trusted, with no PIN, sends nothing.
	if code, _ := keylessAssign(a, to, userKey(t, a, "Ana"), r["keyless"].(string), ""); code != 403 {
		t.Fatalf("an untrusted Assign: %d", code)
	}
	silent(t, lines)

	if code, m := keylessAssign(a, to, cy, r["keyless"].(string), ""); code != 200 {
		t.Fatalf("assign: %d %v", code, m)
	}
	msg := assignLineOf(t, lines)
	key, _ := msg["key"].(string)
	if msg["panel"] != to || msg["instance"] != "Lobby" || key == "" {
		t.Fatalf("message %v", msg)
	}
	if got := panelOf(t, a, to, key); fmt.Sprint(got["rights"].(map[string]any)["dsts"]) != "[2]" {
		t.Fatalf("rights %v", got["rights"])
	}
	if len(keylessRows(t, a, from, cy)) != 0 {
		t.Fatal("the assigned page is still listed as keyless")
	}
	row := lastRow(t, a, "assign")
	if !strings.Contains(row.Note, "Lobby") || !strings.Contains(row.Note, "studio-a") || !strings.Contains(row.Note, "Studio B") || !strings.Contains(row.Note, "127.0.0.1") {
		t.Fatalf("row %+v", row)
	}
}

func TestKeylessPageGetsAKeyOnlyInsideTheAllowedNetworks(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A"}`)
	to := makePanel(t, a, cy, assignTarget)
	lines, _ := keylessHold(t, a, from, "?instance=Lobby")
	id := keylessRow(t, a, from, cy, "Lobby")["keyless"].(string)

	if rec := putNetworks(a, cy, to, `["10.0.0.0/8"]`); rec.Code != 200 {
		t.Fatalf("networks: %d %s", rec.Code, rec.Body.String())
	}
	if code, m := keylessAssign(a, to, cy, id, ""); code != 403 || m["error"].(map[string]any)["code"] != "network_not_allowed" {
		t.Fatalf("outside: %d %v", code, m)
	}
	silent(t, lines)
	if rec := apiSend(a, "GET", "/api/v1/panels/"+to+"/keys", cy, ""); strings.Contains(rec.Body.String(), "Lobby") {
		t.Fatalf("a key was made: %s", rec.Body.String())
	}

	putNetworks(a, cy, to, `["127.0.0.0/8"]`)
	if code, m := keylessAssign(a, to, cy, id, ""); code != 200 {
		t.Fatalf("inside: %d %v", code, m)
	}
	if assignLineOf(t, lines)["key"] == nil {
		t.Fatal("no key inside the networks")
	}
}

func TestKeylessPageCanBeAssignedToThePanelItShows(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, assignTarget)
	lines, _ := keylessHold(t, a, id, "?instance=Lobby&ended=1")
	kid := keylessRow(t, a, id, cy, "Lobby")["keyless"].(string)
	if code, m := keylessAssign(a, id, cy, kid, ""); code != 200 {
		t.Fatalf("assign: %d %v", code, m)
	}
	if msg := assignLineOf(t, lines); msg["panel"] != id || msg["key"] == nil {
		t.Fatalf("message %v", msg)
	}
}

func TestKeylessPageAssignedToAUserModePanelGetsNoKey(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A"}`)
	to := makePanel(t, a, cy, `{"name":"Desk","slug":"desk"}`)
	lines, _ := keylessHold(t, a, from, "")
	kid := keylessRow(t, a, from, cy, "127.0.0.1")["keyless"].(string)
	if code, m := keylessAssign(a, to, cy, kid, `,"name":"Front desk"`); code != 200 {
		t.Fatalf("assign: %d %v", code, m)
	}
	msg := assignLineOf(t, lines)
	if msg["signIn"] != "user" || msg["key"] != nil || msg["instance"] != "Front desk" || msg["slug"] != "desk" {
		t.Fatalf("message %v", msg)
	}
}

func TestKeylessAssignNeedsAnOpenPage(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A"}`)
	to := makePanel(t, a, cy, assignTarget)
	_, cancel := keylessHold(t, a, from, "?instance=Lobby")
	kid := keylessRow(t, a, from, cy, "Lobby")["keyless"].(string)
	cancel()
	for deadline := time.Now().Add(5 * time.Second); keylessRow(t, a, from, cy, "Lobby")["stream"] == true; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the closed page still holds a stream")
		}
	}
	if code, _ := keylessAssign(a, to, cy, kid, ""); code != 409 {
		t.Fatalf("a closed page: %d", code)
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+to+"/keys", cy, ""); strings.Contains(rec.Body.String(), "Lobby") {
		t.Fatalf("a key was made: %s", rec.Body.String())
	}
}

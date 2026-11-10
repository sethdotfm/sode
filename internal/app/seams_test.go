package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Seams between Assign, Fixed destination, artwork, keyless instances and the idle Lock.

// unwritten is the part of a panel an Assign must leave alone, on either end.
func unwritten(t *testing.T, a *App, id, key string) string {
	t.Helper()
	p := panelOf(t, a, id, key)
	return fmt.Sprint(p["rev"], p["acts"], p["settings"].(map[string]any)["fixedDests"])
}

// A panel with fixed destinations sends an instance to another one: neither panel is written, and the key the
// instance hears patches the target's fixed destinations, within the target's client rights only.
func TestSeamAssignBetweenFixedDestinationPanels(t *testing.T) {
	a, f := apiTestApp(t, func(c *config.Config) {
		cy := mkUser(t, "Cy", "4321")
		cy.Trusted = true
		c.Users = append(c.Users, cy)
	})
	shortTimers(a)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api","slug":"studio-a","settings":{"mode":"take","fixedDests":[5]}}`)
	to := makePanel(t, a, cy, `{"name":"Studio B","acts":"api","slug":"studio-b","clientRights":{"sources":[1,4],"dsts":[2,5]},"settings":{"mode":"take","fixedDests":[2,5]}}`)
	lobby := assignHold(t, a, from, instanceKey(t, a, from, cy, "Lobby"), "Lobby")
	beforeFrom, beforeTo := unwritten(t, a, from, cy), unwritten(t, a, to, cy)

	if rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", cy, assignBody(from, "Lobby", "")); rec.Code != 200 {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	msg := assignLineOf(t, lobby)
	key, _ := msg["key"].(string)
	if msg["slug"] != "studio-b" || key == "" {
		t.Fatalf("message %v", msg)
	}
	if got := unwritten(t, a, from, cy); got != beforeFrom {
		t.Errorf("the source panel changed: %s -> %s", beforeFrom, got)
	}
	if got := unwritten(t, a, to, cy); got != beforeTo || !strings.Contains(got, "[2 5]") {
		t.Errorf("the target changed: %s -> %s", beforeTo, got)
	}
	// The page reads its fixed destinations from the panel it was sent to.
	if got := fmt.Sprint(panelOf(t, a, to, key)["settings"].(map[string]any)["fixedDests"]); got != "[2 5]" {
		t.Fatalf("fixed destinations through the new key: %s", got)
	}

	f.seed(2, 1, 0)
	f.seed(5, 1, 0)
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]},{"src":4,"dst":5,"levels":[1]}]}`, nil)
	if rec.Code != 200 || strings.Count(rec.Body.String(), `"result":"connected"`) != 2 {
		t.Fatalf("take to the fixed destinations: %d %s", rec.Code, rec.Body.String())
	}
	n := f.sends()
	wantErr(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":6,"levels":[1]}]}`, nil), 403, "outside_rights")
	if f.sends() != n {
		t.Fatal("a take outside the target's rights reached the router")
	}
}

// An Assign of a keyless page writes neither panel either, by key or by sign-in, so it never changes acts and the
// editor's Authentication mode Confirm has nothing to guard (panelassign.js confirmModeChange).
func TestSeamKeylessAssignNeverWritesActs(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	from := makePanel(t, a, cy, `{"name":"Studio A","acts":"api","slug":"studio-a","settings":{"mode":"take","fixedDests":[5]}}`)
	to := makePanel(t, a, cy, `{"name":"Studio B","acts":"both","slug":"studio-b","settings":{"mode":"take","fixedDests":[2]}}`)
	beforeFrom, beforeTo := unwritten(t, a, from, cy), unwritten(t, a, to, cy)
	for _, as := range []string{"client", "user"} {
		lines, _ := keylessHold(t, a, "studio-a", "?instance=Lobby%20"+as+"&ended=1")
		kid := keylessRow(t, a, from, cy, "Lobby "+as)["keyless"].(string)
		if code, m := keylessAssign(a, to, cy, kid, `,"as":"`+as+`"`); code != 200 {
			t.Fatalf("%s: %d %v", as, code, m)
		}
		if m := assignLineOf(t, lines); (m["key"] != nil) != (as == "client") {
			t.Fatalf("%s: message %v", as, m)
		}
		if got := unwritten(t, a, from, cy); got != beforeFrom {
			t.Errorf("%s: the source panel changed: %s -> %s", as, beforeFrom, got)
		}
		if got := unwritten(t, a, to, cy); got != beforeTo {
			t.Errorf("%s: the target changed: %s -> %s", as, beforeTo, got)
		}
	}
}

// A panel with an Icon library icon, a Default artwork cell and its own images is Pushed, Copied and saved as a
// Template: library icons go as references, Default artwork is never frozen into a cell, the 64 MB total holds on
// the target and no new bytes past 2 MB or 1024 by 1024 get in by any path.
func TestSeamArtworkLimitsHoldAcrossPushCopyTemplateAndDefaults(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	ref := "lib:" + libIcon(t, a, cy, "Logo")
	src := makePanel(t, a, ana, `{"name":"From"}`)
	img := apiBody(t, apiSendRaw(a, "POST", "/api/v1/panels/"+src+"/artwork", ana, bigSVG()))["id"].(string)
	cells := `[{"kind":"source","n":1,"r":1,"c":1,"art":"` + ref + `","artB":"2"},{"kind":"source","n":2,"r":1,"c":2,"artB":"A"},{"kind":"source","n":3,"r":1,"c":3,"art":"` + img + `"}]`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+src, ana, `{"name":"From","cells":`+cells+`}`); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":2,"art":"`+ref+`"}`); rec.Code != 200 {
		t.Fatalf("library default: %d %s", rec.Code, rec.Body.String())
	}

	// A target with less room than the image: the push keeps the cells and reports the image.
	dst := makePanel(t, a, ana, `{"name":"To"}`)
	fakeFull(t, a, dst, 1000)
	code, body := pushTo(a, ana, src, dst)
	if code != 200 {
		t.Fatalf("push: %d %v", code, body)
	}
	if got := fmt.Sprint(body["pushed"].([]any)[0].(map[string]any)["artSkipped"]); got != "["+img+"]" {
		t.Errorf("skipped %s", got)
	}
	got, _ := a.panels.get(dst)
	if artBytes(got) > panelArtTotal {
		t.Errorf("the target holds %d bytes", artBytes(got))
	}
	if got.Cells[0]["art"] != ref || got.Cells[0]["artB"] != "2" || got.Cells[1]["art"] != nil || got.Cells[1]["artB"] != "A" {
		t.Errorf("pushed cells %v", got.Cells)
	}

	cp := apiBody(t, apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", ana, ""))
	if cp["artSkipped"] != nil || len(cp["art"].([]any)) != 1 {
		t.Errorf("copy: %v %v", cp["artSkipped"], cp["art"])
	}
	if c := cp["cells"].([]any)[1].(map[string]any); c["art"] != nil {
		t.Errorf("the copy froze the default: %v", c)
	}

	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"T","scope":"me","sources":true,"cells":`+cells+`}`)
	if rec.Code != 201 {
		t.Fatalf("template: %d %s", rec.Code, rec.Body.String())
	}
	tb, _ := json.Marshal(apiBody(t, rec))
	if strings.Contains(string(tb), img) || !strings.Contains(string(tb), ref) {
		t.Errorf("template %s", tb)
	}

	// An image older than the limits, still on a panel, is not lifted into Default artwork, nor added by upload.
	if !a.panels.putArtAs(src, PanelArt{ID: "a-0ld0ld00", Type: "image/png"}, imgBytes("png", 1025, 4)) {
		t.Fatal("seed the old image")
	}
	wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":3,"art":"a-0ld0ld00","panel":"`+src+`"}`), 400, "bad_request")
	wantErr(t, apiSendRaw(a, "POST", "/api/v1/panels/"+src+"/artwork", ana, imgBytes("png", 1025, 4)), 400, "bad_request")
	wantErr(t, apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload("Wide", imgBytes("png", 1025, 4), "")), 400, "bad_request")
	big := append(bigSVG(), make([]byte, 200)...)
	wantErr(t, apiSendRaw(a, "POST", "/api/v1/panels/"+src+"/artwork", ana, big), 413, "too_large")
	wantErr(t, apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload("Huge", big, "")), 413, "too_large")
}

// The idle Lock reaches a panel page signed in over its page socket, as an assigned-to-sign-in or keyless page is.
// Known limit (docs/agents/users-locks.md): the Session-mode panel's session key still answers while the Lock is
// up, since apiAuth checks only tokens.live; the page itself drops the key on the Lock (sessionkey.js).
func TestSeamIdleLockCoversAPanelPageSessionKeyStillAnswers(t *testing.T) {
	for _, panelLock := range []bool{false, true} {
		a := sessionKeyApp(t)
		if panelLock {
			cfg := a.r().cfg
			cfg.Locks = config.Locks{Panel: true}
			if err := a.ApplyConfig(cfg); err != nil {
				t.Fatal(err)
			}
		}
		clk := useFakeClock(a)
		setAutoLock(t, a, 5, "")
		c := &client{ip: "10.0.0.1"}
		next := registerAll(a, c)
		a.Hi(c, "panel", "")
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		key := sessionKeyOf(t, a, c, next)
		a.mu.Lock()
		tok := c.u.tokenMu()
		a.mu.Unlock()
		next()

		clk.pass(6 * time.Minute)
		a.idleSweep()
		if st := lastOfType(next(), "lockState"); st["held"] != true {
			t.Fatalf("panelLock=%v: lockState %v", panelLock, st)
		}
		if !a.tokens.locked(tok) {
			t.Fatalf("panelLock=%v: the session token is not locked", panelLock)
		}
		r := send(t, a, c, next, inMsg{T: "sessionKey", ID: 2}, "sessionKeyResult")
		if r["ok"] != false {
			t.Fatalf("panelLock=%v: a new session key under the Lock: %v", panelLock, r)
		}
		if got := panelsStatus(a, key); got != 200 {
			t.Fatalf("panelLock=%v: the session key under the Lock answers %d; update the known limit", panelLock, got)
		}

		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		// Under the page lock a resume drops the tab token, which ends its session keys; the page asks again.
		want := 200
		if panelLock {
			a.endSessionKeys()
			want = 401
		}
		if got := panelsStatus(a, key); got != want {
			t.Fatalf("panelLock=%v: after the resume %d, want %d", panelLock, got, want)
		}
		if k := sessionKeyOf(t, a, c, next); panelsStatus(a, k) != 200 {
			t.Fatalf("panelLock=%v: a fresh key after the resume is refused", panelLock)
		}
	}
}

func lastOfType(msgs []map[string]any, typ string) map[string]any {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i]["t"] == typ {
			return msgs[i]
		}
	}
	return nil
}

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func seamServer(t *testing.T, a *App) *httptest.Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	return srv
}

func waitNamesAndTally(t *testing.T, a *App, key string) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/names", key, nil).Body.String(), "DSM1") &&
			strings.Contains(apiDo(a, "GET", "/api/v1/tally", key, nil).Body.String(), `"routes":[{`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the app never read names and tally from swp08sim")
		}
	}
}

// A keyless page (its key revoked) of a panel with a Fixed destination is assigned to another panel with fixed
// destinations: it opens the target by its short name on the key stored as sode-panel-key-<slug>, a Source press
// and Take patch both fixed destinations, and once that key is revoked it shows the ended-key note and announces
// itself keyless again.
func TestChromeSeamKeylessAssignRunsFixedDestsThenKeyEnds(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	srv := seamServer(t, a)

	ana := userKey(t, a, "Ana")
	const other = 5
	from := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Studio A","acts":"api","slug":"studio-a","settings":{"mode":"take","fixedDests":[%d]}}`, other))
	to := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Studio B","acts":"api","slug":"studio-b","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"source","n":%d},{"r":1,"c":2,"kind":"take"}],"settings":{"mode":"take","fixedDests":[%d,%d]}}`, simSrc, simDst, other))
	secret := instanceKey(t, a, from, ana, "Lobby")
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+from+"/key?instance=Lobby", ana, ""); rec.Code/100 != 2 {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	beforeFrom, beforeTo := unwritten(t, a, from, ana), unwritten(t, a, to, ana)
	waitNamesAndTally(t, a, ana)
	until := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); !ok(); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}

	b.open(srv.URL + "/panels/studio-a?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix) + "&kiosk=1")
	b.waitFor("the ended-key note", `document.getElementById('panels-note')?.textContent.includes("This link's key no longer works")`)
	fromPanel, _ := a.panels.get(from)
	var kid string
	until("the page announces itself keyless", func() bool {
		rows := a.mon.keylessOf(fromPanel)
		if len(rows) == 1 && rows[0].Device == "Lobby" && rows[0].SignIn == "ended" {
			kid = rows[0].Keyless
		}
		return kid != ""
	})
	if code, m := keylessAssign(a, to, ana, kid, ""); code != 200 {
		t.Fatalf("assign: %d %v", code, m)
	}

	b.waitFor("the page runs the target by its short name", `location.pathname === '/panels/studio-b' && document.getElementById('panel-title')?.textContent === 'Studio B' && !!document.querySelector('.sode-panel__cell--source')`)
	if got := b.eval(`location.search + '|' + location.hash`).(string); !strings.Contains(got, "instance=Lobby") || !strings.Contains(got, "kiosk=1") || strings.Contains(got, "key=") {
		t.Fatalf("address %s", got)
	}
	if b.eval(`!!localStorage.getItem('sode-panel-key-studio-b')`) != true {
		t.Fatal("the key is not stored under the short name")
	}
	until("the target holds the instance", func() bool { _, ok := a.mon.streamDevice(to, "Lobby"); return ok })
	if got := unwritten(t, a, from, ana); got != beforeFrom {
		t.Errorf("the source panel changed: %s -> %s", beforeFrom, got)
	}
	if got := unwritten(t, a, to, ana); got != beforeTo {
		t.Errorf("the target changed: %s -> %s", beforeTo, got)
	}

	b.waitFor("no Destination cell is drawn", `!document.querySelector('.sode-panel__cell--destination')`)
	b.eval(`document.querySelector('.sode-panel__cell--source').click()`)
	b.waitFor("Take armed", `!document.querySelector('.sode-panel__take').disabled`)
	b.eval(`document.querySelector('.sode-panel__take').click()`)
	for _, d := range []int{simDst, other} {
		waitRouted(t, a, ana, d, simSrc)
	}

	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+to+"/key?instance=Lobby", ana, ""); rec.Code/100 != 2 {
		t.Fatalf("revoke the assigned key: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("the ended-key note on the target", `document.getElementById('panels-note')?.textContent.includes("This link's key no longer works")`)
	toPanel, _ := a.panels.get(to)
	until("the target's page announces itself keyless", func() bool {
		rows := a.mon.keylessOf(toPanel)
		return len(rows) == 1 && rows[0].Device == "Lobby" && rows[0].SignIn == "ended" && rows[0].Stream
	})
	for _, e := range b.problems() {
		if !strings.Contains(e, "status of 401") {
			t.Errorf("page error: %s", e)
		}
	}
}

// A panel holding an Icon library icon with Icon B, and a cell drawn whole by a Default artwork image with Icon B
// (a cell with art of its own takes none of the default), is Pushed and Copied: on both the copy's and the target's panel page each cell draws its image with Icon B as the corner badge.
func TestChromeSeamIconBBadgeAfterPushAndCopy(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	srv := seamServer(t, a)

	ana := userKey(t, a, "Ana")
	ref := "lib:" + libIcon(t, a, ana, "Logo")
	src := makePanel(t, a, ana, `{"name":"Art"}`)
	img := apiBody(t, apiSendRaw(a, "POST", "/api/v1/panels/"+src+"/artwork", ana, imgBytes("png", 64, 64)))["id"].(string)
	if rec := apiSend(a, "PUT", "/api/v1/default-artwork", ana, `{"kind":"source","n":2,"art":"`+img+`","artB":"A","panel":"`+src+`"}`); rec.Code != 200 {
		t.Fatalf("default: %d %s", rec.Code, rec.Body.String())
	}
	cells := `[{"r":1,"c":1,"kind":"source","n":1,"art":"` + ref + `","artB":"2"},{"r":1,"c":2,"kind":"source","n":2}]`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+src, ana, `{"name":"Art","acts":"api","rows":4,"cols":8,"cells":`+cells+`}`); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	dst := makePanel(t, a, ana, `{"name":"Pushed","acts":"api"}`)
	if code, body := pushTo(a, ana, src, dst); code != 200 {
		t.Fatalf("push: %d %v", code, body)
	}
	// A copy starts in Session mode; switched to API mode so the page opens on a key with no sign-in.
	copied := apiBody(t, apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", ana, ""))
	cp := copied["id"].(string)
	copied["acts"] = "api"
	body, _ := json.Marshal(copied)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+cp, ana, string(body)); rec.Code != 200 {
		t.Fatalf("copy to API mode: %d %s", rec.Code, rec.Body.String())
	}
	waitNamesAndTally(t, a, ana)

	for _, id := range []string{cp, dst} {
		key := instanceKey(t, a, id, ana, "Wall")
		b.open(srv.URL + "/panels/" + id + "?instance=Wall#key=" + strings.TrimPrefix(key, keyPrefix) + "&kiosk=1")
		for i, what := range []string{"the library icon", "the Default artwork"} {
			cell := fmt.Sprintf(`document.querySelectorAll('.sode-panel__cell--source')[%d]`, i)
			b.waitFor(fmt.Sprintf("%s on %s draws with Icon B as a badge", what, id),
				`!!`+cell+` && !!`+cell+`.querySelector('.sode-panel__art--fill .sode-panel__art-a:not([hidden])') && !!`+cell+`.querySelector('.sode-panel__art-b--badge')`)
		}
	}
	b.check()
}

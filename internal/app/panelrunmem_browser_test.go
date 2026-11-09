package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// The run page must hold steady while it sits open for hours: the poll and the one-second redraw may not add nodes,
// documents, listeners, timers or object URLs. A <use> of the external sprite made Chrome load its own copy of the
// sprite document for each tick icon the redraw built, about 16,700 nodes each until a major GC: renderer memory
// climbed from 0.4 to 4 GB in two minutes with one destination viewed.
func TestChromeRunPageHoldsNoMemory(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	// The page holds the instance stream open for as long as it lives, and the page outlives srv.
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","rows":2,"cols":4,"cells":[]}`)
	rec := apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, b64(t, tinyPNG))
	if rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	art := apiBody(t, rec)["id"].(string)
	// The simulator routes destination d from source d, so viewing simDst ticks the simDst source cell.
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, fmt.Sprintf(`{"name":"Wall","rows":2,"cols":4,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d,"art":%q},
		{"r":1,"c":2,"kind":"source","n":%d,"art":"icon:camera"},
		{"r":1,"c":3,"kind":"source","n":%d},
		{"r":1,"c":4,"kind":"take"}],"settings":{"mode":"take","kiosk":true}}`, simDst, art, simDst, simSrc)); rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1") &&
			strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"routes":[{`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the app never read names and tally from swp08sim")
		}
	}

	b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": leakProbe})
	b.call("Performance.enable", nil)
	b.open(srv.URL + "/panels/" + id + "#kiosk=1")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("cells", `document.querySelectorAll('.sode-panel__cell').length === 4`)
	b.eval(`(() => { const f = document.getElementById('panel-device'); if (!f.hidden) { f.querySelector('input').value = 'Wall'; f.querySelector('button').click(); } })()`)
	b.waitFor("destination named", `document.querySelector('.sode-panel__cell--destination .sode-panel__text')?.textContent === 'DSM1'`)
	b.waitFor("art shown", `!document.querySelector('img.sode-panel__art-a')?.hidden`)
	b.eval(`document.querySelector('.sode-panel__cell--destination').click()`)
	b.waitFor("the viewed destination's source is ticked", `!!document.querySelector('.sode-panel__cell--source[data-s="current"] > svg use')`)
	b.waitFor("the tick and the icon draw", `[...document.querySelectorAll('.sode-panel__cell use')].every((u) => u.getBBox().width > 0)`)
	if ext := b.eval(`[...document.querySelectorAll('use')].map((u) => u.getAttribute('href')).filter((h) => !h.startsWith('#')).join(' ')`); ext != "" {
		t.Errorf("a <use> points outside the page: %v", ext)
	}

	// Pre-GC documents over several redraws: each external <use> made one more.
	docs := func() float64 {
		var m struct {
			Metrics []struct {
				Name  string
				Value float64
			}
		}
		_ = json.Unmarshal(b.call("Performance.getMetrics", nil), &m)
		for _, x := range m.Metrics {
			if x.Name == "Documents" {
				return x.Value
			}
		}
		return 0
	}
	first := docs()
	most := first
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(250 * time.Millisecond) {
		most = max(most, docs())
	}
	if most > first+2 {
		t.Errorf("documents went from %.0f to %.0f over five idle seconds", first, most)
	}

	if n := b.eval(animList); n != "" {
		t.Errorf("the idle run page runs animations: %v", n)
	}
	const rounds = 4
	var samples []leakSample
	for i := 0; i < rounds; i++ {
		time.Sleep(4 * time.Second) // two polls and four redraws
		s := b.measure()
		samples = append(samples, s)
		t.Logf("round %d: %s", i+1, s)
	}
	base, last := samples[0], samples[rounds-1]
	var bad []string
	// The page and the logo's mask image.
	if last.documents > 2 || last.documents != base.documents {
		bad = append(bad, fmt.Sprintf("documents %.0f -> %.0f after GC", base.documents, last.documents))
	}
	if last.intervals != base.intervals || last.docL != base.docL || last.urls != base.urls {
		bad = append(bad, fmt.Sprintf("intervals %.0f -> %.0f, document listeners %.0f -> %.0f, object URLs %.0f -> %.0f",
			base.intervals, last.intervals, base.docL, last.docL, base.urls, last.urls))
	}
	if last.nodes > base.nodes+20 || last.listeners > base.listeners+10 {
		bad = append(bad, fmt.Sprintf("nodes %.0f -> %.0f, listeners %.0f -> %.0f", base.nodes, last.nodes, base.listeners, last.listeners))
	}
	if len(bad) > 0 {
		t.Errorf("the idle run page grew: %s", strings.Join(bad, "; "))
	}
	b.check()
}

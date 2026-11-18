package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A Grid panel page sits open for hours while other callers change routes. Its socket frames, polls and redraws must
// not add nodes, listeners, timers or heap. Runs only in the leak job (name ends HoldsNoMemory).
func TestChromeGridPanelHoldsNoMemory(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Studio grid","type":"grid","settings":{"mode":"take","levels":[1,2]}}`)
	waitNamesAndTally(t, a, ana)

	b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": leakProbe})
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	b.waitFor("the pie shows what the router has", gridXp(2, 2)+`.dataset.on?.startsWith('1,2')`)

	// Each round another caller moves one route back and forth, so frames reach the page.
	take := func(src, dst int) {
		if rec := apiSend(a, "POST", "/api/v1/takes", ana, fmt.Sprintf(`{"routes":[{"src":%d,"dst":%d}]}`, src, dst)); rec.Code != 200 {
			t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
		}
	}
	const rounds = 5
	var samples []leakSample
	for i := 0; i < rounds; i++ {
		for j := 0; j < 6; j++ {
			take(11+j%2, 2)
			time.Sleep(500 * time.Millisecond)
		}
		b.waitFor("the last take shows", gridXp(12, 2)+`.dataset.on?.startsWith('1')`)
		s := b.measure()
		samples = append(samples, s)
		t.Logf("round %d: %s", i+1, s)
	}
	// The first round still warms caches; compare from the second.
	base, last := samples[1], samples[rounds-1]
	var bad []string
	if last.documents != base.documents || last.intervals != base.intervals || last.docL != base.docL || last.urls != base.urls {
		bad = append(bad, fmt.Sprintf("documents %.0f -> %.0f, intervals %.0f -> %.0f, document listeners %.0f -> %.0f, object URLs %.0f -> %.0f",
			base.documents, last.documents, base.intervals, last.intervals, base.docL, last.docL, base.urls, last.urls))
	}
	if last.nodes > base.nodes+50 || last.listeners > base.listeners+10 {
		bad = append(bad, fmt.Sprintf("nodes %.0f -> %.0f, listeners %.0f -> %.0f", base.nodes, last.nodes, base.listeners, last.listeners))
	}
	// A kept copy of the tally per frame would add about the tally's size each round.
	if last.heap > base.heap*1.25+2e6 {
		bad = append(bad, fmt.Sprintf("heap %.1f MB -> %.1f MB", base.heap/1e6, last.heap/1e6))
	}
	if len(bad) > 0 {
		t.Errorf("the Grid panel page grew: %s", strings.Join(bad, "; "))
	}
	b.check()
}

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// The browser harness with several routers (docs/agents/browser-checks.md).

// simNames writes a SIM_NAMES_FILE for a simulator ("src 10 Stage Cam", "dst 3 Stage Mon") and gives its env entry.
func simNames(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "names.txt")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return "SIM_NAMES_FILE=" + p
}

// simRoutersApp is simApp with one router on each simulator address: the first is router A, the next B, and so on.
// Each router after the first has the first router's settings, its own address, prefix and title ("Router B"), and a
// new UID. edit runs last.
func simRoutersApp(t *testing.T, addrs []string, edit ...func(*config.Config)) *App {
	t.Helper()
	more := func(c *config.Config) {
		for i, addr := range addrs[1:] {
			rc := c.Router
			p := string(rune('B' + i))
			rc.UID, rc.Prefix, rc.Title, rc.Address = "", p, "Router "+p, addr
			c.MoreRouters = append(c.MoreRouters, rc)
		}
	}
	return simApp(t, addrs[0], append([]func(*config.Config){more}, edit...)...)
}

// routerUIDs gives the UID of each router of a, the first router first.
func routerUIDs(a *App) []string {
	var out []string
	for _, rc := range a.r().cfg.Routers() {
		out = append(out, rc.UID)
	}
	return out
}

// waitRouterRead waits until router uid's names hold name and its tally has routes, both read over REST.
func waitRouterRead(t *testing.T, a *App, key, uid, name string) {
	t.Helper()
	q := "?router=" + uid
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		names, tally := apiDo(a, "GET", "/api/v1/names"+q, key, nil).Body.String(), apiDo(a, "GET", "/api/v1/tally"+q, key, nil).Body.String()
		if strings.Contains(names, name) && strings.Contains(tally, `"routes":[{`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("router %s never had names and tally: %.300s | %.300s", uid, names, tally)
		}
	}
}

// routerTallyHas waits for router uid, read over REST, to have src on dst at level 1.
func routerTallyHas(t *testing.T, a *App, key, uid string, dst, src int) {
	t.Helper()
	want := fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, dst, src)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		body := apiDo(a, "GET", "/api/v1/tally?router="+uid, key, nil).Body.String()
		if strings.Contains(body, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("router %s tally never had %s: %.400s", uid, want, body)
		}
	}
}

// The main page patches on two routers: a typed B.12 searches router B's names, and the take goes to router B only.
func TestChromeMainPatchesTwoRouters(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 Stage Cam", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	// Text then Tab picks the best match of the box, which must be want (number and name). The input repeats until the
	// page has the router's names.
	pick := func(id, text, want string) {
		b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
			return (document.getElementById('`+id+`-opt-0')?.textContent || '').replace(/\s/g, '') === '`+strings.ReplaceAll(want, " ", "")+`'; })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	take := func() {
		b.eval(`(() => { const t = document.getElementById('lvl'); t.focus(); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); t.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); return true; })()`)
		b.waitFor("confirm unlocks", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
		b.eval(`document.getElementById('confirm-ok').click(); true`)
		b.waitFor("confirm closes", `document.getElementById('confirm').hidden`)
	}

	// Router A, as with one router: no prefix.
	pick("src", simSrcName, "11 "+simSrcName)
	pick("dst", "DSM1", "3 DSM1")
	take()
	routerTallyHas(t, a, ana, uids[0], simDst, simSrc)

	// Router B: its names, its prefix in the label, and its own route view.
	pick("src", "B.12", "B.12 Stage Cam")
	pick("dst", "B.4", "B.4 Stage Mon")
	b.waitFor("the route view shows router B's tally", `document.getElementById('dst-under').textContent.includes('Currently on B.4 Stage Mon')`)
	take()
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	b.waitFor("router B's tally message reaches the route view", `(() => {
		const t = document.getElementById('dst'); t.focus(); t.value = 'B.4'; t.dispatchEvent(new Event('input', {bubbles: true}));
		t.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true}));
		return document.getElementById('dst-under').textContent.includes('B.12 Stage Cam');
	})()`)
	// Each take went to its own router only: router A's destination 3 and router B's destination 2 keep their sources.
	routerTallyHas(t, a, ana, uids[0], 3, 3)
	routerTallyHas(t, a, ana, uids[1], simDst, simDst)
	b.check()
}

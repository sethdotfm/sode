package app

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// simConnect routes src to dst on the given wire levels from a second SW-P-08 client: an External.
func simConnect(t *testing.T, sim string, dst, src int, levels ...int) {
	t.Helper()
	c, err := net.DialTimeout("tcp", sim, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for _, l := range levels {
		if _, err := c.Write(swp08.Encode(swp08.ExtConnect(0, l, dst, src))); err != nil {
			t.Fatal(err)
		}
	}
	// The sim answers each level; keep the link open until it has.
	time.Sleep(300 * time.Millisecond)
}

// Via on the main page with two simulators (docs/tielines-design.md, Via). TIE 1 goes from A.6 to B.12 From A, and
// B.12 Far Mon routes B.12 From A from the start. An External on the upstream simulator changes what TIE 1 carries.
// The Destination box of B.12 then shows the new origin Via TIE 1. History notes what the External also changes. The
// Source box of A.11 lists B.12 with its Via mark. An External on one level makes the chain mixed, with the level pie.
func TestChromeTieViaAfterExternal(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "dst 11 Far Mon"))
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Far Mon")
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{{ID: "aaaaaaaaaaaaaaa1", Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}}}
	})
	var levels []int
	for _, l := range a.r().levelsOf(uids[0]).levels {
		levels = append(levels, l.Wire)
	}

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	pick := func(id, text, want string) {
		b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
			return (document.getElementById('`+id+`-opt-0')?.textContent || '').replace(/\s/g, '') === '`+want+`'; })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	under := `document.getElementById('dst-under')`
	viaRow := `[...` + under + `.querySelectorAll('.sode__route')].find((r) => r.querySelector('.sode-via'))`

	pick("dst", "B12", "B.12FarMon")
	b.waitFor("the Destination box shows the origin Via TIE 1", `(() => { const r = `+viaRow+`;
		return !!r && r.querySelector('.sode__end--src').textContent.startsWith('A.6') && r.querySelector('.sode-via').textContent === 'Via TIE 1'
			&& r.querySelector('.sode-via').dataset.pbTip.endsWith('Via TIE 1'); })()`)

	// An External on the upstream simulator: A.6 now routes A.11 Zoom 1 on every level.
	simConnect(t, simA, 5, simSrc, levels...)
	b.waitFor("the Destination box follows the External upstream", `(() => { const r = `+viaRow+`;
		return !!r && r.querySelector('.sode__end--src').textContent === 'A.11 `+simSrcName+`' && !r.textContent.includes('mixed'); })()`)
	b.waitFor("History notes what the External also changes", `[...document.querySelectorAll('#history .sode__hist-note')].some((n) => n.textContent.includes('also changes B.12 Far Mon Via TIE 1'))`)

	// The Source box of A.11 lists B.12 through TIE 1.
	pick("src", "A11", "A.11"+strings.ReplaceAll(simSrcName, " ", ""))
	b.waitFor("the Source box lists B.12 Via TIE 1", `[...document.getElementById('src-under').querySelectorAll('.sode__route')].some((r) =>
		r.querySelector('.sode__end--dst')?.textContent === 'B.12 Far Mon' && r.querySelector('.sode-via')?.textContent === 'Via TIE 1')`)

	// An External on one level only: the chain is mixed, with the level pie under it.
	b.eval(`document.getElementById('src').value = ''; document.getElementById('src').dispatchEvent(new Event('input', {bubbles: true})); true`)
	simConnect(t, simA, 5, 3, levels[1])
	b.waitFor("the mixed mark and the level pie", `(() => { const r = `+viaRow+`;
		return !!r && r.querySelector('.sode-via').textContent === 'Via TIE 1 (mixed)' && `+under+`.querySelectorAll('.sode-via__pie').length === 2
			&& [...`+under+`.querySelectorAll('.sode-via__pie')].some((p) => p.textContent.includes('A.4')); })()`)
	b.check()
}

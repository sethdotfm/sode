package app

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// History of tie takes with two simulators (docs/tielines-design.md, History). A tie take of one destination is one
// row that names the typed source and the tie line. It opens to show what TIE 1 carried before. A second tie take to
// that destination through TIE 1 stacks with it.
func TestChromeHistoryTieRows(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")
	const tie = "aaaaaaaaaaaaaaa1"
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{{ID: tie, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}}}
	})
	// TIE 1 carries A.1 on every level, so that each tie take below changes it.
	body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":0},"dst":{"router":%q,"n":5},"levels":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17]}]}`, uids[0], uids[0])
	if rec := apiPost(a, "/api/v1/takes", ana, body, nil); rec.Code != http.StatusOK {
		t.Fatalf("move TIE 1: %d %s", rec.Code, rec.Body)
	}
	routerTallyHas(t, a, ana, uids[0], 5, 0)
	tieTake := func(src, was int) {
		t.Helper()
		body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":%d},"dst":{"router":%q,"n":2},"levels":[1],"ties":[{"id":%q,"expect":{"router":%q,"n":%d}}]}]}`,
			uids[0], src, uids[1], tie, uids[0], was)
		if rec := apiPost(a, "/api/v1/takes", ana, body, nil); rec.Code != http.StatusOK {
			t.Fatalf("tie take from A.%d: %d %s", src+1, rec.Code, rec.Body)
		}
	}
	tieTake(1, 0)

	b.open(srv.URL + "/")
	row := `document.querySelector('#history > li')`
	b.waitFor("one row names the typed source, TIE 1 and B.3", `(() => { const r = `+row+`; if (!r) return false;
		const t = r.querySelector('.sode__hist-route').textContent;
		return t.includes('A.2') && t.includes('Via TIE 1') && t.includes('B.3') && !t.includes('From A')
			&& document.querySelectorAll('#history > li').length === 2; })()`)
	// The second row is the plain take that moved TIE 1, not a hop of the tie take.
	b.waitFor("no hop row", `!document.querySelector('#history > li:nth-child(2) .sode__hist-route').textContent.includes('Via')`)
	b.eval(row + `.querySelector('.sode__hist-toggle').click(); true`)
	b.waitFor("the open row shows what TIE 1 carried before", `(() => { const d = `+row+`.querySelector('.sode__hist-before'); if (!d) return false;
		const labels = [...d.querySelectorAll('.sode__label')].map((l) => l.textContent);
		return labels.some((l) => /^Before #\d+, on TIE 1$/.test(l)) && d.textContent.includes('A.1'); })()`)

	// A second tie take through TIE 1 stacks with the first.
	tieTake(2, 1)
	b.waitFor("the second tie take stacks", `(() => { const r = `+row+`; return !!r && document.querySelectorAll('#history > li').length === 2
		&& r.querySelector('.sode__hist-route').textContent.includes('A.3') && r.querySelector('.sode__hist-toggle').textContent.trim() === '2'; })()`)
	b.check()
}

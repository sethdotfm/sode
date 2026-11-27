package app

import (
	"fmt"
	"net/http"
	"testing"
)

// The plan of a route counts the other routes of the same take (docs/tielines-design.md, Takes with plain routes and
// tie routes). A route with ties puts its source on its tie lines. A later route with the same source shares the tie
// line and sends nothing upstream. A route with another source gets no path through it ("taken"). A plain route to
// the upstream destination of a tie line takes that tie line too. The 409 body lists the routes with no ties only.
func TestAPITakeAcrossRoutersPlanCountsTheTake(t *testing.T) {
	a, _, _, key := tieApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	tied := func(src int) string {
		return fmt.Sprintf(`{"src":{"router":%q,"n":%d},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"id":"t1","expect":{"router":%q,"n":3}}]}`, uidA, src, uidA)
	}
	bare := func(src int) string {
		return fmt.Sprintf(`{"src":{"router":%q,"n":%d},"dst":{"router":"b2b2b2b2","n":7},"levels":[1]}`, uidA, src)
	}
	plain := fmt.Sprintf(`{"src":{"router":%q,"n":6},"dst":{"router":%q,"n":10},"levels":[1]}`, uidA, uidA)

	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[`+tied(4)+`,`+bare(4)+`]}`, nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_needed")
	b := tieBody(t, rec.Body)
	if len(b.Routes) != 1 || b.Routes[0].Route != 1 || len(b.Routes[0].Paths) != 1 {
		t.Fatalf("same source: routes %+v", b.Routes)
	}
	if p := b.Routes[0].Paths[0]; p.Hops[0].Sends || p.Cost != 0 {
		t.Errorf("same source: the second route shares TIE 3 and sends nothing upstream: %+v", p)
	}

	for name, body := range map[string]string{
		"another source": `{"routes":[` + tied(4) + `,` + bare(8) + `]}`,
		"plain route":    `{"routes":[` + plain + `,` + bare(4) + `]}`,
	} {
		rec := apiPost(a, "/api/v1/takes", key, body, nil)
		wantErr(t, rec, http.StatusConflict, "no_path")
		b := tieBody(t, rec.Body)
		if len(b.Routes) != 1 || b.Routes[0].Route != 1 || len(b.Routes[0].Paths) != 0 {
			t.Fatalf("%s: routes %+v", name, b.Routes)
		}
		if rs := b.Routes[0].Reasons; len(rs) != 1 || rs[0].Why != "taken" || rs[0].Tie != "t1" || rs[0].TieName != "TIE 3" {
			t.Errorf("%s: reasons %+v", name, rs)
		}
	}
}

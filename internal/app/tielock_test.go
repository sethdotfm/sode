package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// gateConnects holds each connect that the unit of f sends until open is called, as a busy router does. arrived
// closes at the first connect.
func gateConnects(f *fakeRouter) (arrived <-chan struct{}, open func()) {
	gate, first := make(chan struct{}), make(chan struct{})
	var once, opened sync.Once
	send := f.u.sendMsg
	f.u.sendMsg = func(m []byte, why swp08.Why) error {
		if m[0] != swp08.CmdExtConnect {
			return send(m, why)
		}
		once.Do(func() { close(first) })
		go func() {
			<-gate
			send(m, why)
		}()
		return nil
	}
	return first, func() { opened.Do(func() { close(gate) }) }
}

type tieChangedIn struct {
	Error  apiErr               `json:"error"`
	Routes []apiTieChangedRoute `json:"routes"`
}

func tieChangedBody(t *testing.T, rec *httptest.ResponseRecorder) tieChangedIn {
	t.Helper()
	wantErr(t, rec, http.StatusConflict, "tie_line_changed")
	var b tieChangedIn
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

// post runs a POST /takes in the background.
func post(a *App, key, body string) <-chan *httptest.ResponseRecorder {
	out := make(chan *httptest.ResponseRecorder, 1)
	go func() { out <- apiPost(a, "/api/v1/takes", key, body, nil) }()
	return out
}

func wait[T any](t *testing.T, what string, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
	panic("unreachable")
}

// seedAll routes src to dst on every level of the first router.
func seedAll(a *App, f *fakeRouter, dst, src int) {
	for l := 1; l <= a.r().routerOf(a.r().cfg.UID).Levels; l++ {
		f.seed(dst, src, a.r().byUI[l].Wire)
	}
}

// The stale check compares every level of a tie line. A has two levels. A move on level 2 only is a change, and
// a mixed tie line never matches one source. A caller that saw the mixed state sends one source for each level.
func TestAPITieTakeComparesEveryLevel(t *testing.T) {
	a, fa, fb, key := tieApp(t, func(c *config.Config) { c.Levels, c.DefaultLevels = 2, "1-2" })
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	fa.seed(10, 6, a.r().byUI[2].Wire) // level 2 moves to A.6; level 1 stays on A.3
	b := tieChangedBody(t, apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil))
	now := b.Routes[0].Paths[0].Hops[0].Now
	if len(b.Routes[0].Changed) != 1 || !now.Mixed || len(now.Levels) != 2 || now.Levels[0].N != 3 || now.Levels[1].N != 6 {
		t.Fatalf("route %+v, now %+v", b.Routes[0], now)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	mixed := func(l1, l2 string) string {
		return strings.Replace(tieRoutes(uidA, [2]int{4, 2}), `"expect":{"router":"`+uidA+`","n":3}`, `"expect":[`+l1+`,`+l2+`]`, 1)
	}
	ref := func(n int) string { return fmt.Sprintf(`{"router":%q,"n":%d}`, uidA, n) }
	// One entry for each level of A, or 400.
	wantErr(t, apiPost(a, "/api/v1/takes", key, strings.Replace(mixed(ref(3), ref(6)), `,{"router"`, `,null,{"router"`, 1), nil), http.StatusBadRequest, "bad_request")
	wantErr(t, apiPost(a, "/api/v1/takes", key, mixed(ref(3), "null"), nil), http.StatusConflict, "tie_line_changed")
	if rec := apiPost(a, "/api/v1/takes", key, mixed(ref(3), ref(6)), nil); rec.Code != http.StatusOK {
		t.Fatalf("mixed as seen: %d %s", rec.Code, rec.Body)
	}
	if got := log(); len(got) != 3 || got[0] != "A 4>10" || got[1] != "A 4>10" || got[2] != "B 5>2" {
		t.Errorf("connects %v", got)
	}
}

// A tie line that does not carry the source that the caller expects refuses the whole take before anything is
// sent. The answer gives the tie line as it is now.
func TestAPITieTakeRefusesMovedTieLine(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	for _, expect := range []string{`{"router":"` + uidA + `","n":6}`, `null`} {
		body := strings.Replace(tieRoutes(uidA, [2]int{4, 2}), `{"router":"`+uidA+`","n":3}}`, expect+"}", 1)
		b := tieChangedBody(t, apiPost(a, "/api/v1/takes", key, body, nil))
		if len(b.Routes) != 1 || len(b.Routes[0].Changed) != 1 || b.Routes[0].Changed[0] != (apiTieRef{ID: "t1", Name: "TIE 3"}) {
			t.Fatalf("expect %s: routes %+v", expect, b.Routes)
		}
		r := b.Routes[0]
		if len(r.Paths) != 1 || r.Paths[0].Hops[0].Now == nil || *r.Paths[0].Hops[0].Now.Origin != (apiTieEndOut{Router: uidA, N: 3}) {
			t.Errorf("expect %s: path %+v", expect, r.Paths)
		}
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	if es, _ := a.hist.Page(-1, 5); len(es) != 0 {
		t.Errorf("entries %+v", es)
	}

	// Nothing routed on the tie line, and the caller expects nothing: the take goes.
	seedAll(a, fa, 10, 0xFFFF)
	body := strings.Replace(tieRoutes(uidA, [2]int{4, 2}), `{"router":"`+uidA+`","n":3}}`, "null}", 1)
	if rec := apiPost(a, "/api/v1/takes", key, body, nil); rec.Code != http.StatusOK {
		t.Errorf("expect null on an empty tie line: %d %s", rec.Code, rec.Body)
	}
}

// With affected, the path must have no Affected destination that the caller did not see. B.20 and B.21 show TIE 3.
func TestAPITieTakeComparesAffected(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	withAffected := func(ns ...int) string {
		var es []string
		for _, n := range ns {
			es = append(es, fmt.Sprintf(`{"router":"b2b2b2b2","n":%d}`, n))
		}
		return strings.Replace(tieRoutes(uidA, [2]int{4, 2}), `"levels":[1],`, `"levels":[1],"affected":[`+strings.Join(es, ",")+`],`, 1)
	}
	b := tieChangedBody(t, apiPost(a, "/api/v1/takes", key, withAffected(20), nil))
	r := b.Routes[0]
	if len(r.Changed) != 0 || len(r.NewAffected) != 1 || r.NewAffected[0] != (apiTieEndOut{Router: "b2b2b2b2", N: 21, Name: "B MON 21"}) {
		t.Fatalf("route %+v", r)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	if rec := apiPost(a, "/api/v1/takes", key, withAffected(21, 20, 30), nil); rec.Code != http.StatusOK {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
	// A route on one router has no tie lines, so no Affected destinations to compare.
	one := `{"routes":[{"src":{"router":"b2b2b2b2","n":5},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"affected":[]}]}`
	wantErr(t, apiPost(a, "/api/v1/takes", key, one, nil), http.StatusBadRequest, "bad_request")
}

// Two takes on one tie line, with a held router: the second waits for the lock of the first. When it gets the
// lock, the tie line carries the source of the first, so the second is refused and sends nothing.
func TestAPITieTakeWaitsForTieLock(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	fa.u.routeTimeout = 5 * time.Second
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	arrived, open := gateConnects(fa)
	t.Cleanup(open)
	first := post(a, key, tieRoutes(uidA, [2]int{4, 2}))
	wait(t, "the first take's hop", arrived)
	second := post(a, key, tieRoutes(uidA, [2]int{6, 21}))
	waitFor(t, "the second take to wait for TIE 3", func() bool { return a.tieLocks.waiting.Load() == 1 })
	open()
	if rec := wait(t, "the first take", first); rec.Code != http.StatusOK {
		t.Fatalf("first: %d %s", rec.Code, rec.Body)
	}
	b := tieChangedBody(t, wait(t, "the second take", second))
	if r := b.Routes[0]; len(r.Changed) != 1 || r.Paths[0].Hops[0].Now.Origin.N != 4 {
		t.Errorf("second: %+v", r)
	}
	if got := log(); slices.Contains(got, "A 6>10") || slices.Contains(got, "B 5>21") {
		t.Errorf("connects %v", got)
	}
}

// A plain take to the upstream destination of a tie line takes the same lock: it waits for the tie take.
func TestPlainTakeWaitsForTieLock(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	fa.u.routeTimeout = 5 * time.Second
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	arrived, open := gateConnects(fa)
	t.Cleanup(open)
	tie := post(a, key, tieRoutes(uidA, [2]int{4, 2}))
	wait(t, "the tie take's hop", arrived)
	plain := runTake(a, routeReq{ID: 1, Src: 6, Dst: 10, Levels: []int{1}})
	waitFor(t, "the plain take to wait for TIE 3", func() bool { return a.tieLocks.waiting.Load() == 1 })
	open()
	if rec := wait(t, "the tie take", tie); rec.Code != http.StatusOK {
		t.Fatalf("tie take: %d %s", rec.Code, rec.Body)
	}
	wait(t, "the plain take", plain)
	got := log()
	if i, j := slices.Index(got, "B 5>2"), slices.Index(got, "A 6>10"); i < 0 || j < i {
		t.Errorf("connects %v: want A 6>10 after B 5>2", got)
	}
	// A take to a destination that feeds no tie line takes no lock.
	if ids := a.upstreamTies(a.r(), []routeReq{{Dst: 11}}); len(ids) != 0 {
		t.Errorf("locks %v", ids)
	}
}

// Two takes that name the same two tie lines in opposite orders never wait on each other: each locks in id order.
func TestAPITieTakesDoNotDeadlock(t *testing.T) {
	a, fa, _, key := tieApp(t, func(c *config.Config) {
		c.TieLines = append(c.TieLines, config.TieLine{ID: "t2", Name: "TIE 9", From: config.Ref{Router: c.UID, N: 11}, To: config.Ref{Router: "b2b2b2b2", N: 6}})
	})
	fa.u.routeTimeout = 5 * time.Second
	uidA := a.r().cfg.UID
	seedAll(a, fa, 11, 3)
	route := func(tie string, dst int) string {
		return fmt.Sprintf(`{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":%d},"levels":[1],"ties":[{"id":%q,"expect":{"router":%q,"n":3}}]}`, uidA, dst, tie, uidA)
	}
	x := `{"routes":[` + route("t1", 2) + "," + route("t2", 3) + `]}`
	y := `{"routes":[` + route("t2", 3) + "," + route("t1", 2) + `]}`
	arrived, open := gateConnects(fa)
	t.Cleanup(open)
	first := post(a, key, x)
	wait(t, "the first take's hops", arrived)
	second := post(a, key, y)
	waitFor(t, "the second take to wait", func() bool { return a.tieLocks.waiting.Load() == 1 })
	open()
	if rec := wait(t, "the first take", first); rec.Code != http.StatusOK {
		t.Fatalf("first: %d %s", rec.Code, rec.Body)
	}
	b := tieChangedBody(t, wait(t, "the second take", second))
	if len(b.Routes) != 2 || len(b.Routes[0].Changed) != 1 || b.Routes[0].Changed[0].ID != "t2" || b.Routes[1].Changed[0].ID != "t1" {
		t.Errorf("second: %+v", b.Routes)
	}
}

// Many takes at one time on the tie lines and their upstream destinations all end. Run it with -race.
func TestTieLocksUnderLoad(t *testing.T) {
	a, fa, _, key := tieApp(t, func(c *config.Config) {
		c.TieLines = append(c.TieLines, config.TieLine{ID: "t2", Name: "TIE 9", From: config.Ref{Router: c.UID, N: 11}, To: config.Ref{Router: "b2b2b2b2", N: 6}})
	})
	fa.hold = 2 * time.Millisecond
	fa.u.routeTimeout = 2 * time.Second
	uidA := a.r().cfg.UID
	var wg sync.WaitGroup
	for i := range 6 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":%d},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"id":"t%d","expect":null}]}]}`, uidA, i, i%2+1)
			if rec := apiPost(a, "/api/v1/takes", key, body, nil); rec.Code != http.StatusConflict && rec.Code != http.StatusOK && rec.Code != http.StatusTooManyRequests {
				t.Errorf("%d %s", rec.Code, rec.Body)
			}
		}()
		go func() {
			defer wg.Done()
			a.takeAllowed(&client{ip: "10.0.0.1"}, []routeReq{{ID: 1, Src: i, Dst: 10 + i%2, Levels: []int{1}}, {ID: 2, Src: i, Dst: 11 - i%2, Levels: []int{1}}})
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	wait(t, "every take", done)
	if n := a.tieLocks.waiting.Load(); n != 0 {
		t.Errorf("%d takes still wait", n)
	}
}

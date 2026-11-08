package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func ints(v ...int) *[]int { return &v }

// rightsApp is userApp with Ana's rights set and a fake router.
func rightsApp(t *testing.T, r config.Rights) (*App, *fakeRouter, *client) {
	t.Helper()
	a := userApp(t, false)
	f := newFakeRouter(t, a)
	cfg := a.r().cfg
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Ana" {
			cfg.Users[i].Rights = r
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return a, f, rightsSignedIn(t, a, "Ana")
}

func rightsSignedIn(t *testing.T, a *App, name string) *client {
	t.Helper()
	u, _ := a.r().cfg.FindUser(name)
	c := &client{ip: "10.0.0.1", send: make(chan []byte, 32)}
	register(a, c)
	a.mu.Lock()
	c.u.user.Store(&u)
	a.mu.Unlock()
	return c
}

// takeResults runs a take and returns what the client was sent.
func takeResults(t *testing.T, a *App, c *client, items []routeReq) []routeResult {
	t.Helper()
	a.take(c, items)
	var out []routeResult
	for {
		select {
		case b := <-c.send:
			var r routeResult
			if json.Unmarshal(b, &r) == nil && r.T == "takeResult" {
				out = append(out, r)
			}
		case <-time.After(50 * time.Millisecond):
			return out
		}
	}
}

func allFailed(t *testing.T, rs []routeResult, n int, want string) {
	t.Helper()
	if len(rs) != n {
		t.Fatalf("results %+v, want %d", rs, n)
	}
	for _, r := range rs {
		if r.OK || !strings.Contains(r.Err, want) || r.Entry != nil {
			t.Fatalf("result %+v, want failure with %q and no entry", r, want)
		}
	}
}

func TestRightsRefuseWholeTake(t *testing.T) {
	a, f, c := rightsApp(t, config.Rights{Dsts: ints(5)})
	defer startLog(t, a)()
	f.seed(5, 1, 0, 1, 2)
	n := f.sends()
	rs := takeResults(t, a, c, []routeReq{
		{ID: 1, Src: 7, Dst: 5, Levels: []int{1}},
		{ID: 2, Src: 7, Dst: 6, Levels: []int{1}},
	})
	allFailed(t, rs, 2, "destination 7")
	if f.sends() != n {
		t.Fatal("something was sent for a refused take")
	}
	if a.hist.Len() != 0 {
		t.Fatalf("history has %d entries", a.hist.Len())
	}
	rows := waitLogged(t, a, 1)
	e := rows[len(rows)-1]
	if e.Kind != kindAPI || e.By != "Ana" || e.Why != "take" || e.Result != "refused" || !strings.Contains(e.Err, "destination 7") {
		t.Fatalf("row %+v", e)
	}
	leftovers(t, a)
}

func TestRightsAllowInside(t *testing.T) {
	a, f, c := rightsApp(t, config.Rights{Sources: ints(7), Dsts: ints(5), Levels: ints(1)})
	f.seed(5, 1, 0)
	rs := takeResults(t, a, c, []routeReq{{ID: 1, Src: 7, Dst: 5, Levels: []int{1}}})
	if len(rs) != 1 || !rs[0].OK {
		t.Fatalf("results %+v", rs)
	}
}

func TestRightsEachKind(t *testing.T) {
	for name, tc := range map[string]struct {
		r    config.Rights
		want string
	}{
		"source": {config.Rights{Sources: ints(1)}, "source 8"},
		"level":  {config.Rights{Levels: ints(1)}, "level"},
		"none":   {config.Rights{Dsts: ints()}, "destination 6"},
	} {
		t.Run(name, func(t *testing.T) {
			a, f, c := rightsApp(t, tc.r)
			n := f.sends()
			rs := takeResults(t, a, c, []routeReq{{ID: 1, Src: 7, Dst: 5, Levels: []int{1, 2}}})
			allFailed(t, rs, 1, tc.want)
			if f.sends() != n {
				t.Fatal("sent")
			}
		})
	}
}

func TestRightsPreset(t *testing.T) {
	items := func(preset string, dst int) []routeReq {
		return []routeReq{{ID: 1, Src: 7, Dst: dst, Levels: []int{1}, Preset: preset}}
	}
	a, f, c := rightsApp(t, config.Rights{Dsts: ints(5), Presets: &[]string{"Show"}})
	f.seed(5, 1, 0)
	allFailed(t, takeResults(t, a, c, items("Other", 5)), 1, `preset "Other"`)
	allFailed(t, takeResults(t, a, c, items("Show", 6)), 1, "destination 7")
	if rs := takeResults(t, a, c, items("Show", 5)); len(rs) != 1 || !rs[0].OK {
		t.Fatalf("allowed preset: %+v", rs)
	}
}

func TestRightsUndoSkipsBeforeSource(t *testing.T) {
	a, f, c := rightsApp(t, config.Rights{Sources: ints(7), Dsts: ints(5)})
	f.seed(5, 1, 0)
	undo := 3
	// Undo restores source 1, which is outside the rights.
	if rs := takeResults(t, a, c, []routeReq{{ID: 1, Src: 1, Dst: 5, Levels: []int{1}, Undo: &undo}}); len(rs) != 1 || !rs[0].OK {
		t.Fatalf("undo: %+v", rs)
	}
	allFailed(t, takeResults(t, a, c, []routeReq{{ID: 2, Src: 1, Dst: 5, Levels: []int{1}}}), 1, "source 2")
	retry := 3
	allFailed(t, takeResults(t, a, c, []routeReq{{ID: 3, Src: 1, Dst: 5, Levels: []int{1}, Retry: &retry}}), 1, "source 2")
	// Undo still checks the destination.
	allFailed(t, takeResults(t, a, c, []routeReq{{ID: 4, Src: 7, Dst: 6, Levels: []int{1}, Undo: &undo}}), 1, "destination 7")
}

func TestRightsGuestApplyToSignedOutBrowser(t *testing.T) {
	a, f, _ := rightsApp(t, config.Rights{})
	cfg := a.r().cfg
	cfg.GuestRights = config.Rights{Dsts: ints(5)}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.2", send: make(chan []byte, 8)}
	register(a, c)
	n := f.sends()
	allFailed(t, takeResults(t, a, c, []routeReq{{ID: 1, Src: 7, Dst: 6, Levels: []int{1}}}), 1, "destination 7")
	if f.sends() != n {
		t.Fatal("sent")
	}
}

func TestRightsNativeFireRefusedBeforeStaging(t *testing.T) {
	a, f := nativeApp(t, 3)
	cfg := a.r().cfg
	cfg.GuestRights = config.Rights{Dsts: ints(1, 2)}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.2", send: make(chan []byte, 8)}
	register(a, c)
	rs := takeResults(t, a, c, []routeReq{
		{ID: 1, Src: 7, Dst: 1, Levels: []int{1}},
		{ID: 2, Src: 7, Dst: 9, Levels: []int{1}},
	})
	allFailed(t, rs, 2, "destination 10")
	if f.sends() != 0 {
		t.Fatalf("%d messages sent, none expected (no salvo clear, stage or Go)", f.sends())
	}
}

// An API caller brings its own actor and rights; the take labels History with the actor.
func TestRightsAPICaller(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0)
	act := Actor{Kind: actorClient, Name: "PanelsBot", Key: "Panel 43"}
	c := &client{ip: "10.0.0.3", send: make(chan []byte, 8), api: &apiCaller{Actor: act, Rights: config.Rights{Dsts: ints(5)}}}
	register(a, c)
	allFailed(t, takeResults(t, a, c, []routeReq{{ID: 1, Src: 7, Dst: 6, Levels: []int{1}}}), 1, "destination 7")
	rs := takeResults(t, a, c, []routeReq{{ID: 2, Src: 7, Dst: 5, Levels: []int{1}}})
	if len(rs) != 1 || !rs[0].OK {
		t.Fatalf("results %+v", rs)
	}
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].Client != "PanelsBot (Panel 43)" || es[0].Actor == nil || es[0].Actor.Key != "Panel 43" {
		t.Fatalf("entry %+v", es)
	}
}

// A browser batch takes the same path as the API: same failed result per item, same row.
func TestRightsBrowserBatchParity(t *testing.T) {
	a, f, c := rightsApp(t, config.Rights{Dsts: ints(5)})
	defer startLog(t, a)()
	n := f.sends()
	a.serveTake(c, &inMsg{Items: []routeReq{
		{ID: 1, Src: 7, Dst: 6, Levels: []int{1}},
		{ID: 2, Src: 7, Dst: 7, Levels: []int{1}},
	}})
	var rs []routeResult
	for len(rs) < 2 {
		select {
		case b := <-c.send:
			var r routeResult
			if json.Unmarshal(b, &r) == nil && r.T == "takeResult" {
				rs = append(rs, r)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("results %+v", rs)
		}
	}
	allFailed(t, rs, 2, "destination 7")
	if f.sends() != n {
		t.Fatal("sent")
	}
	var api *Sent
	for _, e := range waitLogged(t, a, 2) {
		if e.Kind == kindAPI {
			api = &e
		}
	}
	if api == nil || api.By != "Ana" || api.Result != "refused" || api.Why != "take" {
		t.Fatalf("rows %+v", logged(t, a))
	}
}

package app

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func wIDs(t *testing.T, body map[string]any) []int {
	t.Helper()
	var ids []int
	for _, r := range body["results"].([]any) {
		e, ok := r.(map[string]any)["entry"].(float64)
		if !ok {
			t.Fatalf("a result has no entry: %v", body)
		}
		ids = append(ids, int(e))
	}
	return ids
}

func TestAPIWScopeNeeded(t *testing.T) {
	a, _, _ := takerApp(t, nil)
	names := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "NamesBot"}, "N", time.Time{})
	wantErr(t, apiPost(a, "/api/v1/takes", names, `{"routes":[{"src":1,"dst":2}]}`, nil), 403, "scope")
}

func TestAPIWBadRequests(t *testing.T) {
	a, _, key := takerApp(t, nil)
	for name, body := range map[string]string{
		"empty":   `{}`,
		"junk":    `nope`,
		"unknown": `{"routes":[{"src":1,"dst":2}],"extra":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			wantErr(t, apiPost(a, "/api/v1/takes", key, body, nil), 400, "bad_request")
		})
	}
	wantErr(t, apiPost(a, "/api/v1/history/x/undo", key, ``, nil), 400, "bad_request")
	wantErr(t, apiPost(a, "/api/v1/history/99/undo", key, ``, nil), 404, "not_found")
}

func TestAPIWRefusedWholeBeforeSending(t *testing.T) {
	a, f, key := takerApp(t, func(c *config.Config) { c.APIClients[0].Rights = config.Rights{Dsts: ints(2)} })
	defer startLog(t, a)()
	n := f.sends()
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]},{"src":4,"dst":5,"levels":[1]}]}`, nil)
	wantErr(t, rec, 403, "outside_rights")
	if !strings.Contains(rec.Body.String(), "destination 6") {
		t.Errorf("body %s", rec.Body.String())
	}
	if f.sends() != n || a.hist.Len() != 0 {
		t.Fatalf("sent %d, history %d for a refused take", f.sends()-n, a.hist.Len())
	}
	rows := waitLogged(t, a, 1)
	var refused *Sent
	for i := range rows {
		if rows[i].Kind == kindAPI && rows[i].Result == "refused" {
			refused = &rows[i]
		}
		if rows[i].Why == "take" && rows[i].Result == "sent" {
			t.Errorf("a refused take logged a command row: %+v", rows[i])
		}
	}
	if refused == nil || refused.By != "PanelsBot (Panel 43)" || !strings.Contains(refused.Err, "destination 6") {
		t.Fatalf("rows %+v", rows)
	}
}

func TestAPIWUndoRestoresOutsideSources(t *testing.T) {
	a, f, key := takerApp(t, func(c *config.Config) {
		c.APIClients[0].Rights = config.Rights{Sources: ints(4), Dsts: ints(2)}
	})
	f.seed(2, 1, 0)
	first := apiBody(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, nil))
	id := wIDs(t, first)[0]
	rec := apiPost(a, "/api/v1/history/"+strconv.Itoa(id)+"/undo", key, ``, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	r0 := apiBody(t, rec)["results"].([]any)[0].(map[string]any)
	if r0["src"] != float64(1) || r0["result"] != "connected" {
		t.Fatalf("undo result %v", r0)
	}
	es, _ := a.hist.Page(-1, 1)
	if es[0].Undo == nil || *es[0].Undo != id || es[0].Actor == nil {
		t.Fatalf("entry %+v", es[0])
	}
	a.mu.Lock()
	now := a.router.tally[2][1]
	a.mu.Unlock()
	if now != 1 {
		t.Fatalf("dst 2 level 1 is on %d, want 1", now)
	}
}

func TestAPIWUndoLeavesLevelsWithNoPrevious(t *testing.T) {
	a, _, key := takerApp(t, nil)
	// dst 7 has no known route, so nothing is restored and the plan says so
	first := apiBody(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":7,"levels":[1]}]}`, nil))
	id := wIDs(t, first)[0]
	wantErr(t, apiPost(a, "/api/v1/history/"+strconv.Itoa(id)+"/undo", key, ``, nil), 422, "nothing_to_do")
}

func TestAPIWRetryFailedAndRedoOk(t *testing.T) {
	a, f, key := takerApp(t, nil)
	f.seed(2, 1, 0)
	f.mu.Lock()
	f.ignore[2] = true
	f.mu.Unlock()
	first := apiBody(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, nil))
	r0 := first["results"].([]any)[0].(map[string]any)
	if r0["result"] != "failed" || first["ok"] != false || r0["entry"] == nil || r0["error"] == "" {
		t.Fatalf("failed take %v", first)
	}
	id := wIDs(t, first)[0]
	wantErr(t, apiPost(a, "/api/v1/history/"+strconv.Itoa(id)+"/redo", key, ``, nil), 409, "conflict")
	f.mu.Lock()
	delete(f.ignore, 2)
	f.mu.Unlock()
	rec := apiPost(a, "/api/v1/history/"+strconv.Itoa(id)+"/retry", key, ``, nil)
	if rec.Code != 200 || apiBody(t, rec)["ok"] != true {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 1)
	if es[0].Retry == nil || *es[0].Retry != id {
		t.Fatalf("entry %+v", es[0])
	}
	id2 := wIDs(t, apiBody(t, rec))[0]
	wantErr(t, apiPost(a, "/api/v1/history/"+strconv.Itoa(id2)+"/retry", key, ``, nil), 409, "conflict")
	if rec := apiPost(a, "/api/v1/history/"+strconv.Itoa(id2)+"/redo", key, ``, nil); rec.Code != 200 {
		t.Fatalf("redo %d %s", rec.Code, rec.Body.String())
	}
}

func TestAPIWPreset(t *testing.T) {
	a, f, key := takerApp(t, func(c *config.Config) {
		c.Presets = append(c.Presets, config.Preset{Name: "Gone", Items: []config.PresetItem{{Src: 1, Dsts: []int{99}}}},
			config.Preset{Name: "Half", Items: []config.PresetItem{{Src: 1, Dsts: []int{2, 98}, Levels: []int{1}}}})
	})
	f.seed(2, 3, 0)
	rec := apiPost(a, "/api/v1/presets/everyone/Studio%20A", key, ``, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	body := apiBody(t, rec)
	if body["ok"] != true || len(body["results"].([]any)) != 1 {
		t.Fatalf("body %s", rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 1)
	if es[0].Preset != "Studio A" {
		t.Fatalf("entry %+v", es[0])
	}
	half := apiBody(t, apiPost(a, "/api/v1/presets/everyone/Half", key, ``, nil))
	if lo, _ := half["leftOut"].([]any); len(lo) != 1 || lo[0] != "destination 99" {
		t.Fatalf("half %v", half)
	}
	wantErr(t, apiPost(a, "/api/v1/presets/everyone/Gone", key, ``, nil), 422, "nothing_to_do")
	wantErr(t, apiPost(a, "/api/v1/presets/everyone/Nope", key, ``, nil), 404, "not_found")
	wantErr(t, apiPost(a, "/api/v1/presets/me/Studio%20A", key, ``, nil), 404, "not_found")
}

func TestAPIWPresetRights(t *testing.T) {
	a, f, key := takerApp(t, func(c *config.Config) {
		c.APIClients[0].Rights = config.Rights{Presets: &[]string{"Other"}}
	})
	n := f.sends()
	wantErr(t, apiPost(a, "/api/v1/presets/everyone/Studio%20A", key, ``, nil), 403, "outside_rights")
	if f.sends() != n {
		t.Fatal("sent for a preset outside rights")
	}
}

func TestAPIWUserKeyFiresMePreset(t *testing.T) {
	a, f, _ := takerApp(t, func(c *config.Config) {
		for i := range c.Users {
			if c.Users[i].Name == "Ana" {
				c.Users[i].Profile.Presets = []config.Preset{{Name: "Mine", Items: []config.PresetItem{{Src: 1, Dsts: []int{2}, Levels: []int{1}}}}}
			}
		}
	})
	f.seed(2, 3, 0)
	key := apiTestKey(t, a, keyOwner{Kind: actorUser, Name: "Ana"}, "Desk", time.Time{})
	rec := apiPost(a, "/api/v1/presets/me/Mine", key, ``, nil)
	if rec.Code != 200 || apiBody(t, rec)["ok"] != true {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 1)
	if es[0].Actor == nil || es[0].Actor.Kind != actorUser || es[0].Client != "Ana (Desk)" {
		t.Fatalf("entry %+v", es[0])
	}
}

func TestAPIWBigTakeSaysCheckWasSkipped(t *testing.T) {
	a, _, key := takerApp(t, nil)
	small := apiBody(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, nil))
	if small["checkSkipped"] != false {
		t.Fatalf("small %v", small)
	}
	var rs []string
	for d := 10; d < 10+bigTake; d++ {
		rs = append(rs, `{"src":4,"dst":`+strconv.Itoa(d)+`,"levels":[1]}`)
	}
	big := apiBody(t, apiPost(a, "/api/v1/takes", key, `{"routes":[`+strings.Join(rs, ",")+`]}`, nil))
	if big["checkSkipped"] != true || len(big["results"].([]any)) != bigTake {
		t.Fatalf("big %v", big)
	}
}

func TestAPIWIdempotencyKey(t *testing.T) {
	a, f, key := takerApp(t, nil)
	f.seed(2, 1, 0)
	body := `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`
	h := map[string]string{"Idempotency-Key": "press-1"}
	first := apiPost(a, "/api/v1/takes", key, body, h)
	n, hist := f.sends(), a.hist.Len()
	again := apiPost(a, "/api/v1/takes", key, body, h)
	if again.Code != 200 || again.Body.String() != first.Body.String() || again.Header().Get("Idempotent-Replay") != "true" {
		t.Fatalf("repeat %d %q vs %q", again.Code, again.Body.String(), first.Body.String())
	}
	if f.sends() != n || a.hist.Len() != hist {
		t.Fatal("the repeat reached the router")
	}
	wantErr(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":5,"dst":2,"levels":[1]}]}`, h), 422, "idempotency_mismatch")
	wantErr(t, apiPost(a, "/api/v1/takes", key, body, map[string]string{"Idempotency-Key": "bad key"}), 400, "bad_request")
	// another key does not share the answer
	other := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Panel 44", time.Time{})
	if rec := apiPost(a, "/api/v1/takes", other, body, h); rec.Header().Get("Idempotent-Replay") != "" {
		t.Error("an Idempotency-Key leaked across keys")
	}
}

func TestAPIWIdempotentRepeatWaitsForFirst(t *testing.T) {
	a, f, key := takerApp(t, nil)
	f.seed(2, 1, 0)
	f.mu.Lock()
	f.hold = 60 * time.Millisecond
	f.mu.Unlock()
	body := `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`
	h := map[string]string{"Idempotency-Key": "k"}
	var wg sync.WaitGroup
	bodies := make([]string, 2)
	for i := range bodies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bodies[i] = apiPost(a, "/api/v1/takes", key, body, h).Body.String()
		}()
	}
	wg.Wait()
	if bodies[0] != bodies[1] || a.hist.Len() != 1 {
		t.Fatalf("bodies %q, history %d", bodies, a.hist.Len())
	}
}

func TestAPIWTakeCap(t *testing.T) {
	a, f, key := takerApp(t, func(c *config.Config) { c.API.TakeCap = 1 })
	a.routeTimeout = 400 * time.Millisecond
	f.mu.Lock()
	f.ignore[2] = true
	f.mu.Unlock()
	body := `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`
	done := make(chan struct{})
	go func() { apiPost(a, "/api/v1/takes", key, body, nil); close(done) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		a.apiS.take.mu.Lock()
		in := a.apiS.take.inflight
		a.apiS.take.mu.Unlock()
		if in == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the first take never started")
		}
		time.Sleep(5 * time.Millisecond)
	}
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":3,"levels":[1]}]}`, nil)
	wantErr(t, rec, 429, "take_cap")
	<-done
	// the slot is free again
	if rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":3,"levels":[1]}]}`, nil); rec.Code != 200 {
		t.Fatalf("after: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAPIWCommandRows(t *testing.T) {
	a, f, key := takerApp(t, nil)
	defer startLog(t, a)()
	f.seed(2, 1, 0)
	apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, map[string]string{"X-Sode-Via": "wall 3"})
	var row *Sent
	for _, r := range waitLogged(t, a, 1) {
		if r.Kind == kindAPI && r.Result == "sent" {
			row = &r
		}
	}
	if row == nil || row.Why != "take" || row.By != "PanelsBot (Panel 43)" || !strings.Contains(row.Note, "1 route") || !strings.Contains(row.Note, "unverified via: wall 3") {
		t.Fatalf("row %+v", row)
	}
}

func TestAPIWDeadlinePerRequest(t *testing.T) {
	a, _, _ := takerApp(t, nil)
	a.checkTimeout, a.queueMax, a.routeTimeout, a.routeGrace = time.Second, 30*time.Second, 3*time.Second, time.Second
	if d := a.takeDeadline(1); d < 35*time.Second || d > 60*time.Second {
		t.Fatalf("one route: %v, want above 35s and under a proxy's 60s cut", d)
	}
	if a.takeDeadline(3*routeLimit) <= a.takeDeadline(1) {
		t.Fatal("a larger take must get a longer deadline")
	}
}

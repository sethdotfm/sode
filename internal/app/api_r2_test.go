package app

import (
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func TestAPIHistoryActionsNeedTakeAndHistory(t *testing.T) {
	a, _, _ := takerApp(t, func(c *config.Config) {
		c.APIClients = append(c.APIClients, config.APIClient{Name: "TakeOnly", Scopes: []string{config.ScopeTake}})
	})
	key := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "TakeOnly"}, "T", time.Time{})
	for _, verb := range []string{"undo", "redo", "retry"} {
		wantErr(t, apiPost(a, "/api/v1/history/0/"+verb, key, ``, nil), 403, "scope")
	}
}

func TestAPI401SaysBearer(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	for name, key := range map[string]string{"bad key": "sode_nope", "no key": ""} {
		rec := apiDo(a, "GET", "/api/v1/names", key, nil)
		if rec.Code != 401 || rec.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s: %d %q", name, rec.Code, rec.Header().Get("WWW-Authenticate"))
		}
	}
}

func TestAPIWrongMethodOnTemplatedPathIs405(t *testing.T) {
	a, _, key := takerApp(t, nil)
	rec := apiDo(a, "GET", "/api/v1/history/3/undo", key, nil)
	wantErr(t, rec, 405, "method_not_allowed")
	if rec.Header().Get("Allow") != "POST" {
		t.Errorf("Allow %q", rec.Header().Get("Allow"))
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/presets/everyone/Studio%20A", key, nil), 405, "method_not_allowed")
	wantErr(t, apiDo(a, "GET", "/api/v1/nowhere/1", key, nil), 404, "not_found")
}

func TestAPIOversizedBodyIs413WithOwnCode(t *testing.T) {
	a, _, key := takerApp(t, nil)
	wantErr(t, apiPost(a, "/api/v1/takes", key, strings.Repeat(" ", apiTakeBody+1), nil), 413, "too_large")
	if !strings.Contains(string(openAPIFile), "too_large:") {
		t.Error("x-error-codes lacks too_large")
	}
}

func TestStreamStatusUsesAddressLikeGETStatus(t *testing.T) {
	a := fixStatusApp(t)
	c := strmDial(t, strmServer(t, a), apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "StatLog"}, "Wall", time.Time{}), "status")
	c.next() // hello
	m := c.next()
	var d map[string]any
	if err := json.Unmarshal(m.Data, &d); err != nil {
		t.Fatal(err)
	}
	if d["address"] != fixRouterAddr {
		t.Errorf("snapshot %s", m.Data)
	}
	if _, ok := d["addr"]; ok {
		t.Errorf("snapshot still has addr: %s", m.Data)
	}
}

func TestAPIRateTableStopsAtTrackMax(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	now := time.Now()
	for i := 0; i < apiTrackMax+50; i++ {
		a.apiRateOK(httptest.NewRecorder(), "ip:"+strconv.Itoa(i), 100, now)
	}
	a.apiS.mu.Lock()
	n := len(a.apiS.rate)
	a.apiS.mu.Unlock()
	if n > apiTrackMax {
		t.Fatalf("%d subjects tracked, cap %d", n, apiTrackMax)
	}
}

func TestStreamConnectWritesNoReadRow(t *testing.T) {
	a, key := viewApp(t, "")
	cfg := a.r().cfg
	cfg.API.LogReads = true
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	defer startLog(t, a)()
	c, _, ctx, cancel, err := strmDialRaw(strmServer(t, a), key, "names")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	_, _, _ = c.Read(ctx) // hello
	a.apiEvent(sodeActor, "marker", "ok", "", nil)
	waitLogged(t, a, 2)
	time.Sleep(100 * time.Millisecond)
	for _, e := range logged(t, a) {
		if e.Kind == kindAPI && e.Why == "read" {
			t.Errorf("stream wrote a read row: %+v", e)
		}
	}
}

func TestCleanTextTrimsAndCaps(t *testing.T) {
	if got := cleanText("  wall\x00 ui\n2  ", 32); got != "wall ui2" {
		t.Errorf("got %q", got)
	}
	if got := cleanText("abcdef", 3); got != "abc" {
		t.Errorf("got %q", got)
	}
}

func TestUserKeyScopesAreAllButLog(t *testing.T) {
	var want []string
	for _, s := range config.AllScopes {
		if s != config.ScopeLog {
			want = append(want, s)
		}
	}
	if !slices.Equal(userKeyScopes, want) {
		t.Errorf("got %v, want %v", userKeyScopes, want)
	}
}

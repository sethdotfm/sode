package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// healthTwoRouters: an API client and two routers, A and B, each with a fake router whose link is up.
func healthTwoRouters(t *testing.T) (a *App, fa, fb *fakeRouter, b *routerUnit, key string) {
	t.Helper()
	a, fa, fb, b, key = apiTwoRouters(t)
	fa.restore()
	fb.restore()
	return a, fa, fb, b, key
}

type wholeBody struct {
	State   string           `json:"state"`
	Router  string           `json:"router"`
	Routers []map[string]any `json:"routers"`
}

func decodeWhole(t *testing.T, b []byte) wholeBody {
	t.Helper()
	var w wholeBody
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatalf("body %q: %v", b, err)
	}
	return w
}

// links gives the link state of each router in the body, in order.
func (w wholeBody) links() []string {
	var out []string
	for _, r := range w.Routers {
		s, _ := r["router"].(string)
		out = append(out, s)
	}
	return out
}

// /api/health is 200 only when every router is up. One router down is Degraded and both down is Down, each 503.
func TestHealthDegradedAndDown(t *testing.T) {
	a, fa, fb, b, _ := healthTwoRouters(t)
	want := func(code int, state string, links ...string) {
		t.Helper()
		rec, _ := getHealth(t, a, http.MethodGet)
		w := decodeWhole(t, rec.Body.Bytes())
		if rec.Code != code || w.State != state || strings.Join(w.links(), ",") != strings.Join(links, ",") {
			t.Errorf("health %d %s, want %d %s %v", rec.Code, rec.Body, code, state, links)
		}
	}
	want(http.StatusOK, "healthy", "up", "up")
	rec, _ := getHealth(t, a, http.MethodGet)
	w := decodeWhole(t, rec.Body.Bytes())
	if len(w.Routers) != 2 || w.Routers[0]["uid"] != a.r().cfg.Routers[0].UID || w.Routers[1]["uid"] != b.uid() ||
		w.Routers[1]["prefix"] != "B" || w.Routers[1]["title"] != "Second" || w.Routers[1]["targetAddress"] != "10.0.0.9:8910" {
		t.Errorf("routers %s", rec.Body)
	}

	fb.drop(errors.New("connection refused"))
	want(http.StatusServiceUnavailable, "degraded", "up", "down")
	rec, _ = getHealth(t, a, http.MethodGet)
	if w := decodeWhole(t, rec.Body.Bytes()); w.Router != "" || w.Routers[0]["router"] != "up" || w.Routers[1]["err"] != "connection refused" {
		t.Errorf("no flat router, A up, B err: %s", rec.Body)
	}

	fa.drop(errors.New("connection refused"))
	want(http.StatusServiceUnavailable, "down", "down", "down")

	fb.restore()
	want(http.StatusServiceUnavailable, "degraded", "down", "up")
	fa.restore()
	want(http.StatusOK, "healthy", "up", "up")
}

// GET /status gives the state and every router without a router parameter. The address of each router needs the log
// scope.
func TestStatusGivesEveryRouter(t *testing.T) {
	a, _, fb, b, key := healthTwoRouters(t)
	rec := apiDo(a, "GET", "/api/v1/status", key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	if w := decodeWhole(t, rec.Body.Bytes()); w.State != "healthy" || len(w.Routers) != 2 || w.Routers[1]["uid"] != b.uid() {
		t.Errorf("status %s", rec.Body)
	}
	fb.drop(errors.New("dial tcp 10.0.0.9:8910: connection refused"))
	rec = apiDo(a, "GET", "/api/v1/status", key, nil)
	w := decodeWhole(t, rec.Body.Bytes())
	if w.State != "degraded" || strings.Join(w.links(), ",") != "up,down" || w.Routers[1]["routerUp"] != false ||
		w.Routers[1]["error"] != true || w.Routers[1]["prefix"] != "B" {
		t.Errorf("status %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.9") {
		t.Errorf("address without the log scope: %s", rec.Body)
	}
}

// The status topic needs no router parameter. Its snapshot and each change give the state and every router.
func TestStreamStatusGivesEveryRouter(t *testing.T) {
	a, fa, fb, _, key := healthTwoRouters(t)
	s := strmDial(t, strmServer(t, a), key, "status")
	s.next() // hello
	m := s.next()
	strmWantType(t, m, "snapshot", "status")
	if w := decodeWhole(t, m.Data); w.State != "healthy" || len(w.Routers) != 2 {
		t.Errorf("snapshot %s", m.Data)
	}
	fb.drop(errors.New("gone"))
	m = s.next()
	strmWantType(t, m, "status", "status")
	if w := decodeWhole(t, m.Data); w.State != "degraded" || strings.Join(w.links(), ",") != "up,down" {
		t.Errorf("one down %s", m.Data)
	}
	fa.drop(errors.New("gone"))
	m = s.next()
	if w := decodeWhole(t, m.Data); w.State != "down" || strings.Join(w.links(), ",") != "down,down" {
		t.Errorf("both down %s", m.Data)
	}
}

func TestWholeState(t *testing.T) {
	up, conn, down := statusMsg{Router: "up"}, statusMsg{Router: "connecting"}, statusMsg{Router: "down"}
	for _, c := range []struct {
		links []statusMsg
		want  string
	}{
		{[]statusMsg{up}, "healthy"},
		{[]statusMsg{up, up}, "healthy"},
		{[]statusMsg{up, down}, "degraded"},
		{[]statusMsg{conn, up}, "degraded"},
		{[]statusMsg{down}, "down"},
		{[]statusMsg{conn, down}, "down"},
	} {
		if got := wholeState(c.links); got != c.want {
			t.Errorf("%v: %s, want %s", c.links, got, c.want)
		}
	}
}

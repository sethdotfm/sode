package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// tieProtectApp: tieApp with Sode's device 126 on router A, and the API client with the protect scope and the
// Protect right.
func tieProtectApp(t *testing.T) (a *App, fa, fb *fakeRouter, key string) {
	t.Helper()
	return tieApp(t, func(c *config.Config) {
		c.Routers[0].ProtectDevice = 126
		c.APIClients[0].Scopes = append(c.APIClients[0].Scopes, config.ScopeProtect)
		on := true
		c.APIClients[0].Rights.Protect = &on
	})
}

// protectBody is tieRoutes with protect on TIE 3.
func protectBody(uidA string, src, dst int) string {
	return fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":%d},"dst":{"router":"b2b2b2b2","n":%d},"levels":[1],"ties":[{"id":"t1","expect":{"router":%q,"n":3},"protect":true}]}]}`,
		uidA, src, dst, uidA)
}

type tieProtectAnswer struct {
	OK       bool               `json:"ok"`
	Results  []apiRouteOut      `json:"results"`
	Protects []apiTieProtectOut `json:"protects"`
}

func tieProtectOf(t *testing.T, rec interface{ Bytes() []byte }) tieProtectAnswer {
	t.Helper()
	var b tieProtectAnswer
	if err := json.Unmarshal(rec.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

// sendOrder notes the connects and Protects that the units of fa and fb send, in the order sent.
func sendOrder(fa, fb *fakeRouter) func() []string {
	var mu sync.Mutex
	var out []string
	for _, f := range []*fakeRouter{fa, fb} {
		name, send := "A", f.u.sendMsg
		if f == fb {
			name = "B"
		}
		f.u.sendMsg = func(m []byte, why swp08.Why) error {
			mu.Lock()
			switch m[0] {
			case swp08.CmdExtConnect:
				out = append(out, fmt.Sprintf("%s connect %d", name, int(m[3])<<8|int(m[4])))
			case swp08.CmdExtProtectConnect:
				out = append(out, fmt.Sprintf("%s protect %d", name, int(m[3])<<8|int(m[4])))
			}
			mu.Unlock()
			return send(m, why)
		}
	}
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Compact(slices.Clone(out))
	}
}

// Sode protects the upstream destination of TIE 3 after every hop, on its router. The Protect is its own entry in
// the Group of the take.
func TestAPITieTakeProtectsAfterHops(t *testing.T) {
	a, fa, fb, key := tieProtectApp(t)
	uidA := a.r().cfg.Routers[0].UID
	order := sendOrder(fa, fb)
	rec := apiPost(a, "/api/v1/takes", key, protectBody(uidA, 4, 2), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := order(); !slices.Equal(got, []string{"A connect 10", "B connect 2", "A protect 10"}) {
		t.Fatalf("sent %v", got)
	}
	b := tieProtectOf(t, rec.Body)
	if !b.OK || len(b.Protects) != 1 {
		t.Fatalf("answer %s", rec.Body)
	}
	p := b.Protects[0]
	if !p.OK || p.Tie != "t1" || p.TieName != "TIE 3" || p.Router != uidA || p.Dst != 10 || p.Entry == nil {
		t.Fatalf("protect %+v", p)
	}
	if hs := fa.holdsOn(10); len(hs) != a.r().routerOf(uidA).Levels {
		t.Fatalf("holds on A.10 %+v", hs)
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 3 {
		t.Fatalf("entries %+v", es)
	}
	pe := es[0]
	if pe.ID != *p.Entry || pe.Protect != "set" || pe.Router != uidA || pe.Dst != 10 || !pe.OK {
		t.Fatalf("protect entry %+v", pe)
	}
	for _, e := range es {
		if e.Group == "" || e.Group != pe.Group || e.GroupN != 3 {
			t.Fatalf("group of #%d %q/%d, want %q/3", e.ID, e.Group, e.GroupN, pe.Group)
		}
	}

	// The plan names who set the Protect and when, and the tie line now carries A.4.
	rec = apiPost(a, "/api/v1/takes", key, ends(uidA, "b2b2b2b2", 6, 20), nil)
	wantErr(t, rec, http.StatusConflict, "no_path")
	tb := tieBody(t, rec.Body)
	if len(tb.Routes) != 1 || len(tb.Routes[0].Paths) != 1 {
		t.Fatalf("routes %s", rec.Body)
	}
	h := tb.Routes[0].Paths[0].Hops[0]
	if h.Protect != "PanelsBot (Panel 43)" || h.ProtectSince == nil || !tb.Routes[0].Paths[0].Protected {
		t.Fatalf("hop %+v", h)
	}

	// A Protected tie line that carries the source stays usable: nothing goes upstream.
	rec = apiPost(a, "/api/v1/takes", key, ends(uidA, "b2b2b2b2", 4, 20), nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_needed")
	if p := tieBody(t, rec.Body).Routes[0].Paths[0]; p.Protected || p.Hops[0].Sends {
		t.Fatalf("path %+v", p)
	}
	rec = apiPost(a, "/api/v1/takes", key, strings.Replace(protectBody(uidA, 4, 20), `"n":3}`, `"n":4}`, 1), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	b = tieProtectOf(t, rec.Body)
	if !b.OK || len(b.Results[0].Hops) != 1 || b.Results[0].Hops[0].Result != "already" {
		t.Fatalf("answer %s", rec.Body)
	}
	// The Protect on A.10 is there already, so this second Protect fails. The take stays a success.
	if len(b.Protects) != 1 || b.Protects[0].OK || b.Protects[0].Entry == nil || len(b.Protects[0].Still) == 0 {
		t.Fatalf("protects %+v", b.Protects)
	}
}

// A Protect that the router does not confirm leaves the take a success. Its entry is failed, with Still.
func TestAPITieTakeProtectFails(t *testing.T) {
	a, fa, _, key := tieProtectApp(t)
	uidA := a.r().cfg.Routers[0].UID
	fa.mu.Lock()
	fa.prot.deafSet = map[int]bool{a.r().byUI[2].Wire: true}
	fa.mu.Unlock()
	rec := apiPost(a, "/api/v1/takes", key, protectBody(uidA, 4, 2), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	b := tieProtectOf(t, rec.Body)
	if !b.OK || b.Results[0].Result != "connected" || len(b.Protects) != 1 || b.Protects[0].OK || b.Protects[0].Error == "" {
		t.Fatalf("answer %s", rec.Body)
	}
	e, ok := a.hist.Get(*b.Protects[0].Entry)
	if !ok || e.OK || e.Protect != "set" || !slices.Contains(e.Failed, 2) {
		t.Fatalf("entry %+v", e)
	}
}

// When a route fails, Sode does not send its Protect. The Protect entry says so, and the Group keeps its size.
func TestAPITieTakeProtectNotSentOnFailure(t *testing.T) {
	a, fa, fb, key := tieProtectApp(t)
	uidA := a.r().cfg.Routers[0].UID
	fb.mu.Lock()
	fb.ignore[2] = true
	fb.mu.Unlock()
	rec := apiPost(a, "/api/v1/takes", key, protectBody(uidA, 4, 2), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	b := tieProtectOf(t, rec.Body)
	if b.OK || len(b.Protects) != 1 || b.Protects[0].OK || !strings.Contains(b.Protects[0].Error, "not sent") {
		t.Fatalf("answer %s", rec.Body)
	}
	if n := fa.cmds(swp08.CmdExtProtectConnect); n != 0 {
		t.Fatalf("%d Protect messages", n)
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 3 || es[0].Protect != "set" || es[0].OK || es[0].GroupN != 3 {
		t.Fatalf("entries %+v", es)
	}
}

// A take that asks for a Protect needs the protect scope and the Protect right. Sode refuses it before it sends
// anything.
func TestAPITieTakeProtectNeedsRight(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*config.Config)
		code string
	}{
		{"no scope", func(c *config.Config) {
			on := true
			c.APIClients[0].Rights.Protect = &on
		}, "scope"},
		{"no right", func(c *config.Config) {
			c.APIClients[0].Scopes = append(c.APIClients[0].Scopes, config.ScopeProtect)
		}, "outside_rights"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, fa, fb, key := tieApp(t, func(c *config.Config) {
				c.Routers[0].ProtectDevice = 126
				tc.mut(c)
			})
			sa, sb := fa.sends(), fb.sends()
			rec := apiPost(a, "/api/v1/takes", key, protectBody(a.r().cfg.Routers[0].UID, 4, 2), nil)
			wantErr(t, rec, http.StatusForbidden, tc.code)
			if fa.sends() != sa || fb.sends() != sb {
				t.Fatal("a router got a message")
			}
		})
	}
}

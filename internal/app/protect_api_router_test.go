package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// POST and DELETE /protected/{dst} act on the router of the router parameter. With two routers, a call with no
// router is 400 router_needed and sends nothing.
func TestAPIProtectUsesItsRouter(t *testing.T) {
	a, first, second, u := protectTwoRouters(t)
	cfg := a.r().cfg.Clone()
	cfg.APIClients = []config.APIClient{{Name: "Holder", Scopes: []string{config.ScopeProtect}, Rights: config.Rights{Protect: ptr(true)}}}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	holder := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "Holder"}, "k", time.Time{})

	if rec := apiSend(a, "POST", "/api/v1/protected/5", holder, ""); rec.Code != 400 || apiErrCode(t, rec) != "router_needed" {
		t.Fatalf("no router: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "POST", "/api/v1/protected/5?router="+u.uid(), holder, ""); rec.Code != 200 || apiBody(t, rec)["ok"] != true {
		t.Fatalf("set on B: %d %s", rec.Code, rec.Body.String())
	}
	if n, hs := first.sends(), second.holdsOn(5); n != 0 || len(hs) != 3 || hs[1].dev != 127 {
		t.Fatalf("set on B: the first router got %d messages, holds on B %+v", n, hs)
	}
	if rec := apiSend(a, "DELETE", "/api/v1/protected/5", holder, ""); rec.Code != 400 || apiErrCode(t, rec) != "router_needed" {
		t.Fatalf("free with no router: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "DELETE", "/api/v1/protected/5?router="+u.uid(), holder, ""); rec.Code != 200 || apiBody(t, rec)["ok"] != true {
		t.Fatalf("free on B: %d %s", rec.Code, rec.Body.String())
	}
	if n, hs := first.sends(), second.holdsOn(5); n != 0 || len(hs) != 0 {
		t.Fatalf("free on B: the first router got %d messages, holds on B %+v", n, hs)
	}
	if rec := apiSend(a, "POST", "/api/v1/protected/5?router="+a.r().cfg.UID, holder, ""); rec.Code != 200 || len(first.holdsOn(5)) != 3 {
		t.Fatalf("set on A: %d %s", rec.Code, rec.Body.String())
	}
}

// The page's Protect names the router of each destination: a destination of B gets its Protect on B. The same
// number on A in one message is a different destination.
func TestPageProtectUsesItsRouter(t *testing.T) {
	a, first, second, u := protectTwoRouters(t)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	r := p.ask("protectSet", 3, map[string]any{"protect": map[string]any{"dsts": []map[string]any{{"dst": 5, "router": u.uid()}, {"dst": 5}}}})
	if r["ok"] != true {
		t.Fatalf("result %v", r)
	}
	if hs, ha := second.holdsOn(5), first.holdsOn(5); len(hs) != 3 || hs[1].dev != 127 || len(ha) != 3 || ha[1].dev != 126 {
		t.Fatalf("holds on B %+v, on A %+v", hs, ha)
	}
	r = p.ask("protectClear", 4, map[string]any{"protect": map[string]any{"dsts": []map[string]any{{"dst": 5, "router": u.uid()}}}})
	if r["ok"] != true || len(second.holdsOn(5)) != 0 || len(first.holdsOn(5)) != 3 {
		t.Fatalf("free on B: %v, holds on B %+v, on A %+v", r, second.holdsOn(5), first.holdsOn(5))
	}
	if r := p.ask("protectSet", 5, map[string]any{"protect": map[string]any{"dsts": []map[string]any{{"dst": 5, "router": "b0b0b0b0"}}}}); r["ok"] != false {
		t.Fatalf("unknown router: %v", r)
	}
}

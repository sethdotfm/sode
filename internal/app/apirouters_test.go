package app

import (
	"net/http"
	"testing"
)

func TestAPIRoutersListsUIDPrefixAndTitle(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	cfg := a.r().cfg
	rec := apiDo(a, "GET", "/api/v1/routers", botKey(t, a), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	list, _ := apiBody(t, rec)["routers"].([]any)
	if len(list) != 1 {
		t.Fatalf("routers %v", list)
	}
	r := list[0].(map[string]any)
	if r["uid"] != cfg.Routers[0].UID || r["prefix"] != "A" || r["title"] != cfg.Routers[0].Title || len(r) != 3 {
		t.Errorf("router %v, want uid %q prefix A title %q", r, cfg.Routers[0].UID, cfg.Routers[0].Title)
	}
}

func TestAPIRouterParameter(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key, uid := botKey(t, a), a.r().cfg.Routers[0].UID
	for _, path := range []string{"/api/v1/names", "/api/v1/tally"} {
		if rec := apiDo(a, "GET", path+"?router="+uid, key, nil); rec.Code != http.StatusOK {
			t.Errorf("%s with the right UID: %d %s", path, rec.Code, rec.Body)
		}
		for _, bad := range []string{"0123456789abcdef", "A"} {
			rec := apiDo(a, "GET", path+"?router="+bad, key, nil)
			if rec.Code != http.StatusNotFound || apiErrCode(t, rec) != "unknown_router" {
				t.Errorf("%s with router %q: %d %s", path, bad, rec.Code, rec.Body)
			}
		}
	}
	rec := apiDo(a, "POST", "/api/v1/takes?router=nope", key, nil)
	if rec.Code != http.StatusNotFound || apiErrCode(t, rec) != "unknown_router" {
		t.Errorf("take with a wrong router: %d %s", rec.Code, rec.Body)
	}
}

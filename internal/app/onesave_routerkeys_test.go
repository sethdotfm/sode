package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Every router setting saves as router/<uid>/<attribute>, for router B and for the first router alike. The hello's
// routers list then carries the new value, and the old flat keys are no keys.
func TestOneSaveEveryRouterKey(t *testing.T) {
	a, uidB := routerLevelsApp(t)
	uidA := a.r().cfg.Routers[0].UID
	values := map[string]any{
		"title": "Stage", "numberOffset": 0, "nameRereadSeconds": 120, "nameLength": 16, "nameRequests": "ext",
		"tallyResyncSeconds": 60, "fireMode": "native", "salvoGroup": 9, "protectDevice": 77, "levelWireOffset": 0,
	}
	check := func(r config.Router, who string) {
		t.Helper()
		if r.Title != "Stage" || r.NumberOffset != 0 || r.NameRefreshSeconds != 120 || r.NameLength != 16 ||
			r.NameRequests != "ext" || r.TallyResyncSeconds != 60 || r.FireMode != "native" || r.SalvoGroup != 9 ||
			r.ProtectDevice != 77 || r.LevelWireOffset != 0 {
			t.Errorf("router %s after the save: %+v", who, r)
		}
	}
	for i, uid := range []string{uidB, uidA} {
		p := dialPeer(t, a)
		p.signInRechecked("Ada", "2468")
		kv := map[string]any{}
		for attr, v := range values {
			kv["router/"+uid+"/"+attr] = v
		}
		r := p.saveLive(a, i+1, kv)
		for k := range kv {
			wantSaved(t, r, k)
		}
		rc, _ := a.r().cfg.RouterByUID(uid)
		check(rc, rc.Prefix)
	}

	// The hello's routers list carries every setting of every router.
	a.mu.Lock()
	h := asMap(t, a.hello(helloPage{}))
	a.mu.Unlock()
	for _, x := range h["routers"].([]any) {
		r := x.(map[string]any)
		if r["fireMode"] != "native" || r["salvoGroup"] != 9.0 || r["protectDevice"] != 77.0 || r["nameRequests"] != "ext" ||
			r["nameLength"] != 16.0 || r["nameRereadSeconds"] != 120.0 || r["tallyResyncSeconds"] != 60.0 || r["numberOffset"] != 0.0 {
			t.Errorf("hello router %v", r)
		}
	}

	// The flat keys of the first router are gone from the one Save.
	for _, k := range []string{"router", "routerName", "matrix", "levels", "levelWireOffset", "levelNumbering", "numberOffset",
		"defaultLevels", "nameLength", "nameRequests", "nameRereadSeconds", "tallyResyncSeconds", "fireMode", "salvoGroup",
		"protectDevice"} {
		if _, ok := lookupField(k); ok {
			t.Errorf("%s is still a one Save key", k)
		}
	}
}

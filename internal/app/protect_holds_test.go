package app

import (
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

func TestProtectRecOwnedBy(t *testing.T) {
	user := protectRec{User: "u1", By: "Ana"}
	client := protectRec{Client: "Bot", By: "Bot"}
	guest := protectRec{By: "10.0.0.9"}
	guestAct := Actor{Kind: actorGuest}
	userAct := Actor{Kind: actorUser, User: "u1"}
	clientAct := Actor{Kind: actorClient, Name: "Bot"}
	for _, tc := range []struct {
		name         string
		rec          *protectRec
		user, client string
		act          Actor
		want         bool
	}{
		{"no record", nil, "u1", "", userAct, false},
		{"same user", &user, "u1", "", userAct, true},
		{"other user", &user, "u2", "", Actor{Kind: actorUser, User: "u2"}, false},
		{"user's own client counts as the user", &user, "u1", "", clientAct, true},
		{"same client", &client, "", "Bot", clientAct, true},
		{"other client", &client, "", "Other", Actor{Kind: actorClient, Name: "Other"}, false},
		{"user on a client's record", &client, "u1", "", userAct, false},
		{"guest on a guest's record", &guest, "", "", guestAct, true},
		{"guest on a user's record", &user, "", "", guestAct, false},
		{"no owner and not a guest", &guest, "", "", Actor{Kind: actorSode}, false},
	} {
		if got := tc.rec.ownedBy(tc.user, tc.client, tc.act); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The page's copy of the router's kinds must match the server's.
func TestProtectRulesKindsMatchSWP08(t *testing.T) {
	b, err := os.ReadFile("../../web/protectrules.js")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{"KIND_PROTECT": swp08.KindProtect, "KIND_ROUTER_LOCK": swp08.KindRouterLock} {
		m := regexp.MustCompile(`export const ` + name + ` = (\d+);`).FindSubmatch(b)
		if m == nil {
			t.Fatalf("%s not found in web/protectrules.js", name)
		}
		if got, _ := strconv.Atoi(string(m[1])); got != want {
			t.Errorf("%s = %d in web/protectrules.js, swp08 has %d", name, got, want)
		}
	}
}

// /api/v1/protected marks Sode's holds by the rule of protectLevelsMu: Sode's device now, or the record's device.
func TestAPIProtectedUsesSodesHoldRule(t *testing.T) {
	a, _ := protectApp(t)
	cfg := a.r().cfg.Clone()
	cfg.APIClients = []config.APIClient{{Name: "Reader", Scopes: []string{config.ScopeProtected}}}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.first.status.Router = string(swp08.StateUp)
	a.mu.Unlock()
	uid := a.r().cfg.Routers[0].UID
	// dst 3: on Sode's device (126) with no record. dst 5: on 126, the record has an earlier device (120).
	// dst 6: on the record's earlier device. dst 7: another device.
	for _, rec := range []protectRec{
		{Router: uid, Dst: 5, Device: 120, Levels: []int{1, 2, 3}, By: "Ana"},
		{Router: uid, Dst: 6, Device: 120, Levels: []int{1, 2, 3}, By: "Bo"},
	} {
		if err := a.protects.put(rec); err != nil {
			t.Fatal(err)
		}
	}
	for dst, dev := range map[int]int{3: 126, 5: 126, 6: 120, 7: 112} {
		for wire := range 3 {
			a.onUnitMsg(a.first, swp08.ExtProtectChange(swp08.CmdExtProtectTally, 0, wire, swp08.KindProtect, dst, dev))
		}
	}
	a.flush()
	key := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "Reader"}, "k", time.Time{})
	rows := apiBody(t, apiDo(a, "GET", "/api/v1/protected", key, nil))["protected"].([]any)
	want := map[int]string{3: "", 5: "Ana", 6: "Bo", 7: ""}
	seen := map[int]int{}
	for _, r := range rows {
		m := r.(map[string]any)
		dst := int(m["dst"].(float64))
		user, _ := m["sodeUser"].(string)
		if m["kind"] != "protect" || user != want[dst] {
			t.Errorf("row %v: want sodeUser %q", m, want[dst])
		}
		seen[dst]++
	}
	for dst := range want {
		if seen[dst] != 3 {
			t.Errorf("dst %d: %d rows, want 3", dst, seen[dst])
		}
	}

	// The hold on Sode's device with no record is still Sode's own for a clear.
	a.protMu.Lock()
	rec, own, others := a.sodeHoldsProtLocked(a.r(), a.first, 3)
	a.protMu.Unlock()
	if rec != nil || len(own) != 3 || len(others) != 0 {
		t.Fatalf("dst 3: rec %v, own %v, others %v", rec, own, others)
	}
}

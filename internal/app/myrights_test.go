package app

import (
	"encoding/json"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func mrLast(msgs []map[string]any, typ string) map[string]any {
	var out map[string]any
	for _, m := range msgs {
		if m["t"] == typ {
			out = m
		}
	}
	return out
}

func mrJSON(m map[string]any) string {
	b, _ := json.Marshal(m["rights"])
	return string(b)
}

func TestMyRightsReachOnlyTheSignedInUser(t *testing.T) {
	a := keyTestApp(t)
	srcs := *nums(7)
	none := *nums()
	mustSave(t, a, adminClient(t, a), map[string]any{ukey(a, "Ana", "rights"): config.Rights{Sources: &srcs, Dsts: &none, Visibility: config.VisHidden}})

	guest := &client{ip: "10.0.0.7"}
	gnext := registerAll(a, guest)
	a.syncAccess(guest)
	// A guest gets the guest's Rights, never a user's; the Guest role sets the Protect right.
	if m := mrLast(gnext(), "myRights"); m == nil || mrJSON(m) != `{"protect":false}` {
		t.Fatalf("guest rights: %v", m)
	}
	editCfg(t, a, func(cfg *config.Config) { cfg.GuestRole = config.RoleTrusted })
	a.syncAccess(guest)
	if m := mrLast(gnext(), "myRights"); m == nil || mrJSON(m) != `{"protect":true}` {
		t.Fatalf("Trusted user guest rights: %v", m)
	}

	ana, anext := signedIn(t, a, "10.0.0.1", "1234")
	m := mrLast(anext(), "myRights")
	if m == nil {
		t.Fatal("Ana got no myRights after sign-in")
	}
	if got := mrJSON(m); got != `{"dsts":[],"protect":false,"sources":[{"n":7,"router":"`+a.r().cfg.UID+`"}],"visibility":"hidden"}` {
		t.Fatalf("Ana's rights %s", got)
	}

	_, bnext := signedIn(t, a, "10.0.0.2", "567890")
	bm := mrLast(bnext(), "myRights")
	if bm == nil {
		t.Fatal("Bo got no myRights")
	}
	if got := mrJSON(bm); got != `{"protect":false}` {
		t.Fatalf("Bo's rights %s", got)
	}

	dsts := *nums(3)
	mustSave(t, a, adminClient(t, a), map[string]any{ukey(a, "Ana", "rights"): config.Rights{Dsts: &dsts}})
	m = mrLast(anext(), "myRights")
	if m == nil {
		t.Fatal("no myRights after the edit")
	}
	if got := mrJSON(m); got != `{"dsts":[{"n":3,"router":"`+a.r().cfg.UID+`"}],"protect":false}` {
		t.Fatalf("Ana's rights after the edit %s", got)
	}

	a.UserLock(ana)
	m = mrLast(anext(), "myRights")
	if m == nil || m["rights"] != nil {
		t.Fatalf("held Lock left rights on the page: %v", m)
	}
}

// A guest under the page lock acts as nobody, so it gets no Rights.
func TestMyRightsSkipAGuestUnderThePageLock(t *testing.T) {
	a := userApp(t, true)
	guest := &client{ip: "10.0.0.7"}
	gnext := registerAll(a, guest)
	a.syncAccess(guest)
	if m := mrLast(gnext(), "myRights"); m != nil && m["rights"] != nil {
		t.Fatalf("locked guest got rights: %v", m)
	}
}

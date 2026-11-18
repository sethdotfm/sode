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

func mrSet(t *testing.T, a *App, edits []userEdit) {
	t.Helper()
	if err := a.SaveUsers(adminClient(t, a), usersReq{Users: edits}); err != nil {
		t.Fatal(err)
	}
}

func mrJSON(m map[string]any) string {
	b, _ := json.Marshal(m["rights"])
	return string(b)
}

func TestMyRightsReachOnlyTheSignedInUser(t *testing.T) {
	a := keyTestApp(t)
	srcs := *config.Nums(7)
	none := *config.Nums()
	mrSet(t, a, []userEdit{
		{ID: idOf(a, "Ana"), Name: "Ana", Rights: &config.Rights{Sources: &srcs, Dsts: &none, Visibility: config.VisHidden}},
		{ID: idOf(a, "Bo"), Name: "Bo"},
	})

	guest := &client{ip: "10.0.0.7"}
	gnext := registerAll(a, guest)
	a.syncAccess(guest)
	if m := mrLast(gnext(), "myRights"); m != nil {
		t.Fatalf("guest got rights: %v", m)
	}

	ana, anext := signedIn(t, a, "10.0.0.1", "1234")
	m := mrLast(anext(), "myRights")
	if m == nil {
		t.Fatal("Ana got no myRights after sign-in")
	}
	if got := mrJSON(m); got != `{"dsts":[],"sources":[{"n":7,"router":"`+a.r().cfg.UID+`"}],"visibility":"hidden"}` {
		t.Fatalf("Ana's rights %s", got)
	}

	_, bnext := signedIn(t, a, "10.0.0.2", "567890")
	bm := mrLast(bnext(), "myRights")
	if bm == nil {
		t.Fatal("Bo got no myRights")
	}
	if got := mrJSON(bm); got != `{}` {
		t.Fatalf("Bo's rights %s", got)
	}

	// An administrator's edit reaches the open page at once.
	dsts := *config.Nums(3)
	mrSet(t, a, []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", Rights: &config.Rights{Dsts: &dsts}}, {ID: idOf(a, "Bo"), Name: "Bo"}})
	m = mrLast(anext(), "myRights")
	if m == nil {
		t.Fatal("no myRights after the edit")
	}
	if got := mrJSON(m); got != `{"dsts":[{"n":3,"router":"`+a.r().cfg.UID+`"}]}` {
		t.Fatalf("Ana's rights after the edit %s", got)
	}

	// The held Lock covers the page: the rights are cleared.
	a.UserLock(ana)
	m = mrLast(anext(), "myRights")
	if m == nil || m["rights"] != nil {
		t.Fatalf("held Lock left rights on the page: %v", m)
	}
}

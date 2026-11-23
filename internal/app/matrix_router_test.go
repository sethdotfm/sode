package app

import (
	"slices"
	"sync"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// matrixTwoRouters: protectApp (matrix 0, sequential, 8-character names) with a second router, B, on matrix 1. B
// has 16-character ext name requests and native fire in group 9. sent gives each message that B's link got.
func matrixTwoRouters(t *testing.T) (a *App, first, second *fakeRouter, u *routerUnit, sent func() []swp08.Request) {
	t.Helper()
	a, first = protectApp(t)
	cfg := a.r().cfg.Clone()
	b := moreRouter("B", "Second", "10.0.0.9:8910", 3)
	b.Matrix, b.NameLength, b.NameRequests, b.FireMode, b.SalvoGroup, b.ProtectDevice = 1, 16, "ext", "native", 9, 127
	cfg.MoreRouters = []config.Router{b}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	second, u = addFakeUnit(t, a, a.r().cfg.MoreRouters[0])
	var mu sync.Mutex
	var got []swp08.Request
	inner := u.sendMsg
	u.sendMsg = func(m []byte, why swp08.Why) error {
		mu.Lock()
		got = append(got, swp08.DescribeTx(m))
		mu.Unlock()
		return inner(m, why)
	}
	a.mu.Lock()
	u.status.Router = string(swp08.StateUp)
	a.mu.Unlock()
	sent = func() []swp08.Request {
		mu.Lock()
		defer mu.Unlock()
		out := slices.Clone(got)
		got = got[:0]
		return out
	}
	return a, first, second, u, sent
}

// onlyMatrix fails the test for a message to B that names a matrix other than 1.
func onlyMatrix(t *testing.T, what string, rs []swp08.Request) {
	t.Helper()
	if len(rs) == 0 {
		t.Fatalf("%s: nothing sent", what)
	}
	for _, r := range rs {
		if r.Matrix != -1 && r.Matrix != 1 {
			t.Errorf("%s: %s on matrix %d, want 1", what, r.Kind, r.Matrix)
		}
	}
}

// Each router uses its own settings: the matrix of every send and of every message Sode reads. Its name requests,
// fire mode and group are its own too.
func TestRouterUsesItsMatrix(t *testing.T) {
	a, first, _, u, sent := matrixTwoRouters(t)

	a.takeAllowed(&client{ip: "10.0.0.1"}, []routeReq{{ID: 1, Src: 4, Dst: 2, Levels: []int{1}, Router: u.uid}})
	onlyMatrix(t, "take", sent())
	if es, _ := a.hist.Page(-1, 1); len(es) != 1 || !es[0].OK {
		t.Fatalf("take on B: %+v", es)
	}
	if n := first.sends(); n != 0 {
		t.Fatalf("the first router got %d messages", n)
	}

	// Native fire on B, in B's group; the first router stays sequential.
	a.takeAllowed(&client{ip: "10.0.0.1"}, []routeReq{
		{ID: 2, Src: 5, Dst: 3, Levels: []int{1}, Router: u.uid},
		{ID: 3, Src: 5, Dst: 4, Levels: []int{1}, Router: u.uid},
	})
	rs := sent()
	onlyMatrix(t, "native take", rs)
	stages := 0
	for _, r := range rs {
		if r.Kind == "salvoStage" || r.Kind == "salvoStageExt" || r.Kind == "salvoGo" {
			stages++
			if r.Group != 9 {
				t.Errorf("%s in group %d, want 9", r.Kind, r.Group)
			}
		}
	}
	if stages == 0 {
		t.Errorf("B's fire mode is native, but nothing was staged: %+v", rs)
	}

	a.requestDumps(u)
	onlyMatrix(t, "dumps", sent())

	u.sendMsg = func(m []byte, why swp08.Why) error {
		if want := swp08.NamesReq(m[0] == swp08.CmdExtDstNamesReq, true, 1, 0, 3); !slices.Equal(m, want) {
			t.Errorf("name request % x, want % x", m, want)
		}
		return nil
	}
	a.refreshNames(u)
	u.sendMsg = func([]byte, swp08.Why) error { return nil }

	// A Connected and a Protect from matrix 1 are B's; from matrix 0 they are of another matrix.
	a.onUnitMsg(u, swp08.ExtConnected(1, 0, 7, 11))
	a.onUnitMsg(u, swp08.ExtConnected(0, 0, 6, 12))
	a.onUnitMsg(u, swp08.ExtProtectChange(swp08.CmdExtProtectConnected, 1, 0, 1, 7, 50))
	a.onUnitMsg(u, swp08.ExtProtectChange(swp08.CmdExtProtectConnected, 0, 0, 1, 6, 50))
	a.mu.Lock()
	src7, seen6 := u.router.tally[7][1], u.router.tally[6] != nil
	_, prot7 := u.router.protect[7][1]
	_, prot6 := u.router.protect[6][1]
	a.mu.Unlock()
	if src7 != 11 || seen6 {
		t.Errorf("tally of B: dst 7 = %d, dst 6 seen %v", src7, seen6)
	}
	if !prot7 || prot6 {
		t.Errorf("Protect of B: dst 7 %v, dst 6 %v", prot7, prot6)
	}
}

// A Protect on B uses B's matrix.
func TestProtectUsesItsMatrix(t *testing.T) {
	a, _, second, u, sent := matrixTwoRouters(t)
	ana := signedClient(t, a, "Ana", "1234")
	e := protEntry(t, a.protectMany(ana, []protectDst{{Dst: 4, Router: u.uid}}, true, nil)[0])
	onlyMatrix(t, "protect", sent())
	if !e.OK || len(second.holdsOn(4)) != 3 {
		t.Fatalf("entry %+v, holds %+v", e, second.holdsOn(4))
	}
}

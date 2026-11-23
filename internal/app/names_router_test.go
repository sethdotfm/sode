package app

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// twoRouterFiles gives an App with a config file and two routers. The second has a unit and a fake router of its own.
func twoRouterFiles(t *testing.T) (a *App, first, second *fakeRouter, u *routerUnit) {
	t.Helper()
	cfg := config.Defaults()
	cfg.Levels, cfg.NameRefreshSeconds, cfg.TallyResyncSeconds = 3, 60, 60
	b := moreRouter("B", "Second", "10.0.0.9:8910", 3)
	b.NameRefreshSeconds, b.TallyResyncSeconds = 60, 60
	cfg.MoreRouters = []config.Router{b}
	withAdministrator(&cfg)
	cfg.Normalize()
	h, _ := OpenHistory("")
	a, err := New(cfg, filepath.Join(settledTempDir(t), "config.yaml"), h, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	shortTimers(a)
	first = newFakeRouter(t, a)
	second, u = addFakeUnit(t, a, a.r().cfg.MoreRouters[0])
	return a, first, second, u
}

// readUnitNames is a full name read on one router: the request, the replies, then the quiet time.
func readUnitNames(a *App, u *routerUnit, srcs, dsts []string) {
	a.mu.Lock()
	u.router.newSrc, u.router.newDst = map[int]string{}, map[int]string{}
	u.nameReqAt = time.Now()
	u.applyNames(&swp08.NameBlock{First: 0, Names: srcs})
	u.applyNames(&swp08.NameBlock{Dest: true, First: 0, Names: dsts})
	u.nameRxAt = u.nameRxAt.Add(-time.Second)
	u.nameReqAt = u.nameReqAt.Add(-time.Second)
	a.mu.Unlock()
	a.flush()
	a.writes.wg.Wait()
}

func unitTablePath(a *App, uid string) string {
	return filepath.Join(filepath.Dir(a.cfgPath), "names", uid+".yaml")
}

// A name read, the saved names table and the hello each use their own router.
func TestNamesUseTheirRouter(t *testing.T) {
	a, first, second, u := twoRouterFiles(t)
	a.refreshNames(u)
	if n, m := first.sentFor(swp08.ReasonNames), second.sentFor(swp08.ReasonNames); n != 0 || m != 2 {
		t.Fatalf("name reads: first router %d, second router %d", n, m)
	}

	readUnitNames(a, u, []string{"B CAM 1"}, []string{"B MON 1"})
	bPath, aPath := unitTablePath(a, u.uid()), unitTablePath(a, a.r().cfg.UID)
	b, err := os.ReadFile(bPath)
	if err != nil || !strings.Contains(string(b), "router: "+u.uid()) || !strings.Contains(string(b), "B CAM 1") {
		t.Fatalf("second router's names table: %v\n%s", err, b)
	}
	if _, err := os.Stat(aPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the first router's names table was written: %v", err)
	}
	a.mu.Lock()
	firstNames := len(a.firstUnit().router.srcNames)
	var hm struct {
		Srcs json.RawMessage `json:"srcs"`
	}
	json.Unmarshal(a.hello(helloPage{}), &hm)
	a.mu.Unlock()
	// The hello's routers list has them under the second router; its flat fields are the first router's.
	if h := string(hm.Srcs); firstNames != 0 || strings.Contains(h, "B CAM 1") {
		t.Fatalf("the second router's names reached the first router: %d names, hello %s", firstNames, h)
	}

	// Each router has its own record of its file: the first router's write does not make the second write again.
	readUnitNames(a, a.firstUnit(), []string{"A CAM 1"}, []string{"A MON 1"})
	if b, err := os.ReadFile(aPath); err != nil || !strings.Contains(string(b), "A CAM 1") {
		t.Fatalf("first router's names table: %v\n%s", err, b)
	}
	os.Remove(bPath)
	readUnitNames(a, u, []string{"B CAM 1"}, []string{"B MON 1"})
	if _, err := os.Stat(bPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second router's names table written with no change: %v", err)
	}

	// At start each router reads its own file.
	a2, err := New(a.r().cfg, a.cfgPath, a.hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := a2.firstUnit().router.srcNames[0]; got != "A CAM 1" {
		t.Fatalf("first router at start: %q", got)
	}
}

// API reads and stream frames come from the router of the request.
func TestAPIReadsUseTheirRouter(t *testing.T) {
	a, _, _, u := twoRouterFiles(t)
	readUnitNames(a, u, []string{"B CAM 1"}, []string{"B MON 1"})
	if got := a.apiUnit(httptest.NewRequest("GET", apiBase+"/names?router="+u.uid(), nil)); got != u {
		t.Fatalf("router parameter gave the wrong unit")
	}
	if got := a.apiUnit(httptest.NewRequest("GET", apiBase+"/names", nil)); got != a.firstUnit() {
		t.Fatalf("no router parameter is not the first router")
	}
	a.mu.Lock()
	fromB := a.namesSnapshot(u, apiView{})
	fromA := a.namesSnapshot(a.firstUnit(), apiView{})
	a.mu.Unlock()
	if len(fromB.Sources) != 1 || fromB.Sources[0].Name != "B CAM 1" || len(fromA.Sources) != 0 {
		t.Fatalf("names: second %+v, first %+v", fromB, fromA)
	}

	st := &apiStream{unit: a.firstUnit()}
	if st.hears(u) || !st.hears(a.firstUnit()) || !st.hears(nil) {
		t.Fatal("a stream hears another router's frames")
	}
}

// The link status and the link row of a router name that router.
func TestLinkUsesItsRouter(t *testing.T) {
	a, _, _, u := twoRouterFiles(t)
	events(a)
	a.onUnitState(u, swp08.StateDown, errors.New("refused"))
	a.mu.Lock()
	bState, aState := u.status.Router, a.firstUnit().status.Router
	bAddr := u.status.Addr
	a.mu.Unlock()
	if bState != string(swp08.StateDown) || aState == string(swp08.StateDown) || bAddr != "10.0.0.9:8910" {
		t.Fatalf("status: second %q at %q, first %q", bState, bAddr, aState)
	}
	var row *Sent
	for _, e := range events(a) {
		if e.Kind == kindLink {
			row = &e
		}
	}
	if row == nil || row.Router != u.uid() || row.Note != "10.0.0.9:8910" {
		t.Fatalf("link row %+v", row)
	}
}

// Re-reads run for each router that is up, on that router's link.
func TestRereadsUseTheirRouter(t *testing.T) {
	a, first, second, u := twoRouterFiles(t)
	a.mu.Lock()
	u.status.Router = string(swp08.StateUp)
	a.firstUnit().status.Router = string(swp08.StateDown)
	a.mu.Unlock()
	a.periodic()
	if n := first.sentFor(swp08.ReasonNames) + first.sentFor(swp08.ReasonResync); n != 0 {
		t.Fatalf("the first router is down but got %d re-reads", n)
	}
	if second.sentFor(swp08.ReasonNames) != 2 || second.sentFor(swp08.ReasonResync) == 0 {
		t.Fatalf("second router: %d name reads, %d route re-reads", second.sentFor(swp08.ReasonNames), second.sentFor(swp08.ReasonResync))
	}
}

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// settleNames makes the name replies old enough for flush to finish the read.
func settleNames(a *App) {
	a.mu.Lock()
	a.nameRxAt = a.nameRxAt.Add(-time.Second)
	a.nameReqAt = a.nameReqAt.Add(-time.Second)
	a.mu.Unlock()
	a.flush()
	a.writes.wg.Wait()
}

func giveNames(a *App, dest bool, names ...string) {
	a.mu.Lock()
	a.applyNames(&swp08.NameBlock{Dest: dest, First: 0, Names: names})
	a.mu.Unlock()
}

// readNames is a full read: the request, the replies, then the quiet time.
func readNames(a *App, srcs, dsts []string) {
	a.mu.Lock()
	a.router.newSrc, a.router.newDst = map[int]string{}, map[int]string{}
	a.nameReqAt = time.Now()
	a.mu.Unlock()
	giveNames(a, false, srcs...)
	giveNames(a, true, dsts...)
	settleNames(a)
}

func namesTablePath(a *App) string {
	return filepath.Join(filepath.Dir(a.cfgPath), "names", a.r().cfg.UID+".yaml")
}

func TestNamesTableWrittenOnlyWhenNamesChange(t *testing.T) {
	a := futureApp(t, "routers: [{address: 127.0.0.1:1}]\n")
	path := namesTablePath(a)
	readNames(a, []string{"CAM 1", "CAM 2"}, []string{"MON 1"})
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no names table after the first read: %v", err)
	}
	for _, want := range []string{"router: " + a.r().cfg.UID, "CAM 2", "MON 1"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("names table has no %q:\n%s", want, b)
		}
	}

	os.Remove(path)
	readNames(a, []string{"CAM 1", "CAM 2"}, []string{"MON 1"})
	giveNames(a, false, "CAM 1")
	settleNames(a)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("names table written with no change: %v", err)
	}

	giveNames(a, true, "MON A")
	settleNames(a)
	if b, err := os.ReadFile(path); err != nil || !strings.Contains(string(b), "MON A") {
		t.Fatalf("names table not written after a change: %v\n%s", err, b)
	}
}

type savedHello struct {
	Srcs    [][2]any  `json:"srcs"`
	At      time.Time `json:"at"`
	SavedAt time.Time `json:"savedAt"`
}

func helloOf(t *testing.T, a *App) savedHello {
	t.Helper()
	a.mu.Lock()
	b := a.hello(helloPage{})
	a.mu.Unlock()
	var h savedHello
	if err := json.Unmarshal(b, &h); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSavedNamesShowUntilFirstRead(t *testing.T) {
	a := futureApp(t, "routers: [{address: 127.0.0.1:1}]\n")
	readNames(a, []string{"CAM 1", "CAM 2"}, []string{"MON 1"})
	cfg := a.r().cfg

	hist, _ := OpenHistory("")
	b, err := New(cfg, a.cfgPath, hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	h := helloOf(t, b)
	if len(h.Srcs) != 2 || h.Srcs[1][1] != "CAM 2" {
		t.Fatalf("saved sources not loaded: %v", h.Srcs)
	}
	if !h.At.IsZero() || h.SavedAt.IsZero() {
		t.Fatalf("before the first read: at %v savedAt %v", h.At, h.SavedAt)
	}

	b.mu.Lock()
	b.router.newSrc, b.router.newDst = map[int]string{}, map[int]string{}
	b.nameReqAt = time.Now().Add(-namesTimeout - time.Second)
	b.mu.Unlock()
	b.flush()
	if h := helloOf(t, b); h.SavedAt.IsZero() {
		t.Fatal("a read with no replies cleared the saved time")
	}

	readNames(b, []string{"CAM 1", "CAM 2"}, []string{"MON 1"})
	h = helloOf(t, b)
	if h.At.IsZero() || !h.SavedAt.IsZero() {
		t.Fatalf("after the first read: at %v savedAt %v", h.At, h.SavedAt)
	}
}

func TestSavedNamesIgnoreOtherRouterAndBadFile(t *testing.T) {
	a := futureApp(t, "routers: [{address: 127.0.0.1:1}]\n")
	path := namesTablePath(a)
	os.MkdirAll(filepath.Dir(path), 0o700)
	for name, body := range map[string]string{
		"other router": "router: 0123456789abcdef\nat: 2026-11-12T14:02:00Z\nsources:\n  - {n: 0, name: CAM 1}\n",
		"not yaml":     "router: [\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		hist, _ := OpenHistory("")
		b, err := New(a.r().cfg, a.cfgPath, hist, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if h := helloOf(t, b); len(h.Srcs) != 0 || !h.SavedAt.IsZero() {
			t.Errorf("%s: loaded %v at %v", name, h.Srcs, h.SavedAt)
		}
	}
}

func TestRemoveNamesTable(t *testing.T) {
	a := futureApp(t, "routers: [{address: 127.0.0.1:1}]\n")
	readNames(a, []string{"CAM 1"}, []string{"MON 1"})
	path := namesTablePath(a)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := a.names.remove(a.r().cfg.UID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("names table still there: %v", err)
	}
	if err := a.names.remove(a.r().cfg.UID); err != nil {
		t.Fatalf("second remove: %v", err)
	}
}

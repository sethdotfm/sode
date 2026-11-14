package app

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// startSim builds cmd/swp08sim and runs it on a free port, returning its address. Never a
// live Ultrix: the only router a browser check may reach is this one.
func startSim(t *testing.T) string {
	addr, _ := startSimStoppable(t)
	return addr
}

var simBuild struct {
	once     sync.Once
	dir, bin string
	err      error
	out      []byte
}

// simBinary builds cmd/swp08sim once per test binary; TestMain removes it.
func simBinary(t *testing.T) string {
	t.Helper()
	simBuild.once.Do(func() {
		if simBuild.dir, simBuild.err = os.MkdirTemp("", "sode-swp08sim-"); simBuild.err != nil {
			return
		}
		simBuild.bin = filepath.Join(simBuild.dir, "swp08sim")
		build := exec.Command("go", "build", "-o", simBuild.bin, "./cmd/swp08sim")
		build.Dir = repoRoot()
		simBuild.out, simBuild.err = build.CombinedOutput()
	})
	if simBuild.err != nil {
		t.Fatalf("build swp08sim: %v\n%s", simBuild.err, simBuild.out)
	}
	return simBuild.bin
}

// startSimStoppable also returns a func that kills the simulator mid-test.
func startSimStoppable(t *testing.T) (string, func()) {
	t.Helper()
	browserTest(t)
	if _, err := exec.LookPath("go"); err != nil {
		browserSkip(t, "browser checks skipped: go is not on PATH, so swp08sim cannot be built")
	}
	bin := simBinary(t)
	// The port is free when picked but another agent's simulator can take it before ours binds, so
	// ready means our own "swp08sim on" line, and an early exit retries on a new port.
	for try := 0; try < 3; try++ {
		if addr, stop, ok := runSim(t, bin); ok {
			return addr, stop
		}
	}
	t.Fatal("swp08sim did not start listening")
	return "", nil
}

func runSim(t *testing.T, bin string) (addr string, stop func(), ok bool) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SIM_PORT=%d", port))
	logs, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	ready, exited := make(chan struct{}), make(chan struct{})
	go func() {
		sc := bufio.NewScanner(logs)
		for seen := false; sc.Scan(); {
			if !seen && strings.Contains(sc.Text(), "swp08sim on ") {
				seen = true
				close(ready)
			}
		}
		_ = cmd.Wait()
		close(exited)
	}()
	t.Cleanup(func() { cancel(); <-exited })
	select {
	case <-ready:
		return fmt.Sprintf("127.0.0.1:%d", port), cancel, true
	case <-exited:
		return "", nil, false
	case <-time.After(10 * time.Second):
		t.Fatal("swp08sim did not start listening")
		return "", nil, false
	}
}

// simApp is an app with user Ana (PIN 1234) and the API on, connected to the simulator through
// the real SW-P-08 client (apiTestApp swaps in a fake router).
func simApp(t *testing.T, router string, edit ...func(*config.Config)) *App {
	t.Helper()
	cfg := config.Defaults()
	cfg.Router = router
	cfg.API.Enabled = true
	cfg.Users = []config.User{mkUser(t, "Ana", "1234")}
	for _, fn := range edit {
		fn(&cfg)
	}
	cfg.Normalize()
	hist, _ := OpenHistory("")
	sl, err := OpenSentLog(filepath.Join(t.TempDir(), "sent.jsonl"), 200_000)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(cfg, filepath.Join(settledTempDir(t), "config.yaml"), hist, sl, false)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// The simulator routes destination d from source d at the start, so DSM1 (index 2) shows CG1 Fill.
const (
	simSrc     = 10 // "Zoom 1"
	simDst     = 2  // "DSM1"
	simSrcName = "Zoom 1"
)

// Panel launch in Session mode: the page asks for a PIN, the signed-in user's key reads the
// panel, and stage plus Take switch a destination on the simulated router.
func TestChromePanelSessionTake(t *testing.T) {
	b := startBrowser(t) // first: a box with no Chrome skips before building the simulator
	sim := startSim(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	// The named instance holds its stream for as long as the page lives, and the page outlives srv.
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Browser panel","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"source","n":%d},
		{"r":1,"c":3,"kind":"take"}],"settings":{"mode":"take"}}`, simDst, simSrc))

	// Names and tally arrive after the router connects; the page only re-reads names every 30 s,
	// so open it once they are there.
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1") &&
			strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"routes":[{`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the app never read names and tally from swp08sim: %.300s | %.300s", apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String())
		}
	}
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")

	// Launch: nobody is signed in, so the page raises the PIN pad before anything else.
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.shot("pin")
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)

	b.waitFor("panel title", `document.getElementById('panel-title').textContent === 'Browser panel'`)
	b.waitFor("three cells", `document.querySelectorAll('.sode-panel__cell').length === 3`)
	cell := func(kind string) string { return fmt.Sprintf(`document.querySelector('.sode-panel__cell--%s')`, kind) }
	b.waitFor("destination named from the router", cell("destination")+`?.querySelector('span')?.textContent === 'DSM1'`)
	b.waitFor("current source under the destination", cell("destination")+`?.querySelector('small')?.textContent === 'CG1 Fill'`)
	b.shot("launched")

	b.eval(cell("destination") + `.click()`)
	b.eval(cell("source") + `.click()`)
	b.waitFor("Take armed", `!`+cell("take")+`.disabled`)
	b.eval(cell("take") + `.click()`)
	b.waitFor("Connected result", cell("destination")+`?.dataset.result === 'connected'`)
	b.waitFor("new source under the destination", cell("destination")+`?.querySelector('small')?.textContent === '`+simSrcName+`'`)
	b.shot("taken")

	// The router itself, read through the REST seam, not the page's own picture of it.
	rec := apiDo(a, "GET", "/api/v1/tally", ana, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, simDst, simSrc)) {
		t.Errorf("tally after take: %d %s", rec.Code, rec.Body.String())
	}
	// The Communication Log names the panel and the instance; a user's key proves neither, so it says so.
	if e := lastRow(t, a, "take"); !strings.Contains(e.Note, "unverified panel "+id+" (Browser panel), instance Desk") {
		t.Errorf("take row %+v", e)
	}
	b.check()
}

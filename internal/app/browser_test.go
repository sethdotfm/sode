package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// Browser seam: headless Chrome driven over the DevTools protocol against the real Handler and a
// swp08sim router. How to run and add a check: docs/agents/browser-checks.md.

// chromePath returns a Chrome or Chromium binary, or "" when none is installed.
func chromePath() string {
	if p := os.Getenv("SODE_CHROME"); p != "" {
		return p
	}
	for _, p := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, n := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}

type cdpReply struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// tab is one page target. Console errors, uncaught exceptions and failed loads collect in
// problems, which check() turns into a test failure.
type tab struct {
	t       *testing.T
	conn    *websocket.Conn
	mu      sync.Mutex
	next    int
	pending map[int]chan cdpReply
	errs    []string
}

// startBrowser skips the test when no Chrome is installed.
func startBrowser(t *testing.T) *tab {
	t.Helper()
	bin := chromePath()
	if bin == "" {
		t.Skip("browser checks skipped: no Chrome or Chromium found (install one or set SODE_CHROME)")
	}
	// Chrome sometimes exits at start-up when other Chrome processes are busy, so a missing port gets a fresh
	// profile and process, up to three times.
	var port string
	var ctx context.Context
	var cancel context.CancelFunc
	for try := 0; try < 3 && port == ""; try++ {
		// Chrome's helper processes can still write to the profile after the main one exits, which makes
		// t.TempDir's single RemoveAll fail with "directory not empty" on a slow runner.
		dir := settledTempDir(t)
		ctx, cancel = context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, bin, "--headless=new", "--remote-debugging-port=0", "--user-data-dir="+dir,
			"--no-first-run", "--no-default-browser-check", "--disable-gpu", "--window-size=1280,800", "about:blank")
		if err := cmd.Start(); err != nil {
			cancel()
			t.Skipf("browser checks skipped: cannot start %s: %v", bin, err)
		}
		c := cancel
		t.Cleanup(func() { c(); _ = cmd.Wait() })

		// Chrome writes the port it chose into the profile directory.
		for deadline := time.Now().Add(6 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if b, err := os.ReadFile(filepath.Join(dir, "DevToolsActivePort")); err == nil {
				port = strings.SplitN(string(b), "\n", 2)[0]
				break
			}
		}
		if port == "" {
			c()
		}
	}
	if port == "" {
		t.Fatalf("%s did not report a DevTools port after 3 tries", bin)
	}
	var ws string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && ws == ""; time.Sleep(50 * time.Millisecond) {
		res, err := http.Get("http://127.0.0.1:" + port + "/json/list")
		if err != nil {
			continue
		}
		var list []struct{ Type, WebSocketDebuggerURL string }
		_ = json.NewDecoder(res.Body).Decode(&list)
		res.Body.Close()
		for _, x := range list {
			if x.Type == "page" {
				ws = x.WebSocketDebuggerURL
			}
		}
	}
	if ws == "" {
		t.Fatal("no page target in the browser")
	}
	conn, _, err := websocket.Dial(ctx, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadLimit(32 << 20)
	b := &tab{t: t, conn: conn, pending: map[int]chan cdpReply{}}
	t.Cleanup(func() { conn.CloseNow() })
	go b.read(ctx)
	// Page.bringToFront: since Chrome 157 a background tab's requestFullscreen fails "not granted", even
	// with a user gesture.
	for _, m := range []string{"Page.enable", "Runtime.enable", "Log.enable", "Page.bringToFront"} {
		b.call(m, nil)
	}
	return b
}

func (b *tab) read(ctx context.Context) {
	for {
		_, data, err := b.conn.Read(ctx)
		if err != nil {
			return
		}
		var m struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			cdpReply
		}
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		if m.Method != "" {
			b.event(m.Method, m.Params)
			continue
		}
		b.mu.Lock()
		ch := b.pending[m.ID]
		delete(b.pending, m.ID)
		b.mu.Unlock()
		if ch != nil {
			ch <- m.cdpReply
		}
	}
}

func (b *tab) event(method string, params json.RawMessage) {
	var p struct {
		Type string `json:"type"`
		Args []struct {
			Value       any
			Description string
		} `json:"args"`
		ExceptionDetails struct {
			Text      string                       `json:"text"`
			Exception struct{ Description string } `json:"exception"`
		} `json:"exceptionDetails"`
		Entry struct{ Level, Text, URL string } `json:"entry"`
	}
	_ = json.Unmarshal(params, &p)
	var msg string
	switch method {
	case "Runtime.exceptionThrown":
		msg = "exception: " + p.ExceptionDetails.Text + " " + p.ExceptionDetails.Exception.Description
	case "Runtime.consoleAPICalled":
		if p.Type != "error" {
			return
		}
		var parts []string
		for _, a := range p.Args {
			parts = append(parts, fmt.Sprint(a.Value, a.Description))
		}
		msg = "console.error: " + strings.Join(parts, " ")
	case "Log.entryAdded":
		// The pages link Google Fonts, which a CI box without internet cannot load.
		if p.Entry.Level != "error" || strings.Contains(p.Entry.URL, "fonts.g") {
			return
		}
		msg = "log: " + p.Entry.Text + " " + p.Entry.URL
	default:
		return
	}
	b.mu.Lock()
	b.errs = append(b.errs, msg)
	b.mu.Unlock()
}

func (b *tab) call(method string, params any) json.RawMessage {
	b.t.Helper()
	b.mu.Lock()
	b.next++
	id := b.next
	ch := make(chan cdpReply, 1)
	b.pending[id] = ch
	b.mu.Unlock()
	data, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := b.conn.Write(ctx, websocket.MessageText, data); err != nil {
		b.t.Fatalf("%s: %v", method, err)
	}
	select {
	case r := <-ch:
		if r.Error != nil {
			b.t.Fatalf("%s: %s", method, r.Error.Message)
		}
		return r.Result
	case <-ctx.Done():
		b.t.Fatalf("%s: no answer", method)
	}
	return nil
}

func (b *tab) open(url string) {
	b.t.Helper()
	b.call("Page.navigate", map[string]any{"url": url})
}

// eval runs a JS expression in the page (promises are awaited) and returns its JSON value.
func (b *tab) eval(expr string) any {
	b.t.Helper()
	v, err := b.evalTry(expr)
	if err != "" {
		b.t.Fatalf("eval %q: %s", expr, err)
	}
	return v
}

// evalTry reports a page exception as a string instead of failing the test.
func (b *tab) evalTry(expr string) (any, string) {
	b.t.Helper()
	raw := b.call("Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true, "awaitPromise": true})
	var r struct {
		Result           struct{ Value any }
		ExceptionDetails *struct{ Text string }
	}
	_ = json.Unmarshal(raw, &r)
	if r.ExceptionDetails != nil {
		return nil, r.ExceptionDetails.Text
	}
	return r.Result.Value, ""
}

// waitFor polls a JS expression until it is truthy. A miss fails the test, naming the check
// and any page errors, so a missing element reads as what it is.
func (b *tab) waitFor(what, expr string) {
	b.t.Helper()
	var last string
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		// A poll that lands mid-reload throws because the page has no DOM yet; keep polling.
		v, exc := b.evalTry(expr)
		if exc != "" {
			last = exc
			continue
		}
		if v != nil && v != false && v != "" {
			return
		}
	}
	b.shot("timeout")
	b.t.Fatalf("timed out waiting for %s (%s); last exception %q; page errors: %v", what, expr, last, b.problems())
}

func (b *tab) problems() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.errs...)
}

// check fails the test on any console error or exception seen so far.
func (b *tab) check() {
	b.t.Helper()
	for _, e := range b.problems() {
		b.t.Errorf("page error: %s", e)
	}
}

// shot saves a PNG into SODE_BROWSER_SHOTS when that directory is set.
func (b *tab) shot(name string) {
	dir := os.Getenv("SODE_BROWSER_SHOTS")
	if dir == "" {
		return
	}
	var r struct{ Data []byte }
	_ = json.Unmarshal(b.call("Page.captureScreenshot", map[string]any{"format": "png"}), &r)
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(b.t.Name(), "/", "_")+"-"+name+".png"), r.Data, 0o644)
}

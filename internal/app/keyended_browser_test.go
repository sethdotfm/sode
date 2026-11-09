package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// apiTap records every /api/v1 answer: which key sent it and the status.
type apiTap struct {
	mu   sync.Mutex
	rows []tapRow
}

type tapRow struct {
	key    string
	status int
}

type tapWriter struct {
	http.ResponseWriter
	status int
}

func (w *tapWriter) WriteHeader(s int) {
	if w.status == 0 {
		w.status = s
	}
	w.ResponseWriter.WriteHeader(s)
}

func (w *tapWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.ResponseWriter.Write(b)
}

func (w *tapWriter) Flush() { w.ResponseWriter.(http.Flusher).Flush() }

func (w *tapWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (t *apiTap) wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			h.ServeHTTP(w, r)
			return
		}
		tw := &tapWriter{ResponseWriter: w}
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		// A held stream answers 200 at once; record it then, not when it closes.
		t.mu.Lock()
		i := len(t.rows)
		t.rows = append(t.rows, tapRow{key: key})
		t.mu.Unlock()
		h.ServeHTTP(tw, r)
		t.mu.Lock()
		t.rows[i].status = tw.status
		t.mu.Unlock()
	})
}

func (t *apiTap) since(n int, pick func(tapRow) bool) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := 0
	for _, r := range t.rows[n:] {
		if pick(r) {
			c++
		}
	}
	return c
}

func (t *apiTap) len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.rows)
}

// The user's failure: open pages kept polling with keys Sode had ended, every call counted as a failed key,
// and the address (every tab and the editor on that machine) was locked out. Two tabs sharing one session
// also ended each other's key on every renewal. Now: tabs keep their own keys, an ended session key is
// renewed once, an ended link key stops its page, and the address is never locked.
func TestChromeEndedKeysStopWithoutLockingTheAddress(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Persistent = true // tabs share the session token, as for a wall user
		c.Users[0].Trusted = true
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	tap := &apiTap{}
	srv := httptest.NewServer(tap.wrap(a.Handler(web.FS)))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","acts":"both","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	sessKeys := func() int {
		a.keys.mu.Lock()
		defer a.keys.mu.Unlock()
		n := 0
		for _, r := range a.keys.recs {
			if r.Sess != "" {
				n++
			}
		}
		return n
	}
	until := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); !ok(); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}
	session := func(r tapRow) bool { return isSessionShape(r.key) }
	refused := func(r tapRow) bool { return r.status == 401 || r.status == 429 }
	addressOK := func() {
		t.Helper()
		if n := failCount(a, "127.0.0.1"); n != 0 {
			t.Fatalf("%d failed keys counted against the address", n)
		}
		if rec := apiDoFrom(a, "127.0.0.1:1", "GET", "/api/v1/names", ana, nil); rec.Code != 200 {
			t.Fatalf("the address is locked out: %d %s", rec.Code, rec.Body.String())
		}
	}

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("the panel", `document.querySelectorAll('.sode-panel__cell').length === 1 && document.getElementById('panels-note')?.hidden`)

	// A second tab on the same session: the panels page, which polls every 2 s.
	b.call("Target.createTarget", map[string]any{"url": srv.URL + "/panels"})
	until("a key for each tab", func() bool { return sessKeys() >= 2 })
	mark := tap.len()
	until("both tabs calling with their keys", func() bool { return tap.since(mark, func(r tapRow) bool { return session(r) && r.status == 200 }) > 10 })
	if n := tap.since(0, func(r tapRow) bool { return session(r) && refused(r) }); n != 0 {
		t.Fatalf("tabs on one session refused %d times: they end each other's keys", n)
	}

	// A restart of a memory-only Sode forgets the keys; the session stays.
	a.keys.dropWhere(func(r keyRec) bool { return r.Sess != "" })
	mark = tap.len()
	until("both tabs renewed", func() bool { return sessKeys() >= 2 })
	until("calls working again", func() bool {
		return tap.since(mark, func(r tapRow) bool { return session(r) && r.status == 200 }) > 10
	})
	if n := tap.since(mark, func(r tapRow) bool { return session(r) && refused(r) }); n > 14 {
		t.Errorf("%d refused calls to renew two tabs' keys", n)
	}
	b.waitFor("the panel page runs on", `document.getElementById('panels-note')?.hidden`)
	addressOK()

	// A link with its own key, in this tab; the panels page keeps running in the other.
	secret, _ := mintKey(t, a, ana, id)
	b.open(srv.URL + "/panels/" + id + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix))
	link := func(r tapRow) bool { return r.key == secret }
	until("the link polling", func() bool { return tap.since(0, func(r tapRow) bool { return link(r) && r.status == 200 }) > 5 })
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key?instance=Lobby", ana, ""); rec.Code != 204 && rec.Code != 200 {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	mark = tap.len()
	b.waitFor("the page says its key ended", `/no longer works/.test(document.getElementById('panels-note')?.textContent || '')`)
	after := tap.since(mark, link)
	// Absence needs time to show: longer than a poll, an update check and a keepalive retry.
	time.Sleep(7 * time.Second)
	if n := tap.since(mark, link); n-after > 0 || n > 8 {
		t.Errorf("the stopped page sent %d calls with its ended key, %d after it stopped", n, n-after)
	}
	if tap.since(mark, func(r tapRow) bool { return session(r) && r.status == 200 }) == 0 {
		t.Error("the panels page in the other tab stopped working")
	}
	addressOK()
	// The refused calls above log as failed loads; anything else is a real page error.
	for _, e := range b.problems() {
		if !strings.Contains(e, "status of 401") {
			t.Errorf("page error: %s", e)
		}
	}
}

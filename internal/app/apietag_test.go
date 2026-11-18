package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// ifNone reads path with If-None-Match set when tag is not empty. A 304 that carries a body gives -1.
func ifNone(a *App, path, key, tag string) (code int, etag string) {
	var hdr map[string]string
	if tag != "" {
		hdr = map[string]string{"If-None-Match": tag}
	}
	rec := apiDo(a, "GET", path, key, hdr)
	if rec.Code == 304 && rec.Body.Len() != 0 {
		return -1, ""
	}
	return rec.Code, rec.Header().Get("ETag")
}

func etagApp(t *testing.T) *App {
	t.Helper()
	a := panelsApp(t)
	cfg := a.r().cfg
	cfg.APIClients[0].Scopes = append(cfg.APIClients[0].Scopes, config.ScopePanels)
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return a
}

const etagPanel = `{"name":"Wall","share":{"mode":"everyone"},"cells":[{"kind":"source","n":1}]}`

// If-None-Match takes a list, a weak tag and *. A tag is over the caller's own body. So under narrower Rights, a
// tag made under wider Rights gets the whole body, not 304.
func TestETagListWeakStarAndOtherRights(t *testing.T) {
	a := etagApp(t)
	ana := userKey(t, a, "Ana")
	path := "/api/v1/panels/" + makePanel(t, a, ana, etagPanel)
	_, tag := ifNone(a, path, ana, "")
	for _, hdr := range []string{"W/" + tag, `"other", ` + tag, "*"} {
		if code, _ := ifNone(a, path, ana, hdr); code != 304 {
			t.Errorf("If-None-Match %s: %d, want 304", hdr, code)
		}
	}
	if code, _ := ifNone(a, path, ana, `"other"`); code != 200 {
		t.Errorf("another tag: %d, want 200", code)
	}
	bot := botKey(t, a)
	for _, p := range []string{"/api/v1/tally", "/api/v1/names"} {
		_, wide := ifNone(a, p, bot, "")
		cfg := a.r().cfg
		cfg.APIClients[0].Rights = config.Rights{Visibility: config.VisHidden, Dsts: &[]int{1}, Sources: &[]int{1}}
		if err := a.ApplyConfig(cfg); err != nil {
			t.Fatal(err)
		}
		if code, narrow := ifNone(a, p, bot, wide); code != 200 || narrow == wide {
			t.Errorf("%s under narrower Rights with the wide tag: %d, ETag %q", p, code, narrow)
		}
		cfg.APIClients[0].Rights = config.Rights{}
		if err := a.ApplyConfig(cfg); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPanelReadsAnswer304WhenNothingChanged(t *testing.T) {
	a := etagApp(t)
	id := makePanel(t, a, userKey(t, a, "Ana"), etagPanel)
	bot := botKey(t, a)
	for _, path := range []string{"/api/v1/tally", "/api/v1/protected", "/api/v1/names", "/api/v1/panels/" + id} {
		code, tag := ifNone(a, path, bot, "")
		if code != 200 || tag == "" || tag[0] != '"' {
			t.Fatalf("%s: first read %d, ETag %q", path, code, tag)
		}
		if code, again := ifNone(a, path, bot, tag); code != 304 || again != tag {
			t.Errorf("%s: unchanged read %d, ETag %q", path, code, again)
		}
		if code, _ := ifNone(a, path, bot, `"other", `+tag); code != 304 {
			t.Errorf("%s: list of tags %d", path, code)
		}
		if code, _ := ifNone(a, path, bot, `"other"`); code != 200 {
			t.Errorf("%s: other tag %d", path, code)
		}
	}
}

func TestPanelReadETagMovesWithRightsAndDefaultArtwork(t *testing.T) {
	a := etagApp(t)
	id := makePanel(t, a, userKey(t, a, "Ana"), etagPanel)
	cy, bot := userKey(t, a, "Cy"), botKey(t, a)
	path := "/api/v1/panels/" + id
	_, tag := ifNone(a, path, bot, "")

	apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":1,"art":"icon:camera"}`)
	code, art := ifNone(a, path, bot, tag)
	if code != 200 || art == tag {
		t.Fatalf("after Default artwork change: %d, ETag %q was %q", code, art, tag)
	}

	cfg := a.r().cfg
	cfg.APIClients[0].Rights = config.Rights{Sources: &[]int{1}}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	code, rights := ifNone(a, path, bot, art)
	if code != 200 || rights == art {
		t.Fatalf("after Rights change: %d, ETag %q was %q", code, rights, art)
	}
}

// An editor's read lists the instances with their uptime, which is measured to now. Only a change in what the
// page shows moves the tag: time alone does not, a lost instance does.
func TestPanelReadAnswers304ToAnEditorWhileTimePasses(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, etagPanel)
	if code := keepalive(a, id, ana, "Desk"); code != 200 {
		t.Fatalf("keepalive: %d", code)
	}
	done, _ := a.mon.openKeyless(id, "192.0.2.7", "Lobby", false, func() {})
	defer done()
	path := "/api/v1/panels/" + id
	_, tag := ifNone(a, path, ana, "")
	clk.advance(5 * time.Second)
	if code, again := ifNone(a, path, ana, tag); code != 304 || again != tag {
		t.Fatalf("5 s later: %d, ETag %q was %q", code, again, tag)
	}
	clk.advance(10 * time.Second)
	keepalive(a, id, ana, "Desk")
	if code, _ := ifNone(a, path, ana, tag); code != 304 {
		t.Fatalf("after a keepalive: %d, want 304", code)
	}
	clk.advance(2 * time.Minute)
	if code, _ := ifNone(a, path, ana, tag); code != 200 {
		t.Fatalf("after the instance went offline: %d, want 200", code)
	}
}

// A panel page opened by a key or as Public has no socket hello, so the read carries the take confirm's settings.
func TestPanelReadCarriesTheConfirmSettings(t *testing.T) {
	a := etagApp(t)
	id := makePanel(t, a, userKey(t, a, "Ana"), etagPanel)
	cfg := a.r().cfg
	cfg.ConfirmLockoutMs, cfg.ConfirmEnter = 1500, true
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := readPanel(t, a, botKey(t, a), id)["confirm"].(map[string]any)
	if got["lockoutMs"] != 1500.0 || got["enter"] != true {
		t.Fatalf("confirm %v, want lockoutMs 1500 and enter true", got)
	}
}

// The tally body is built once per change for every caller that is not hidden. A route, a level setting and a
// router change each move the tag, and a hidden caller still gets its own filtered body.
// The names read is built once per change for callers that are not hidden, so each input must move the tag.
func TestNamesETagMovesWithNamesOffsetAndRouter(t *testing.T) {
	a := etagApp(t)
	bot := botKey(t, a)
	const path = "/api/v1/names"
	_, tag := ifNone(a, path, bot, "")
	if code, again := ifNone(a, path, bot, tag); code != 304 || again != tag {
		t.Fatalf("unchanged: %d, ETag %q was %q", code, again, tag)
	}

	a.mu.Lock()
	a.applyNames(&swp08.NameBlock{Dest: true, First: 4, Names: []string{"MON NEW"}})
	a.mu.Unlock()
	code, named := ifNone(a, path, bot, tag)
	if code != 200 || named == tag {
		t.Fatalf("after a name: %d, ETag %q was %q", code, named, tag)
	}
	if !strings.Contains(apiDo(a, "GET", path, bot, nil).Body.String(), `{"n":4,"name":"MON NEW"}`) {
		t.Fatal("the new name is not in the body")
	}

	cfg := a.r().cfg
	cfg.NumberOffset = 1 - cfg.NumberOffset
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	code, offset := ifNone(a, path, bot, named)
	if code != 200 || offset == named {
		t.Fatalf("after a number offset: %d, ETag %q was %q", code, offset, named)
	}

	cfg = a.r().cfg
	cfg.APIClients[0].Rights = config.Rights{Visibility: config.VisHidden, Dsts: &[]int{1}}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if body := apiDo(a, "GET", path, bot, nil).Body.String(); strings.Contains(body, "MON NEW") {
		t.Fatalf("a hidden caller got a destination outside its Rights: %s", body)
	}

	cfg = a.r().cfg
	cfg.APIClients[0].Rights = config.Rights{}
	cfg.Router = "192.0.2.1:8910"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if code, moved := ifNone(a, path, bot, offset); code != 200 || moved == offset {
		t.Fatalf("after a router change: %d, ETag %q was %q", code, moved, offset)
	}
}

func TestTallyETagMovesWithRouteLevelsAndRouter(t *testing.T) {
	a := etagApp(t)
	bot := botKey(t, a)
	const path = "/api/v1/tally"
	_, tag := ifNone(a, path, bot, "")
	if code, again := ifNone(a, path, bot, tag); code != 304 || again != tag {
		t.Fatalf("unchanged: %d, ETag %q was %q", code, again, tag)
	}

	r := a.r()
	a.mu.Lock()
	a.applyTally(r, swp08.Crosspoint{Matrix: r.cfg.Matrix, Level: r.levels[0].Wire, Dst: 4, Src: 9, Cmd: swp08.CmdExtConnected})
	a.mu.Unlock()
	code, routed := ifNone(a, path, bot, tag)
	if code != 200 || routed == tag {
		t.Fatalf("after a route: %d, ETag %q was %q", code, routed, tag)
	}
	if !strings.Contains(apiDo(a, "GET", path, bot, nil).Body.String(), `{"dst":4,"level":1,"src":9}`) {
		t.Fatal("the new route is not in the body")
	}

	cfg := a.r().cfg
	cfg.LevelNumbering = "0"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	code, numbered := ifNone(a, path, bot, routed)
	if code != 200 || numbered == routed {
		t.Fatalf("after a level setting: %d, ETag %q was %q", code, numbered, routed)
	}

	cfg = a.r().cfg
	cfg.APIClients[0].Rights = config.Rights{Visibility: config.VisHidden, Dsts: &[]int{1}}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if body := apiDo(a, "GET", path, bot, nil).Body.String(); strings.Contains(body, `"dst":4`) {
		t.Fatalf("a hidden caller got a destination outside its Rights: %s", body)
	}

	cfg = a.r().cfg
	cfg.APIClients[0].Rights = config.Rights{}
	cfg.Router = "192.0.2.1:8910"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if code, moved := ifNone(a, path, bot, numbered); code != 200 || moved == numbered {
		t.Fatalf("after a router change: %d, ETag %q was %q", code, moved, numbered)
	}
}

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
)

// strm* helpers: the stream seam is a real test server, since a recorder cannot upgrade.

type strmConn struct {
	t  *testing.T
	c  *websocket.Conn
	in chan []byte
}

type strmMsg struct {
	T     string          `json:"t"`
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

func strmServer(t *testing.T, a *App) *httptest.Server {
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	t.Cleanup(srv.Close)
	return srv
}

func strmDialRaw(srv *httptest.Server, key, topics string) (*websocket.Conn, *http.Response, context.Context, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	hdr := http.Header{}
	if key != "" {
		hdr.Set("Authorization", "Bearer "+key)
	}
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/stream"
	if topics != "" {
		url += "?topics=" + topics
	}
	c, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	return c, resp, ctx, cancel, err
}

// strmDial reads in a goroutine: a read cut short by a context closes the connection, so
// waiting for silence has to happen on a channel.
func strmDial(t *testing.T, srv *httptest.Server, key, topics string) *strmConn {
	t.Helper()
	c, _, ctx, cancel, err := strmDialRaw(srv, key, topics)
	if err != nil {
		cancel()
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.CloseNow(); cancel() })
	s := &strmConn{t, c, make(chan []byte, 64)}
	go func() {
		for {
			_, b, err := c.Read(ctx)
			if err != nil {
				close(s.in)
				return
			}
			if !strings.Contains(string(b), `"ping"`) {
				s.in <- b
			}
		}
	}()
	return s
}

func (s *strmConn) next() strmMsg {
	s.t.Helper()
	select {
	case b, ok := <-s.in:
		if !ok {
			s.t.Fatal("stream closed")
		}
		var m strmMsg
		if err := json.Unmarshal(b, &m); err != nil {
			s.t.Fatalf("bad frame %q", b)
		}
		return m
	case <-time.After(5 * time.Second):
		s.t.Fatal("no frame")
	}
	return strmMsg{}
}

// quiet fails if a frame other than a heartbeat arrives within d.
func (s *strmConn) quiet(d time.Duration) {
	s.t.Helper()
	select {
	case b := <-s.in:
		s.t.Fatalf("unexpected frame %s", b)
	case <-time.After(d):
	}
}

func strmWantType(t *testing.T, m strmMsg, typ, topic string) {
	t.Helper()
	if m.T != typ || m.Topic != topic {
		t.Fatalf("got t=%q topic=%q data=%s, want %s %s", m.T, m.Topic, m.Data, typ, topic)
	}
}

type strmNames struct {
	Sources      []apiName `json:"sources"`
	Destinations []apiName `json:"destinations"`
}

type strmTally struct {
	Routes []apiRouteRow `json:"routes"`
}

func TestStreamSnapshotThenFilteredDeltas(t *testing.T) {
	a, key := viewApp(t, config.VisHidden)
	s := strmDial(t, strmServer(t, a), key, "names,tally,status")

	m := s.next()
	if m.T != "hello" {
		t.Fatalf("first frame %+v", m)
	}
	var hello struct {
		V      int      `json:"v"`
		Topics []string `json:"topics"`
	}
	json.Unmarshal(m.Data, &hello)
	if hello.V != 1 || len(hello.Topics) != 3 {
		t.Errorf("hello %s", m.Data)
	}

	m = s.next()
	strmWantType(t, m, "snapshot", "names")
	var n strmNames
	json.Unmarshal(m.Data, &n)
	if len(n.Sources) != 1 || n.Sources[0].Name != "CAM 1" || len(n.Destinations) != 1 {
		t.Errorf("names snapshot %s", m.Data)
	}
	m = s.next()
	strmWantType(t, m, "snapshot", "tally")
	var ty strmTally
	json.Unmarshal(m.Data, &ty)
	if len(ty.Routes) != 1 || ty.Routes[0] != (apiRouteRow{2, 1, 1}) {
		t.Errorf("tally snapshot %s", m.Data)
	}
	strmWantType(t, s.next(), "snapshot", "status")

	// Destination 7 is hidden, so its change never arrives. Destination 2 gets a hidden
	// source: that row is left out, as the REST read leaves it out.
	a.mu.Lock()
	a.broadcast(deltaMsg{T: "tally", U: []routeChange{{7, 1, 1}, {2, 1, 5}, {2, 2, 1}}})
	a.mu.Unlock()
	m = s.next()
	strmWantType(t, m, "tally", "")
	json.Unmarshal(m.Data, &ty)
	if len(ty.Routes) != 1 || ty.Routes[0] != (apiRouteRow{2, 2, 1}) {
		t.Errorf("tally delta %s", m.Data)
	}

	a.mu.Lock()
	a.router.srcNames[6] = "ANOTHER SECRET"
	a.broadcast(a.namesMsg())
	a.mu.Unlock()
	m = s.next()
	strmWantType(t, m, "names", "")
	if strings.Contains(string(m.Data), "SECRET") {
		t.Errorf("names leak %s", m.Data)
	}

	a.mu.Lock()
	a.broadcast(statusMsg{T: "status", Router: "down", Addr: "r:1"})
	a.mu.Unlock()
	m = s.next()
	strmWantType(t, m, "status", "")
	if !strings.Contains(string(m.Data), `"router":"down"`) {
		t.Errorf("status %s", m.Data)
	}
}

func TestStreamSendsOnlyChosenTopics(t *testing.T) {
	a, key := viewApp(t, "")
	s := strmDial(t, strmServer(t, a), key, "status")
	s.next() // hello
	strmWantType(t, s.next(), "snapshot", "status")
	a.mu.Lock()
	a.broadcast(deltaMsg{T: "tally", U: []routeChange{{2, 1, 1}}})
	a.broadcast(a.namesMsg())
	a.mu.Unlock()
	s.quiet(300 * time.Millisecond)
}

func TestStreamHistoryEntriesAndLateResults(t *testing.T) {
	a, key := viewApp(t, config.VisHidden)
	srv := strmServer(t, a)
	hist := strmDial(t, srv, key, "history")
	results := strmDial(t, srv, key, "results")
	hist.next()
	results.next()

	secret := Entry{Src: 5, Dst: 7, Levels: []int{1}, OK: true}
	mine := Entry{Src: 1, SrcName: "CAM 1", Dst: 2, Levels: []int{1}, Prev: [][2]int{{1, 1}}, OK: false, Failed: []int{1}}
	a.record(&secret)
	a.record(&mine)

	m := hist.next()
	strmWantType(t, m, "entry", "")
	var e Entry
	json.Unmarshal(m.Data, &e)
	if e.ID != mine.ID || e.Dst != 2 {
		t.Errorf("entry %s: the hidden entry came first or ids differ", m.Data)
	}
	results.quiet(200 * time.Millisecond) // new entries are not results

	// A Late Result is an update to an entry already sent.
	late := mine
	late.Failed, late.Late, late.OK = nil, []int{1}, true
	a.mu.Lock()
	a.broadcast(recentMsg{T: "recent", E: &late, Update: true})
	a.broadcast(recentMsg{T: "recent", E: &Entry{ID: secret.ID, Src: 5, Dst: 7, Levels: []int{1}}, Update: true})
	a.mu.Unlock()
	for name, s := range map[string]*strmConn{"history": hist, "results": results} {
		m := s.next()
		strmWantType(t, m, "entryUpdate", "")
		var u Entry
		json.Unmarshal(m.Data, &u)
		if u.ID != mine.ID || !u.OK || len(u.Late) != 1 {
			t.Errorf("%s update %s", name, m.Data)
		}
		s.quiet(150 * time.Millisecond) // the hidden entry's update never arrives
	}

	a.mu.Lock()
	a.broadcast(simpleMsg{T: "historyCleared"})
	a.mu.Unlock()
	strmWantType(t, hist.next(), "historyCleared", "")
}

func TestStreamRefusals(t *testing.T) {
	a, _ := viewApp(t, "")
	srv := strmServer(t, a)
	nb := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "NamesBot"}, "N1", time.Time{})
	key := botKey(t, a)
	cases := []struct {
		name, key, topics string
		status            int
	}{
		{"no topics", key, "", 400},
		{"unknown topic", key, "names,weather", 400},
		{"topic outside scopes", nb, "names,tally", 403},
		{"results needs history", nb, "results", 403},
		{"no key, guest off", "", "names", 401},
		{"bad key", "sode_nope", "names", 401},
	}
	for _, c := range cases {
		_, resp, _, cancel, err := strmDialRaw(srv, c.key, c.topics)
		cancel()
		if err == nil {
			t.Errorf("%s: connected", c.name)
			continue
		}
		if resp == nil || resp.StatusCode != c.status {
			t.Errorf("%s: %v, want %d", c.name, resp, c.status)
		}
	}

	off, _ := viewApp(t, "")
	cfg := off.r().cfg
	cfg.API.Enabled = false
	off.ApplyConfig(cfg)
	_, resp, _, cancel, err := strmDialRaw(strmServer(t, off), botKey(t, off), "names")
	cancel()
	if err == nil || resp == nil || resp.StatusCode != 503 {
		t.Errorf("API off: %v %v", resp, err)
	}
}

func TestStreamConnectAndDisconnectAreLogged(t *testing.T) {
	a, key := viewApp(t, "")
	defer startLog(t, a)()
	c, _, ctx, cancel, err := strmDialRaw(strmServer(t, a), key, "names,status")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	_, _, _ = c.Read(ctx) // hello
	rows := waitLogged(t, a, 1)
	e := rows[len(rows)-1]
	if e.Kind != kindAPI || e.By != "PanelsBot (Panel 43)" || e.Why != "streamConnect" || !strings.Contains(e.Note, "names") {
		t.Fatalf("connect row %+v", e)
	}
	c.Close(websocket.StatusNormalClosure, "")
	rows = waitLogged(t, a, 2)
	e = rows[len(rows)-1]
	if e.Why != "streamDisconnect" || e.By != "PanelsBot (Panel 43)" || strings.Contains(e.Note+e.By, key) {
		t.Fatalf("disconnect row %+v", e)
	}
}

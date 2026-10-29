package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
)

// /log pages over a real WebSocket, from a persisted, rotated log: the
// newest 100, then Load older until the start, each reply echoing its
// request's before.
func TestSentPagesOverWebSocket(t *testing.T) {
	cfg := config.Defaults()
	cfg.Normalize()
	hist, _ := OpenHistory("")
	sl, err := OpenSentLog(filepath.Join(t.TempDir(), "sent.jsonl"), 200_000)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1500; i++ {
		sl.Append(&Sent{Kind: "keepalive", Dst: -1, Src: -1, Wire: -1, Result: "ack", Hex: strings.Repeat("AB ", 20)})
	}
	a, err := New(cfg, "", hist, sl, false)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 24)
	next := func() sentPageMsg {
		for {
			_, b, err := c.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var m sentPageMsg
			if json.Unmarshal(b, &m) == nil && m.T == "sentPage" {
				return m
			}
		}
	}
	before, got := int64(-1), 0
	var prev int64 = -1
	for i := 0; i < 100; i++ {
		b, _ := json.Marshal(map[string]any{"t": "sent", "before": before, "n": 100})
		if err := c.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
		m := next()
		if m.Before != before {
			t.Fatalf("reply for before %d echoes %d", before, m.Before)
		}
		if i == 0 && len(m.Entries) != 100 {
			t.Fatalf("first page has %d entries", len(m.Entries))
		}
		for _, e := range m.Entries {
			if prev >= 0 && e.Seq != prev-1 {
				t.Fatalf("seq %d after %d", e.Seq, prev)
			}
			prev = e.Seq
		}
		got += len(m.Entries)
		if !m.More {
			break
		}
		before = m.Entries[len(m.Entries)-1].Seq
	}
	if int64(got) != sl.Total() {
		t.Fatalf("paged %d of %d", got, sl.Total())
	}
}

// An uptime request is answered with the echoed id and the range as used:
// a start too long ago is cut to what is kept, so the slots the server
// counted match the start the page draws from.
func TestUptimeOverWebSocket(t *testing.T) {
	cfg := config.Defaults()
	cfg.Normalize()
	hist, _ := OpenHistory("")
	a, err := New(cfg, "", hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	a.up.Note(Sent{Result: "nak", Tries: 3})
	a.up.State(true)
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	b, _ := json.Marshal(map[string]any{"t": "uptime", "id": 7, "from": 1, "slot": 1})
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var m uptimeMsg
		if json.Unmarshal(b, &m) != nil || m.T != "uptimeResult" {
			continue
		}
		if m.ID != 7 || m.Slot != 60_000 {
			t.Fatalf("reply = %+v", m)
		}
		if oldest := time.Now().Add(-uptimeKeep).UnixMilli(); m.From < oldest-1000 {
			t.Fatalf("from %d was not cut to what is kept", m.From)
		}
		if m.Start != "" || len(m.Trans) != 2 || m.Trans[1].S != upUp {
			t.Fatalf("trans = %+v start=%q", m.Trans, m.Start)
		}
		if len(m.Counts) != 1 || m.Counts[0].Bad != 1 {
			t.Fatalf("counts = %+v", m.Counts)
		}
		return
	}
}

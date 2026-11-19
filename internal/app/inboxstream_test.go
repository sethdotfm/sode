package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/sethdotfm/sode/internal/config"
)

type wsUser struct {
	t   *testing.T
	ctx context.Context
	ws  *websocket.Conn
	buf [][]byte // read and not matched yet: a push and an answer can come in either order
}

func dialUser(t *testing.T, ctx context.Context, url string) *wsUser {
	t.Helper()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(url, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.CloseNow() })
	ws.SetReadLimit(1 << 24)
	return &wsUser{t: t, ctx: ctx, ws: ws}
}

func (w *wsUser) send(m map[string]any) {
	b, _ := json.Marshal(m)
	if err := w.ws.Write(w.ctx, websocket.MessageText, b); err != nil {
		w.t.Fatal(err)
	}
}

// until returns the first message, kept or new, with type typ that ok accepts.
func (w *wsUser) until(typ string, ok func([]byte) bool) []byte {
	w.t.Helper()
	match := func(b []byte) bool {
		var h struct{ T string }
		return json.Unmarshal(b, &h) == nil && h.T == typ && ok(b)
	}
	for i, b := range w.buf {
		if match(b) {
			w.buf = append(w.buf[:i:i], w.buf[i+1:]...)
			return b
		}
	}
	for {
		_, b, err := w.ws.Read(w.ctx)
		if err != nil {
			w.t.Fatalf("no %s: %v", typ, err)
		}
		if match(b) {
			return b
		}
		w.buf = append(w.buf, b)
	}
}

func (w *wsUser) inbox(ok func(inboxMsg) bool) inboxMsg {
	w.t.Helper()
	var m inboxMsg
	w.until("inbox", func(b []byte) bool { m = inboxMsg{}; return json.Unmarshal(b, &m) == nil && ok(m) })
	return m
}

func (w *wsUser) signIn(user, pin string) {
	w.t.Helper()
	w.send(map[string]any{"t": "userSignIn", "id": 1, "user": user, "pin": pin})
	w.until("userSignInResult", func(b []byte) bool {
		var r userResult
		if json.Unmarshal(b, &r) != nil || !r.OK {
			w.t.Fatalf("sign-in as %s: %s", user, b)
		}
		return true
	})
}

// The Inbox count on every page comes from the `inbox` push. A user who was offline at the send gets the
// count at sign-in. Their other pages follow each answer live. An answer never asks for a toast.
func TestInboxStreamCount(t *testing.T) {
	a := userApp(t, false)
	setProfile(t, a, "Ana", config.Profile{Presets: []config.Preset{mePreset("Show", 1, 2)}})
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ana := dialUser(t, ctx, srv.URL)
	ana.signIn("Ana", "1234")
	id := profileOf(t, a, "Ana").Presets[0].ID
	ana.send(map[string]any{"t": "inboxSend", "id": 7, "inbox": map[string]any{"to": []string{"Bo"}, "presets": []any{map[string]any{"scope": "user", "id": id}}}})
	ana.until("inboxResult", func(b []byte) bool {
		var r inboxResult
		if json.Unmarshal(b, &r) != nil || r.ID != 7 || !r.OK {
			t.Fatalf("send: %s", b)
		}
		return true
	})
	ana.inbox(func(m inboxMsg) bool { return len(m.Sent) == 1 && m.Sent[0].State == cardWaiting })

	// Bo was offline: the count comes with the sign-in, on each page he signs in on (Main page and Log).
	main, log := dialUser(t, ctx, srv.URL), dialUser(t, ctx, srv.URL)
	main.signIn("Bo", "567890")
	log.signIn("Bo", "567890")
	got := main.inbox(func(m inboxMsg) bool { return len(m.Cards) == 1 })
	if got.Cards[0].From != "Ana" || got.Cards[0].Count != 1 {
		t.Fatalf("Bo's card: %+v", got.Cards[0])
	}
	log.inbox(func(m inboxMsg) bool { return len(m.Cards) == 1 })

	main.send(map[string]any{"t": "inboxDecline", "id": 8, "inbox": map[string]any{"card": got.Cards[0].ID}})
	for _, w := range []*wsUser{main, log} {
		if m := w.inbox(func(m inboxMsg) bool { return len(m.Cards) == 0 }); len(m.New) != 0 {
			t.Fatalf("an answer asked for a toast: %+v", m)
		}
	}
	ana.inbox(func(m inboxMsg) bool { return len(m.Sent) == 1 && m.Sent[0].State == cardDeclined })
}

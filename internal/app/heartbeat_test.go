package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// A connection gets a {"t":"ping"} message every wsHeartbeat, because the
// browser can't see WebSocket ping frames (#142).
func TestWSHeartbeat(t *testing.T) {
	old := wsHeartbeat
	wsHeartbeat = 50 * time.Millisecond
	defer func() { wsHeartbeat = old }()

	a := newTestApp(t)
	srv := httptest.NewServer(a.Handler(nil))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("no ping before the deadline: %v", err)
		}
		var m struct {
			T string `json:"t"`
		}
		if json.Unmarshal(data, &m) == nil && m.T == "ping" {
			return
		}
	}
}

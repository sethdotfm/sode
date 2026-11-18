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

// A preset saved over the WebSocket is stored as {router, n, name} references with a hidden id.
// A rename keeps the id, and firing it routes what the numbers name, as before references.
func TestPresetSavedAsReferencesFiresSameRoutes(t *testing.T) {
	a, f, key := takerApp(t, func(c *config.Config) { c.Presets = nil })
	uid := a.r().cfg.UID
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 24)
	save := func(id int, presets string) {
		t.Helper()
		msg := `{"t":"presets","id":` + string(rune('0'+id)) + `,"presets":` + presets + `}`
		if err := ws.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
			t.Fatal(err)
		}
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var m struct {
				T   string
				OK  bool
				ID  *int
				Err string
			}
			if json.Unmarshal(b, &m) == nil && m.T == "presetsResult" && m.ID != nil && *m.ID == id {
				if !m.OK {
					t.Fatalf("save %d refused: %s", id, b)
				}
				return
			}
		}
	}
	// This browser presets need the UID, so the hello carries it.
	for {
		_, b, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var h struct{ T, RouterUID string }
		if json.Unmarshal(b, &h) == nil && h.T == "hello" {
			if h.RouterUID != uid {
				t.Fatalf("hello routerUid %q, want %q", h.RouterUID, uid)
			}
			break
		}
	}
	if err := ws.Write(ctx, websocket.MessageText, []byte(`{"t":"userUnlock","user":"Ada","pin":"2468","id":9}`)); err != nil {
		t.Fatal(err)
	}
	// A browser that knows no names sends numbers only; the server fills router and names.
	save(1, `[{"name":"Show","items":[{"src":{"n":1},"dsts":[{"n":2}],"levels":[]}]}]`)
	p := a.r().cfg.Presets[0]
	want := config.PresetItem{Src: config.Ref{Router: uid, N: 1, Name: "CAM 1"}, Dsts: []config.Ref{{Router: uid, N: 2, Name: "MON 2"}}, Levels: []int{}}
	if p.ID == "" || p.Name != "Show" || len(p.Items) != 1 || !sameItem(p.Items[0], want) {
		t.Fatalf("stored %+v, want item %+v", p, want)
	}
	id := p.ID

	save(2, `[{"id":"`+id+`","name":"Evening","items":[{"src":{"n":1},"dsts":[{"n":2}],"levels":[]}]}]`)
	if got := a.r().cfg.Presets; len(got) != 1 || got[0].ID != id || got[0].Name != "Evening" {
		t.Fatalf("rename: %+v, want id %s", got, id)
	}

	f.seed(2, 3, 0)
	rec := apiPost(a, "/api/v1/presets/everyone/Evening", key, ``, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rs := apiBody(t, rec)["results"].([]any)
	r0 := rs[0].(map[string]any)
	if len(rs) != 1 || r0["src"] != float64(1) || r0["dst"] != float64(2) || r0["ok"] != true {
		t.Fatalf("fired %s", rec.Body.String())
	}
}

func sameItem(a, b config.PresetItem) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

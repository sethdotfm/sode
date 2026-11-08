package app

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
)

// The table is the dispatch: every rule has a handler, and the read loop has no switch
// on the message type left to fall out of step with it.
func TestPolicyCoversEveryMessage(t *testing.T) {
	for _, file := range []string{"http.go", "users.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sw, ok := n.(*ast.SwitchStmt); ok {
				if sel, ok := sw.Tag.(*ast.SelectorExpr); ok && sel.Sel.Name == "T" {
					t.Errorf("%s switches on the message type; add a rule to policy instead", file)
				}
			}
			return true
		})
	}
	if len(policy) == 0 {
		t.Fatal("empty policy table")
	}
	for m, r := range policy {
		if r.handle == nil {
			t.Errorf("policy lists %q without a handler", m)
		}
	}
	for _, m := range []string{"hi", "batch", "unlock", "userUnlock", "configImportBegin", "users", "usersList", "apiSettings"} {
		if _, ok := policy[m]; !ok {
			t.Errorf("policy lost %q", m)
		}
	}
}

// Refusals are answered the same way everywhere: the lock state or the PIN
// prompt, and a failed result for a message that has one. An unlisted type
// is ignored.
func TestRefusalReplies(t *testing.T) {
	a := userApp(t, true)
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 24)
	send := func(m map[string]any) {
		b, _ := json.Marshal(m)
		if err := ws.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
	}
	until := func(typ string) map[string]any {
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				t.Fatalf("waiting for %s: %v", typ, err)
			}
			var m map[string]any
			json.Unmarshal(b, &m)
			if m["t"] == typ {
				return m
			}
		}
	}
	send(map[string]any{"t": "hi", "page": "main"})
	until("lockState")

	send(map[string]any{"t": "nonsense"}) // dropped, never dispatched
	send(map[string]any{"t": "batch", "items": []map[string]any{{"id": 7, "src": 1, "dst": 1, "levels": []int{1}}}})
	if m := until("takeResult"); m["ok"] == true || m["id"] != float64(7) || m["err"] == "" {
		t.Fatalf("refused batch: %v", m)
	}
	send(map[string]any{"t": "presets", "id": 3})
	if m := until("presetsResult"); m["ok"] == true || m["locked"] != true || m["id"] != float64(3) {
		t.Fatalf("refused salvos: %v", m)
	}
	send(map[string]any{"t": "users", "id": 5, "usersReq": map[string]any{}})
	if m := until("usersResult"); m["locked"] != true || m["id"] != float64(5) {
		t.Fatalf("refused users: %v", m)
	}

	send(map[string]any{"t": "unlock", "pin": "2468", "id": 1})
	if m := until("unlockResult"); m["ok"] != true {
		t.Fatalf("unlock: %v", m)
	}
	send(map[string]any{"t": "presets", "id": 6, "presets": []any{}})
	if m := until("presetsResult"); m["ok"] != true {
		t.Fatalf("salvos with the Advanced Settings PIN: %v", m)
	}
	send(map[string]any{"t": "batch", "items": []map[string]any{{"id": 8}}})
	if m := until("takeResult"); m["ok"] == true {
		t.Fatalf("a take got through on the Advanced Settings PIN: %v", m)
	}
}

func TestClearHistoryNeedsBoth(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	r := policy["clearHistory"]
	if err := a.admit(c, r); !errors.Is(err, errPageLocked) {
		t.Fatalf("nothing unlocked: %v", err)
	}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if err := a.admit(c, r); !errors.Is(err, errLocked) {
		t.Fatalf("user only: %v", err)
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.admit(c, r); err != nil {
		t.Fatalf("both: %v", err)
	}
}

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An open panel polls GET /api/v1/panels/{id} and compares rev: every stored change moves it.
func TestPanelRevMovesOnEveryStoredChange(t *testing.T) {
	a := panelsApp(t)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	id := makePanel(t, a, ana, `{"name":"Wall","share":{"mode":"everyone"},"rev":99}`)
	rev := func(key string) float64 {
		t.Helper()
		r, _ := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, key, nil))["rev"].(float64)
		return r
	}
	first := rev(ana)
	if first != 1 || rev(bo) != first {
		t.Fatalf("a new panel starts at 1 and the use-only reader sees it too: %v", first)
	}
	if rev(ana) != first {
		t.Fatal("a read must not move the revision")
	}

	apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall 2","share":{"mode":"everyone"},"rev":1000}`)
	second := rev(bo)
	if second != first+1 {
		t.Fatalf("a save moves rev by one and ignores the body's rev: %v then %v", first, second)
	}
	if p, _ := a.panels.get(id); p.Extra["rev"] != nil {
		t.Fatalf("rev must not land in Extra: %v", p.Extra)
	}

	// Any store replace moves it, so a Push can use the same path as a save.
	p, _ := a.panels.get(id)
	if _, err := a.panels.replace(p); err != nil {
		t.Fatal(err)
	}
	if rev(ana) != second+1 {
		t.Fatalf("replace did not move rev: %v", rev(ana))
	}

	// A copy is a new panel with its own count.
	c := apiBody(t, apiSend(a, "POST", "/api/v1/panels/"+id+"/copy", bo, ``))
	if r, _ := c["rev"].(float64); r < 1 || r > 2 { // artwork copies may add one
		t.Fatalf("copy rev %v", c["rev"])
	}
}

func TestPanelRevSurvivesReload(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall 2"}`)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(a.cfgPath), "panels", id+".yaml"))
	if err != nil || !strings.Contains(string(raw), "rev: 2") {
		t.Fatalf("file: %v %s", err, raw)
	}
	if got, ok := loadPanels(a.cfgPath).get(id); !ok || got.Rev != 2 {
		t.Fatalf("reload %+v", got)
	}
}

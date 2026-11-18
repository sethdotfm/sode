package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The take flow, `settings.mode`: the server refuses a value the pages do not know.

func TestPanelModeRefusesUnknownValue(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	for _, m := range []string{"take", "takeOnly", "instant", ""} {
		id := makePanel(t, a, ana, `{"name":"Wall","settings":{"mode":"`+m+`"}}`)
		s, _ := readPanel(t, a, ana, id)["settings"].(map[string]any)
		if s["mode"] != m {
			t.Errorf("saved %q, read %v", m, s["mode"])
		}
	}
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	for _, m := range []string{`"stage"`, `"Take"`, `1`, `true`} {
		rec := apiSend(a, "POST", "/api/v1/panels", ana, `{"name":"Bad","settings":{"mode":`+m+`}}`)
		if !strings.Contains(rec.Body.String(), "settings.mode") {
			t.Errorf("mode %s: the error does not name the setting: %s", m, rec.Body.String())
		}
		wantErr(t, rec, 400, "bad_request")
		wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","settings":{"mode":`+m+`}}`), 400, "bad_request")
	}
}

// A file on disk with a known value loads as written, and nothing rewrites it.
func TestPanelModeOldFilesLoadUnchanged(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	dir := filepath.Join(filepath.Dir(a.cfgPath), "panels")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"p-0000aa01": "take", "p-0000aa02": "takeOnly", "p-0000aa03": "instant"}
	raw := map[string][]byte{}
	for id, m := range files {
		raw[id] = []byte("id: " + id + "\nname: Old " + m + "\nowner: Ana\nacts: session\nrows: 1\ncols: 1\ncells: []\nsettings:\n    mode: " + m + "\n")
		if err := os.WriteFile(filepath.Join(dir, id+".yaml"), raw[id], 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.panels = loadPanels(a.cfgPath)
	for id, m := range files {
		s, _ := readPanel(t, a, ana, id)["settings"].(map[string]any)
		if s["mode"] != m {
			t.Errorf("%s reads mode %v, want %s", id, s["mode"], m)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, id+".yaml")); !bytes.Equal(got, raw[id]) {
			t.Errorf("%s was rewritten:\n%s", id, got)
		}
	}
}

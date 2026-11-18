package app

import (
	"encoding/json"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// Pages compare an Item link's install with the hello's installUid.
func TestHelloCarriesInstallUID(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	b := a.hello(helloPage{})
	a.mu.Unlock()
	var m map[string]any
	json.Unmarshal(b, &m)
	if uid := a.r().cfg.InstallUID; uid == "" || m["installUid"] != uid {
		t.Fatalf("hello installUid %v, config %q", m["installUid"], uid)
	}
}

// A config file from another install brings its settings, never its install UID: links made here must keep working.
func TestConfigImportKeepsInstallUID(t *testing.T) {
	a := userApp(t, false)
	uid := a.r().cfg.InstallUID
	text, _ := exportOf(t, a, false)
	file, _ := config.Parse([]byte(text))
	file.InstallUID = "0000000000000000"
	b, _ := yaml.Marshal(file)
	if err := a.ImportConfig(adminClient(t, a), string(b)); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg.InstallUID; got != uid {
		t.Fatalf("install UID %q after import, want %q", got, uid)
	}
}

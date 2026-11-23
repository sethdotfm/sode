package app

import (
	"encoding/json"
	"testing"
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

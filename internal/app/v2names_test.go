package app

import "testing"

// TestNoV1WireNames: v2 keeps no v1 wire names (docs/persisted-names.md).
func TestNoV1WireNames(t *testing.T) {
	for old, now := range map[string]string{"batch": "take", "refreshNames": "rereadNames", "sent": "log", "userUnlock": "userSignIn"} {
		if _, ok := policy[old]; ok {
			t.Errorf("policy still has %q", old)
		}
		if _, ok := policy[now]; !ok {
			t.Errorf("policy has no %q", now)
		}
	}
	a := newTestApp(t)
	a.mu.Lock()
	h := asMap(t, a.hello(helloPage{}))
	a.mu.Unlock()
	for _, old := range []string{"recent", "protect"} {
		if _, ok := h[old]; ok {
			t.Errorf("hello still has %q", old)
		}
	}
	for _, now := range []string{"history", "protected"} {
		if _, ok := h[now]; !ok {
			t.Errorf("hello has no %q", now)
		}
	}
}

// v2 has no settings secret: no message enters or drops it, and the hello says nothing of it.
func TestNoSettingsSecretOnTheWire(t *testing.T) {
	for _, gone := range []string{"unlock", "lock"} {
		if _, ok := policy[gone]; ok {
			t.Errorf("policy still has %q", gone)
		}
	}
	a := newTestApp(t)
	a.mu.Lock()
	h := asMap(t, a.hello(helloPage{}))
	a.mu.Unlock()
	if _, ok := h["pinSet"]; ok {
		t.Error("hello still has pinSet")
	}
}

package app

import "testing"

// docs/persisted-names.md
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
	// protected and protectBy moved into each element of routers.
	for _, old := range []string{"recent", "protect", "protected", "protectBy"} {
		if _, ok := h[old]; ok {
			t.Errorf("hello still has %q", old)
		}
	}
	if _, ok := h["history"]; !ok {
		t.Error("hello has no history")
	}
	if _, ok := firstHelloRouter(t, h)["protected"]; !ok {
		t.Error("the hello's first router has no protected")
	}
}

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

package config

import (
	"path/filepath"
	"testing"
)

func TestClearRequireHTTPS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := Defaults()
	c.Normalize()
	c.TrustedProxies = []string{"10.0.0.2"}
	c.RequireHTTPS = true
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	was, err := ClearRequireHTTPS(path)
	if err != nil || !was {
		t.Fatalf("was %v, err %v", was, err)
	}
	got, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.RequireHTTPS || len(got.TrustedProxies) != 1 {
		t.Fatalf("requireHttps %v, proxies %v", got.RequireHTTPS, got.TrustedProxies)
	}
	if was, err := ClearRequireHTTPS(path); err != nil || was {
		t.Fatalf("second run: was %v, err %v", was, err)
	}
}

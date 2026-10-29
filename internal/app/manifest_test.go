package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestManifestFollowsBranding(t *testing.T) {
	a := newTestApp(t)
	rec := get(t, a, "/manifest.webmanifest")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/manifest+json") {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var m struct {
		ShortName string `json:"short_name"`
		Icons     []struct{ Src string }
	}
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m.ShortName != "Sode" || len(m.Icons) != 3 || m.Icons[0].Src != iconURL(192, "") {
		t.Fatalf("default manifest %+v", m)
	}

	cfg := a.r().cfg
	cfg.BrandText, cfg.BrandBorder = "Studio B", "teal"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(get(t, a, "/manifest.webmanifest").Body.Bytes(), &m)
	if m.ShortName != "Studio B" || m.Icons[1].Src != iconURL(512, "teal") {
		t.Fatalf("branded manifest %+v", m)
	}
}

package app

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func zipDir(t *testing.T, dir string) []byte {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return zipOf(t, files)
}

// The format 1 fixture is written by hand, so this catches a loader change that would misread a released file.
func TestExportFormat1Fixture(t *testing.T) {
	a := newTestApp(t)
	b, done, err := a.stageImport(bytes.NewReader(zipDir(t, "testdata/export-v1")))
	defer done()
	if err != nil {
		t.Fatal(err)
	}
	const uid = "5f3a9c1e7b2d4a60"
	if m := b.Manifest; m.Format != 1 || m.Install != "0123456789abcdef0123456789abcdef" || len(m.Routers) != 1 || m.Routers[0].Prefix != "A" {
		t.Errorf("manifest: %+v", m)
	}
	slices.Sort(b.Unknown)
	// A router of format 1 had its own statusLabel. An import no longer reads it: the status label is one setting.
	if want := []string{"routers[0].statusLabel", "settings.confirm.confirmSound", "settings.weather", "users[0].shoeSize"}; !slices.Equal(b.Unknown, want) {
		t.Errorf("unknown keys %v, want %v", b.Unknown, want)
	}
	if len(b.Users) != 2 || b.Users[0].Extra != nil || b.Users[0].Pin == "" || b.Users[0].Role != config.RoleAdministrator {
		t.Errorf("users: %+v", b.Users)
	}
	if pins := b.Users[0].Profile.PalettePins; !slices.Equal(pins, []string{"page:log", "preset:user:pr-me1"}) {
		t.Errorf("Me pins: %q", pins)
	}
	me := b.Users[0].Profile.Presets[0].Items[0]
	if me.Src.Router != uid || me.Src.N != 1 || me.Dsts[0].Router != uid {
		t.Errorf("Me preset: %+v", me)
	}
	if d := *b.Users[1].Rights.Dsts; d[0].Router != uid || d[0].Name != "MON 1" {
		t.Errorf("rights: %+v", d)
	}
	if p := b.Presets[0].Items[0]; p.Src.Router != uid || p.Dsts[0].Router != uid || !slices.Equal(p.Levels, []int{1, 2}) {
		t.Errorf("preset: %+v", p)
	}
	if len(b.FilterSets) != 2 || b.FilterSets[0].Router != "" || b.FilterSets[0].Src != "CAM" || b.FilterSets[1].Router != uid {
		t.Errorf("filter sets: %+v", b.FilterSets)
	}
	cells := b.Panels.Panels[0].Pages[0].Cells
	if cells[0]["router"] != uid || cells[1]["router"] != uid || b.Panels.Panels[0].Owner != "u-0001" {
		t.Errorf("panel cells: %+v", cells)
	}
	if cp := b.Panels.Checkpoints["p-0a1b2c3d"]; len(cp) != 1 || cp[0].Layout.Pages[0].Cells[0]["router"] != uid {
		t.Errorf("checkpoints: %+v", b.Panels.Checkpoints)
	}
	if d := b.Artwork.Defaults[0]; d.Router != uid || *d.N != 0 || b.Artwork.Icons[0].ID != "00ff00ff" || b.Art["art/icons/00ff00ff.svg"] == "" {
		t.Errorf("artwork: %+v %+v", b.Artwork, b.Art)
	}
	if c := b.APIClients[0]; c.Owner != "u-0001" || !slices.Equal(c.Scopes, []string{"names", "tally"}) {
		t.Errorf("API client: %+v", c)
	}
	if r := b.Routers[0]; r.UID != uid || r.Address != "10.1.2.3:8910" || r.NumberOffset != 1 || r.NameLength != 8 {
		t.Errorf("router: %+v", r)
	}
	if s := b.Settings; s.BrandText != "Truck B" || s.ConfirmLockoutMs != 700 || s.HiddenDsts != "9" || s.HiddenDstsRouter != uid || !slices.Equal(b.Sections, []string{"branding", "confirm", "history"}) {
		t.Errorf("settings: %+v %v", s, b.Sections)
	}
	if n := b.Names[uid]; len(n.Sources) != 2 || n.Destinations[0].Name != "MON 1" {
		t.Errorf("names: %+v", n)
	}
	if bm := b.Bookmarks; len(bm) != 2 || bm[0].Key != "preset:global:pr-0001" || bm[1].URL != "https://example.com/run" || !bm[1].NewTab {
		t.Errorf("bookmarks: %+v", bm)
	}
	if pins, _ := b.Browser["palettePins"].([]any); len(pins) != 2 || pins[0] != "page:log" {
		t.Errorf("browser pins: %v", b.Browser["palettePins"])
	}
	if bl, _ := b.Browser["bookmarks"].([]any); b.Browser["theme"] != "light" || len(bl) != 1 {
		t.Errorf("browser: %v", b.Browser)
	}
}

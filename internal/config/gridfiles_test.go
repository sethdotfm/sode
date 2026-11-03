package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGridFileRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "grids")
	g := okGrid()
	normalizeGrid(&g)
	if err := SaveGrid(dir, g); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "studio-a.yml")); err != nil {
		t.Fatal(err)
	}
	got, err := LoadGridFiles(dir)
	if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0], g) {
		t.Fatalf("%v %+v", err, got)
	}
	// No stray temp files.
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("%v", ents)
	}
	if err := DeleteGrid(dir, "studio-a"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteGrid(dir, "studio-a"); err != nil {
		t.Fatalf("deleting twice: %v", err)
	}
	if got, err := LoadGridFiles(dir); err != nil || len(got) != 0 {
		t.Fatalf("%v %v", err, got)
	}
	// A missing directory is no panels.
	if got, err := LoadGridFiles(filepath.Join(dir, "nope")); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("%v %v", err, got)
	}
}

func TestGridFileTraversal(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "grids")
	for _, link := range []string{"../x", "a/b", "..", ".", "", "A", "a.b", "a\x00b", `a\b`, "/etc/passwd"} {
		g := okGrid()
		g.Link = link
		if err := SaveGrid(dir, g); err == nil {
			t.Errorf("SaveGrid accepted %q", link)
		}
		if err := DeleteGrid(dir, link); err == nil {
			t.Errorf("DeleteGrid accepted %q", link)
		}
	}
	if ents, _ := os.ReadDir(root); len(ents) != 0 {
		t.Fatalf("something was written: %v", ents)
	}
}

func TestGridFilesBad(t *testing.T) {
	good := "link: ok\nname: Ok\ncols: 2\nrows: 2\nconfirm: default\ncells: []\n"
	for name, c := range map[string]struct{ file, body string }{
		"yaml":          {"a.yml", "link: [\n"},
		"unknown field": {"a.yml", "link: a\ncols: 1\nrows: 1\nbogus: 1\n"},
		"invalid":       {"a.yml", "link: a\ncols: 0\nrows: 1\n"},
		"wrong name":    {"b.yml", "link: a\ncols: 1\nrows: 1\n"},
		"bad file name": {"A_b.yml", good},
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "ok.yml"), []byte(good), 0o644)
		os.WriteFile(filepath.Join(dir, c.file), []byte(c.body), 0o644)
		_, err := LoadGridFiles(dir)
		if err == nil || !strings.Contains(err.Error(), c.file) {
			t.Errorf("%s: want an error naming %s, got %v", name, c.file, err)
		}
	}
	// Other files and hidden ones are left alone.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ok.yml"), []byte(good), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(dir, ".sode-123.tmp"), []byte("{"), 0o644)
	os.WriteFile(filepath.Join(dir, ".hidden.yml"), []byte("{"), 0o644)
	if got, err := LoadGridFiles(dir); err != nil || len(got) != 1 {
		t.Fatalf("%v %v", err, got)
	}
}

func TestSyncGrids(t *testing.T) {
	dir := t.TempDir()
	a, b := okGrid(), okGrid()
	b.Link = "studio-b"
	normalizeGrid(&a)
	normalizeGrid(&b)
	if err := SyncGrids(dir, nil, []Grid{a, b}); err != nil {
		t.Fatal(err)
	}
	st := func(link string) os.FileInfo {
		fi, _ := os.Stat(filepath.Join(dir, link+".yml"))
		return fi
	}
	if st("studio-a") == nil || st("studio-b") == nil {
		t.Fatal("files missing")
	}
	before := st("studio-a").ModTime()
	a2 := a
	a2.Name = "Renamed"
	b2 := b
	b2.Link = "studio-c" // a rename: the old file goes
	if err := SyncGrids(dir, []Grid{a, b}, []Grid{a2, b2}); err != nil {
		t.Fatal(err)
	}
	if st("studio-b") != nil || st("studio-c") == nil {
		t.Fatal("rename left the wrong files")
	}
	got, _ := LoadGridFiles(dir)
	if len(got) != 2 || got[0].Name != "Renamed" {
		t.Fatalf("%+v", got)
	}
	_ = before
	// Unchanged panels aren't rewritten.
	fi := st("studio-a")
	if err := SyncGrids(dir, []Grid{a2, b2}, []Grid{a2, b2}); err != nil || !st("studio-a").ModTime().Equal(fi.ModTime()) {
		t.Fatalf("rewrote an unchanged panel: %v", err)
	}
}

func TestLoadGridsMigration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	old := "router: 1.2.3.4:8910\ngridConfirm: false\ngrids:\n" +
		"  - {link: one, name: One, cols: 2, rows: 1, confirm: default, levels: [], cells: [{x: 0, y: 0, w: 1, h: 1, kind: dst, ref: '3'}]}\n" +
		"  - {link: two, name: Two, cols: 1, rows: 1, confirm: on, levels: [1], cells: []}\n"
	os.WriteFile(path, []byte(old), 0o644)
	// A file that already exists wins over the config's copy.
	os.MkdirAll(filepath.Join(root, "grids"), 0o755)
	os.WriteFile(filepath.Join(root, "grids", "two.yml"), []byte("link: two\nname: Mine\ncols: 3\nrows: 3\nconfirm: off\ncells: []\n"), 0o644)

	c, _, err := Load(path) // KnownFields must accept the old list
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadGrids(path); err != nil {
		t.Fatal(err)
	}
	if len(c.Grids) != 2 || c.Grids[0].Link != "one" || c.Grids[1].Name != "Mine" || c.LegacyGrids != nil {
		t.Fatalf("%+v", c.Grids)
	}
	if _, err := os.Stat(filepath.Join(root, "grids", "one.yml")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "grids") || !strings.Contains(string(b), "gridConfirm: false") || !strings.Contains(string(b), "1.2.3.4:8910") {
		t.Fatalf("config.yaml not rewritten as it should:\n%s", b)
	}
	// Running again changes nothing.
	c2, _, err := Load(path)
	if err != nil || len(c2.LegacyGrids) != 0 {
		t.Fatalf("%v %+v", err, c2.LegacyGrids)
	}
	if err := c2.LoadGrids(path); err != nil || len(c2.Grids) != 2 {
		t.Fatalf("%v %+v", err, c2.Grids)
	}
	// Without a config file nothing is touched.
	var mem Config
	mem.Grids = []Grid{okGrid()}
	if err := mem.LoadGrids(""); err != nil || len(mem.Grids) != 1 {
		t.Fatal(err)
	}
}

func TestLoadGridsBadLegacyOrFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	os.WriteFile(path, []byte("grids:\n  - {link: Bad_Link, cols: 1, rows: 1}\n"), 0o644)
	c, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadGrids(path); err == nil {
		t.Fatal("invalid legacy grid accepted")
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "Bad_Link") {
		t.Fatal("config rewritten despite the error")
	}
	// A damaged file stops startup and names itself.
	os.WriteFile(path, []byte("router: 1.2.3.4:8910\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "grids"), 0o755)
	os.WriteFile(filepath.Join(root, "grids", "broken.yml"), []byte("link: [\n"), 0o644)
	c, _, _ = Load(path)
	if err := c.LoadGrids(path); err == nil || !strings.Contains(err.Error(), "broken.yml") {
		t.Fatalf("%v", err)
	}
}

// Grids are runtime only: config.yaml never gets them.
func TestGridsNotInConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := gridCfg(okGrid())
	c.GridConfirm = false
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "studio-a") || strings.Contains(string(b), "grids:") {
		t.Fatalf("grids in config.yaml:\n%s", b)
	}
	got, _, err := Load(path)
	if err != nil || got.GridConfirm || len(got.Grids) != 0 {
		t.Fatalf("%v %+v", err, got.Grids)
	}
	// Clone shares no cells.
	cl := c.Clone()
	cl.Grids[0].Cells[0].Ref = "99"
	cl.Grids[0].Levels = append(cl.Grids[0].Levels, 1)
	if c.Grids[0].Cells[0].Ref != "4" || len(c.Grids[0].Levels) != 0 {
		t.Fatal("clone shares grid slices")
	}
}

// Caps: one panel at most 32 KiB, all of them 48 KiB, and a gridSave or a
// grids message with those (worst case: every character escaped to six
// bytes) stays under the server's 64 KiB read limit.
func TestGridCaps(t *testing.T) {
	cell := func(i int) GridCell {
		return GridCell{X: i % 24, Y: i / 24, W: 1, H: 1, Kind: "salvo", Ref: strings.Repeat("<", 64), Label: strings.Repeat("<", 24)}
	}
	build := func(link string, n int) Grid {
		g := Grid{Link: link, Name: strings.Repeat("<", 40), Cols: 24, Rows: 24, Confirm: "default"}
		for i := 0; i < n; i++ {
			g.Cells = append(g.Cells, cell(i))
		}
		normalizeGrid(&g)
		return g
	}
	// The biggest single panel that passes.
	n := 0
	for ; n < 24*24; n++ {
		if len(ValidateGrid(build("a", n+1))) > 0 {
			break
		}
	}
	if n == 0 || n >= 24*24 {
		t.Fatalf("per-panel cap not hit sensibly: %d cells", n)
	}
	g := build("a", n)
	b, _ := json.Marshal(g)
	if len(b) > maxGridJSON || len(b) < maxGridJSON-1024 {
		t.Fatalf("largest panel is %d bytes, cap %d", len(b), maxGridJSON)
	}
	msg, _ := json.Marshal(map[string]any{"t": "gridSave", "grid": g, "was": strings.Repeat("a", 40), "id": 1<<31 - 1})
	if len(msg) >= 64<<10 {
		t.Fatalf("gridSave %d bytes", len(msg))
	}
	// All together: more panels than fit are refused.
	c := Defaults()
	c.Normalize()
	for i := 0; i < maxGrids; i++ {
		c.Grids = append(c.Grids, build("g"+strings.Repeat("a", i), n))
	}
	if c.Validate() == nil {
		t.Fatal("grids over the total cap accepted")
	}
	// The most that pass: grids message under the limit.
	c.Grids = c.Grids[:1]
	for len(c.Grids) < maxGrids {
		next := append(append([]Grid{}, c.Grids...), build("g"+strings.Repeat("a", len(c.Grids)), n))
		d := c
		d.Grids = next
		if d.Validate() != nil {
			break
		}
		c = d
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	all, _ := json.Marshal(map[string]any{"t": "grids", "grids": c.Grids, "gridConfirm": true})
	if len(all) >= 64<<10 || len(all) > maxGridsJSON+1024 {
		t.Fatalf("grids message %d bytes", len(all))
	}
	if maxGridJSON+1024 >= 64<<10 || maxGridsJSON+1024 >= 64<<10 {
		t.Fatal("caps leave no room for the envelope")
	}
}

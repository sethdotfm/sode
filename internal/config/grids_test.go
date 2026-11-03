package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func gridCfg(g Grid) Config {
	c := Defaults()
	c.Grids = []Grid{g}
	c.Normalize()
	return c
}

func okGrid() Grid {
	return Grid{Link: "studio-a", Name: "Studio A", Cols: 4, Rows: 3, Confirm: "default", Cells: []GridCell{
		{X: 0, Y: 0, W: 2, H: 1, Kind: "dst", Ref: "4", Label: "PGM"},
		{X: 2, Y: 0, W: 1, H: 1, Kind: "src", Ref: "12"},
		{X: 3, Y: 2, W: 1, H: 1, Kind: "salvo", Ref: "Opening"},
	}}
}

func TestGridValid(t *testing.T) {
	c := gridCfg(okGrid())
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if !Defaults().GridConfirm {
		t.Error("grids confirm by default")
	}
	// Spans default to 1, links are lowercased, the name falls back to the link.
	g := okGrid()
	g.Link, g.Name = " Studio-B ", ""
	g.Cells[1].W, g.Cells[1].H = 0, 0
	c = gridCfg(g)
	if err := c.Validate(); err != nil || c.Grids[0].Link != "studio-b" || c.Grids[0].Name != "studio-b" || c.Grids[0].Cells[1].W != 1 {
		t.Fatalf("%v %+v", err, c.Grids[0])
	}
}

func TestGridInvalid(t *testing.T) {
	for name, mut := range map[string]func(*Grid){
		"empty link":      func(g *Grid) { g.Link = "" },
		"bad chars link":  func(g *Grid) { g.Link = "a_b c" },
		"slash link":      func(g *Grid) { g.Link = "a/b" },
		"double dash":     func(g *Grid) { g.Link = "a--b" },
		"leading dash":    func(g *Grid) { g.Link = "-a" },
		"reserved link":   func(g *Grid) { g.Link = "ws" },
		"long link":       func(g *Grid) { g.Link = strings.Repeat("a", 41) },
		"zero cols":       func(g *Grid) { g.Cols = 0 },
		"huge rows":       func(g *Grid) { g.Rows = 25 },
		"bad confirm":     func(g *Grid) { g.Confirm = "maybe" },
		"bad level":       func(g *Grid) { g.Levels = []int{0} },
		"level past":      func(g *Grid) { g.Levels = []int{18} },
		"overlap":         func(g *Grid) { g.Cells[1].X = 1 },
		"inside a span":   func(g *Grid) { g.Cells = append(g.Cells, GridCell{X: 1, Y: 0, W: 1, H: 1, Kind: "src", Ref: "1"}) },
		"x out of bounds": func(g *Grid) { g.Cells[1].X = 4 },
		"span past edge":  func(g *Grid) { g.Cells[0].W = 5 },
		"y out of bounds": func(g *Grid) { g.Cells[2].Y = 3 },
		"negative":        func(g *Grid) { g.Cells[1].X = -1 },
		"bad kind":        func(g *Grid) { g.Cells[1].Kind = "level" },
		"src not number":  func(g *Grid) { g.Cells[1].Ref = "cam" },
		"src number 007":  func(g *Grid) { g.Cells[1].Ref = "007" },
		"dst negative":    func(g *Grid) { g.Cells[0].Ref = "-1" },
		"empty salvo":     func(g *Grid) { g.Cells[2].Ref = "" },
		"long label":      func(g *Grid) { g.Cells[0].Label = strings.Repeat("x", 25) },
	} {
		g := okGrid()
		mut(&g)
		if err := gridCfg(g).Validate(); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	c := gridCfg(okGrid())
	c.Grids = append(c.Grids, okGrid())
	if c.Validate() == nil {
		t.Error("duplicate links accepted")
	}
	c = Defaults()
	for i := 0; i < 21; i++ {
		g := okGrid()
		g.Link = "g" + strings.Repeat("a", i)
		c.Grids = append(c.Grids, g)
	}
	c.Normalize()
	if c.Validate() == nil {
		t.Error("21 grids accepted")
	}
}

func TestGridSaveLoadClone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := gridCfg(okGrid())
	c.GridConfirm = false
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.GridConfirm || len(got.Grids) != 1 || got.Grids[0].Cells[0].Label != "PGM" || got.Grids[0].Cells[0].W != 2 {
		t.Fatalf("%+v", got)
	}
	// A file without grids loads with the confirm default on.
	empty := filepath.Join(t.TempDir(), "c.yaml")
	if err := Save(empty, Defaults()); err != nil {
		t.Fatal(err)
	}
	if g, _, err := Load(empty); err != nil || !g.GridConfirm || len(g.Grids) != 0 {
		t.Fatalf("%v %+v", err, g.Grids)
	}
	// Clone shares no cells.
	cl := c.Clone()
	cl.Grids[0].Cells[0].Ref = "99"
	cl.Grids[0].Levels = append(cl.Grids[0].Levels, 1)
	if c.Grids[0].Cells[0].Ref != "4" || len(c.Grids[0].Levels) != 0 {
		t.Fatal("clone shares grid slices")
	}
	if _, ok := c.GridByLink("studio-a"); !ok {
		t.Fatal("GridByLink")
	}
}

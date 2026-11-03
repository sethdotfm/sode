package config

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Grid is a customizable panel of source, destination and salvo buttons,
// served at /grid/<Link> and edited at /settings/grids (#150). Each
// is one file, grids/<Link>.yml beside the config file (gridfiles.go).
type Grid struct {
	// Link is the URL slug: lowercase letters, digits and single dashes.
	Link string `yaml:"link" json:"link"`
	Name string `yaml:"name" json:"name"`
	Cols int    `yaml:"cols" json:"cols"`
	Rows int    `yaml:"rows" json:"rows"`
	// Confirm is "default" (follow Config.GridConfirm), "on" or "off".
	Confirm string `yaml:"confirm" json:"confirm"`
	// Levels are UI levels (1..N, whatever the numbering shows) a take on
	// this panel uses; empty = the default levels.
	Levels []int      `yaml:"levels" json:"levels"`
	Cells  []GridCell `yaml:"cells" json:"cells"`
}

// GridCell is one button. X, Y are zero-based from the top left; W, H are
// spans, so a joined cell covers several grid squares.
type GridCell struct {
	X    int    `yaml:"x" json:"x"`
	Y    int    `yaml:"y" json:"y"`
	W    int    `yaml:"w" json:"w"`
	H    int    `yaml:"h" json:"h"`
	Kind string `yaml:"kind" json:"kind"` // src, dst, salvo, confirm or "" (a spacer)
	// Ref is the source or destination number (decimal, as the router has
	// it, not the displayed number) or a salvo's name; confirm and spacer
	// cells have none.
	Ref   string `yaml:"ref" json:"ref"`
	Label string `yaml:"label,omitempty" json:"label"`
}

// The caps keep every message that carries panels under the server's 64 KiB
// WebSocket read limit (and so the whole list too, which is what a browser
// holds): one panel as JSON is at most maxGridJSON, all of them at most
// maxGridsJSON, and a gridSave carries one.
const (
	maxGridJSON  = 32 << 10
	maxGridsJSON = 48 << 10
	maxGrids     = 20
	maxGridSize  = 24
)

// Cell kinds. "" is a spacer: an empty cell that keeps a layout in place.
const (
	CellSrc     = "src"
	CellDst     = "dst"
	CellSalvo   = "salvo"
	CellConfirm = "confirm"
)

var linkRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidLink reports whether s is a usable grid link. It is also the file's
// name (<link>.yml), so nothing else may pass: no slash, dot or space.
func ValidLink(s string) bool {
	return len(s) <= 40 && linkRe.MatchString(s)
}

func cloneGrids(in []Grid) []Grid {
	out := slices.Clone(in)
	for i := range out {
		out[i].Levels = slices.Clone(out[i].Levels)
		out[i].Cells = slices.Clone(out[i].Cells)
	}
	return out
}

func (c *Config) normalizeGrids() {
	if c.Grids == nil {
		c.Grids = []Grid{}
	}
	for i := range c.Grids {
		normalizeGrid(&c.Grids[i])
	}
}

func normalizeGrid(g *Grid) {
	g.Link = strings.ToLower(strings.TrimSpace(g.Link))
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		g.Name = g.Link
	}
	if g.Confirm == "" {
		g.Confirm = "default"
	}
	if g.Levels == nil {
		g.Levels = []int{}
	}
	slices.Sort(g.Levels)
	g.Levels = slices.Compact(g.Levels)
	if g.Cells == nil {
		g.Cells = []GridCell{}
	}
	for j := range g.Cells {
		k := &g.Cells[j]
		k.Kind = strings.TrimSpace(k.Kind)
		k.Ref = strings.TrimSpace(k.Ref)
		k.Label = strings.TrimSpace(k.Label)
		if k.W == 0 {
			k.W = 1
		}
		if k.H == 0 {
			k.H = 1
		}
	}
}

// validateGrids checks the panels as a set: how many, how big together, and
// that no two share a link; each is checked by ValidateGrid.
func (c Config) validateGrids() []error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if len(c.Grids) > maxGrids {
		bad("at most %d grids", maxGrids)
	}
	if b, err := json.Marshal(c.Grids); err == nil && len(b) > maxGridsJSON {
		bad("the grids are too big together (%d KiB of at most %d KiB): remove some buttons or grids", len(b)>>10, maxGridsJSON>>10)
	}
	links := map[string]bool{}
	for _, g := range c.Grids {
		if links[g.Link] {
			bad("two grids have the link %q", g.Link)
		}
		links[g.Link] = true
		errs = append(errs, ValidateGrid(g)...)
	}
	return errs
}

// ValidateGrid returns what is wrong with one normalized panel.
func ValidateGrid(g Grid) []error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	who := fmt.Sprintf("grid %q", g.Link)
	if !ValidLink(g.Link) {
		bad("grid link %q must be lowercase letters, digits and dashes (at most 40)", g.Link)
	}
	if b, err := json.Marshal(g); err == nil && len(b) > maxGridJSON {
		bad("%s is too big (%d KiB of at most %d KiB): remove some buttons", who, len(b)>>10, maxGridJSON>>10)
	}
	if n := len([]rune(g.Name)); n > 40 {
		bad("%s: name must be at most 40 characters", who)
	}
	if g.Cols < 1 || g.Cols > maxGridSize || g.Rows < 1 || g.Rows > maxGridSize {
		bad("%s: columns and rows must be 1 to %d", who, maxGridSize)
		return errs
	}
	switch g.Confirm {
	case "default", "on", "off":
	default:
		bad("%s: confirm must be default, on or off", who)
	}
	// Levels past the router's count are kept but unused (as with
	// LevelColors), so lowering the level count in Settings still saves.
	for _, l := range g.Levels {
		if l < 1 {
			bad("%s: level %d is not a level", who, l)
		}
	}
	taken := make([]bool, g.Cols*g.Rows)
	confirms := 0
	for _, k := range g.Cells {
		at := fmt.Sprintf("%s: cell at column %d, row %d", who, k.X+1, k.Y+1)
		switch k.Kind {
		case CellSrc, CellDst:
			if n, err := strconv.Atoi(k.Ref); err != nil || n < 0 || n > 65535 || strconv.Itoa(n) != k.Ref {
				bad("%s: %q is not a %s number", at, k.Ref, k.Kind)
			}
		case CellSalvo:
			if k.Ref == "" || len([]rune(k.Ref)) > 64 {
				bad("%s: a salvo cell needs the salvo's name", at)
			}
		case CellConfirm:
			if k.Ref != "" {
				bad("%s: a confirm cell has no reference", at)
			}
			if confirms++; confirms == 2 {
				bad("%s: a grid has at most one confirm cell", who)
			}
		case "":
			if k.Ref != "" {
				bad("%s: an empty cell has no reference", at)
			}
		default:
			bad("%s: kind must be src, dst, salvo, confirm or empty", at)
		}
		if len([]rune(k.Label)) > 24 {
			bad("%s: label must be at most 24 characters", at)
		}
		// Subtracting from the (checked, small) size cannot wrap, unlike X+W.
		if k.W < 1 || k.H < 1 || k.X < 0 || k.Y < 0 || k.X > g.Cols-k.W || k.Y > g.Rows-k.H {
			bad("%s is outside the %d x %d grid", at, g.Cols, g.Rows)
			continue
		}
		overlap := false
		for y := k.Y; y < k.Y+k.H; y++ {
			for x := k.X; x < k.X+k.W; x++ {
				if taken[y*g.Cols+x] {
					overlap = true
				}
				taken[y*g.Cols+x] = true
			}
		}
		if overlap {
			bad("%s overlaps another cell", at)
		}
	}
	return errs
}

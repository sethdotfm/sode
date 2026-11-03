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
// served at /grid/<Link> and edited at /settings/grids (#150).
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
	Kind string `yaml:"kind" json:"kind"` // src, dst or salvo
	// Ref is the source or destination number (decimal, as the router has
	// it, not the displayed number) or a salvo's name.
	Ref   string `yaml:"ref" json:"ref"`
	Label string `yaml:"label,omitempty" json:"label"`
}

// MaxGridsJSON is the most JSON the grids may take, so the grids message
// stays under the server's 64 KiB read limit.
const MaxGridsJSON = 48 << 10

const (
	maxGrids    = 20
	maxGridSize = 24
)

var linkRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reservedLinks can't be panel links: they would clash with files and the
// WebSocket served under /grid/.
var reservedLinks = []string{"ws", "vendor", "icon", "manifest"}

// ValidLink reports whether s is a usable grid link.
func ValidLink(s string) bool {
	return len(s) <= 40 && linkRe.MatchString(s) && !slices.Contains(reservedLinks, s)
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
		g := &c.Grids[i]
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
}

func (c Config) validateGrids() []error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if len(c.Grids) > maxGrids {
		bad("at most %d grids", maxGrids)
	}
	// The editor sends every grid in one WebSocket message (read limit
	// 64 KiB); what is saved must fit it with room for the envelope.
	if b, err := json.Marshal(c.Grids); err == nil && len(b) > MaxGridsJSON {
		bad("the grids are too big (%d KiB of at most %d KiB): remove some buttons or grids", len(b)>>10, MaxGridsJSON>>10)
	}
	links := map[string]bool{}
	for _, g := range c.Grids {
		who := fmt.Sprintf("grid %q", g.Link)
		switch {
		case !ValidLink(g.Link):
			bad("grid link %q must be lowercase letters, digits and dashes (at most 40), and not %s", g.Link, strings.Join(reservedLinks, ", "))
		case links[g.Link]:
			bad("two grids have the link %q", g.Link)
		}
		links[g.Link] = true
		if n := len([]rune(g.Name)); n > 40 {
			bad("%s: name must be at most 40 characters", who)
		}
		if g.Cols < 1 || g.Cols > maxGridSize || g.Rows < 1 || g.Rows > maxGridSize {
			bad("%s: columns and rows must be 1 to %d", who, maxGridSize)
			continue
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
		for _, k := range g.Cells {
			at := fmt.Sprintf("%s: cell at column %d, row %d", who, k.X+1, k.Y+1)
			switch k.Kind {
			case "src", "dst":
				if n, err := strconv.Atoi(k.Ref); err != nil || n < 0 || n > 65535 || strconv.Itoa(n) != k.Ref {
					bad("%s: %q is not a %s number", at, k.Ref, k.Kind)
				}
			case "salvo":
				if k.Ref == "" || len([]rune(k.Ref)) > 64 {
					bad("%s: a salvo cell needs the salvo's name", at)
				}
			default:
				bad("%s: kind must be src, dst or salvo", at)
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
	}
	return errs
}

// GridByLink returns the grid with that link.
func (c Config) GridByLink(link string) (Grid, bool) {
	for _, g := range c.Grids {
		if g.Link == link {
			return g, true
		}
	}
	return Grid{}, false
}

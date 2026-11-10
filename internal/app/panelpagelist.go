package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// A panel's Pages (ADR 0007, GLOSSARY.md Page). A Page's number is its place in the list; its id never changes,
// so a Go to Page cell names the id. Keep the limits in step with web/panelpages.js.

const (
	panelPageMax     = 16
	panelPageNameMax = 40
	panelPageCellMax = panelRowsMax * panelColsMax
)

var pageIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,16}$`)

// PanelPage is one Page. Enabled reads as true when a body or file leaves it out.
type PanelPage struct {
	ID      string           `yaml:"id" json:"id"`
	Name    string           `yaml:"name,omitempty" json:"name,omitempty"`
	Enabled bool             `yaml:"enabled" json:"enabled"`
	Cells   []map[string]any `yaml:"cells" json:"cells"`
}

type pagePlain PanelPage // no methods, so the hooks below do not recurse

func (g *PanelPage) UnmarshalJSON(b []byte) error {
	q := pagePlain{Enabled: true}
	if err := json.Unmarshal(b, &q); err != nil {
		return err
	}
	*g = PanelPage(q)
	return nil
}

func (g *PanelPage) UnmarshalYAML(n *yaml.Node) error {
	q := pagePlain{Enabled: true}
	if err := n.Decode(&q); err != nil {
		return err
	}
	*g = PanelPage(q)
	return nil
}

func newPageID() string { return newShortID("") }

// legacyPageID names the Page an old one-grid record becomes after the record's own id, so a restart before the first
// save gives it the same id. A record with no id yet gets a random one.
func legacyPageID(owner string) string {
	if _, rest, ok := strings.Cut(owner, "-"); ok && pageIDRe.MatchString(rest) {
		return rest
	}
	return newPageID()
}

// normalPages fills defaults and never fails; checkPages judges the result. Pages win over legacy cells, so a file an
// older Sode saved with both keeps its Pages. The list is a copy: pages may be shared with a stored panel.
func normalPages(pages []PanelPage, legacy []map[string]any, owner string) []PanelPage {
	if len(pages) == 0 {
		if legacy == nil {
			legacy = []map[string]any{}
		}
		return []PanelPage{{ID: legacyPageID(owner), Enabled: true, Cells: legacy}}
	}
	out := slices.Clone(pages)
	for i := range out {
		if out[i].ID == "" {
			out[i].ID = newPageID()
		}
		if out[i].Cells == nil {
			out[i].Cells = []map[string]any{}
		}
	}
	return out
}

// checkPages is run on every save: the count, ids, names and cells per Page. Each Page's cells are fitted to the grid,
// so a resize drops what no longer fits on every Page.
func checkPages(pages []PanelPage, rows, cols int) ([]PanelPage, error) {
	if len(pages) < 1 || len(pages) > panelPageMax {
		return nil, fmt.Errorf("a panel has 1 to %d Pages", panelPageMax)
	}
	out := slices.Clone(pages)
	seen := map[string]bool{}
	for i, g := range out {
		switch {
		case !pageIDRe.MatchString(g.ID):
			return nil, errors.New("a Page id is 1 to 16 letters, digits, dashes and underscores")
		case seen[g.ID]:
			return nil, fmt.Errorf("two Pages have the id %q", g.ID)
		case len(g.Cells) > panelPageCellMax:
			return nil, errors.New("too many cells for the grid")
		}
		seen[g.ID] = true
		name := strings.TrimSpace(g.Name)
		if utf8.RuneCountInString(name) > panelPageNameMax {
			return nil, fmt.Errorf("a Page name is at most %d characters", panelPageNameMax)
		}
		out[i].Name = cleanText(name, panelPageNameMax)
		out[i].Cells = fitCells(g.Cells, rows, cols)
	}
	if err := checkPageTargets(out); err != nil {
		return nil, err
	}
	if err := checkShowParts(out); err != nil {
		return nil, err
	}
	return out, nil
}

// cellShowParts is what a Page Name or Info cell's `show` may name; keep in step with SHOW_PARTS in web/pagecells.js.
// No `show` field is the kind's default, and an empty list shows nothing.
var cellShowParts = map[string][]string{"pageName": {"number", "name"}, "info": {"panel", "number", "page", "instance"}}

func checkShowParts(pages []PanelPage) error {
	for c := range allCells(pages) {
		kind, _ := c["kind"].(string)
		allowed, ok := cellShowParts[kind]
		v, has := c["show"]
		if !ok || !has {
			continue
		}
		list, isList := v.([]any)
		if !isList {
			return fmt.Errorf("a %s cell's show is a list of %s", kind, strings.Join(allowed, ", "))
		}
		seen := map[string]bool{}
		for _, x := range list {
			s, _ := x.(string)
			if !slices.Contains(allowed, s) || seen[s] {
				return fmt.Errorf("a %s cell's show is a list of %s, each at most once", kind, strings.Join(allowed, ", "))
			}
			seen[s] = true
		}
	}
	return nil
}

// checkPageTargets judges each Go to Page cell's `page`, a Page id. A target that is missing is stored as is and read as
// unassigned (web/panelnav.js goToState), so only a malformed one is refused; "" or no field is unassigned.
func checkPageTargets(pages []PanelPage) error {
	for c := range allCells(pages) {
		v, has := c["page"]
		if c["kind"] != "goToPage" || !has {
			continue
		}
		if s, ok := v.(string); !ok || s != "" && !pageIDRe.MatchString(s) {
			return errors.New("a Go to Page cell's page is a Page id: 1 to 16 letters, digits, dashes and underscores")
		}
	}
	return nil
}

// fitCells drops cells that start outside rows by cols and clips a span that only reaches outside, as resize in
// web/gridmodel.js does. A cell with no whole r and c is left as it is; the pages ignore it. Changed cells are copies.
func fitCells(cells []map[string]any, rows, cols int) []map[string]any {
	out := make([]map[string]any, 0, len(cells))
	for _, c := range cells {
		r, okR := cellInt(c["r"])
		k, okC := cellInt(c["c"])
		if !okR || !okC {
			out = append(out, c)
			continue
		}
		if r > rows || k > cols {
			continue
		}
		rs, okRS := cellInt(c["rs"])
		cs, okCS := cellInt(c["cs"])
		if okRS && r+rs-1 > rows || okCS && k+cs-1 > cols {
			c = maps.Clone(c)
			if okRS && r+rs-1 > rows {
				c["rs"] = rows - r + 1
			}
			if okCS && k+cs-1 > cols {
				c["cs"] = cols - k + 1
			}
		}
		out = append(out, c)
	}
	return out
}

// cellInt reads a whole number: float64 from JSON, int from YAML.
func cellInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case float64:
		if x == float64(int(x)) {
			return int(x), true
		}
	}
	return 0, false
}

// allCells walks every Page's cells, in order.
func allCells(pages []PanelPage) func(func(map[string]any) bool) {
	return func(yield func(map[string]any) bool) {
		for _, g := range pages {
			for _, c := range g.Cells {
				if !yield(c) {
					return
				}
			}
		}
	}
}

// pageCellCount is how many cells the Pages hold together; legacy is a record from before Pages.
func pageCellCount(pages []PanelPage, legacy []map[string]any) (cells, count int) {
	if len(pages) == 0 {
		return len(legacy), 1
	}
	for _, g := range pages {
		cells += len(g.Cells)
	}
	return cells, len(pages)
}

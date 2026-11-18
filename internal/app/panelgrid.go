package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Panel types (ADR 0011). A file with no type is a Buttons panel. Keep in step with web/gridpaneledges.js.
const (
	panelTypeButtons = "buttons"
	panelTypeGrid    = "grid"
)

const gridMembersAll = "all"

// GridEdge is a Grid panel's sources or destinations. Membership stays apart from per-item settings, so a
// pin or artwork never stops an "all" edge from following the router.
type GridEdge struct {
	Members GridMembers `yaml:"members" json:"members"`
	// Hide is only for an "all" edge: router numbers left out of it.
	Hide []int `yaml:"hide,omitempty" json:"hide,omitempty"`
	// Items is keyed by the router number in decimal. An entry for a number that is not a member stays, like artwork.
	Items map[string]GridItem `yaml:"items,omitempty" json:"items,omitempty"`
}

// GridItem is the settings of one source or destination on a Grid panel. Art, ArtB and HideText have the form of a
// cell's. HideText false is kept: it shows the name against a Default artwork that hides it.
type GridItem struct {
	Pinned   bool   `yaml:"pinned,omitempty" json:"pinned,omitempty"`
	Art      string `yaml:"art,omitempty" json:"art,omitempty"`
	ArtB     string `yaml:"artB,omitempty" json:"artB,omitempty"`
	HideText *bool  `yaml:"hideText,omitempty" json:"hideText,omitempty"`
}

// GridMembers is "all" (router order, following the router) or an explicit list of router numbers in panel order.
// A decoded list is never nil, so a nil List with All false means the members key was missing.
type GridMembers struct {
	All  bool
	List []int
}

var errGridMembers = errors.New(`members is "all" or a list of router numbers`)

func (m GridMembers) MarshalJSON() ([]byte, error) {
	if m.All {
		return json.Marshal(gridMembersAll)
	}
	if m.List == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(m.List)
}

func (m *GridMembers) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*m = GridMembers{All: s == gridMembersAll}
		if !m.All {
			return errGridMembers
		}
		return nil
	}
	if bytes.HasPrefix(bytes.TrimSpace(b), []byte("[")) {
		*m = GridMembers{List: []int{}}
		return json.Unmarshal(b, &m.List)
	}
	return errGridMembers
}

func (m GridMembers) MarshalYAML() (any, error) {
	if m.All {
		return gridMembersAll, nil
	}
	if m.List == nil {
		return []int{}, nil
	}
	return m.List, nil
}

func (m *GridMembers) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.Value == gridMembersAll {
		*m = GridMembers{All: true}
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		*m = GridMembers{List: []int{}}
		return n.Decode(&m.List)
	}
	return errGridMembers
}

// shows is whether router number n is a member that is not hidden.
func (e *GridEdge) shows(n int) bool {
	if e.Members.All {
		return !slices.Contains(e.Hide, n)
	}
	return slices.Contains(e.Members.List, n)
}

func allEdge() *GridEdge { return &GridEdge{Members: GridMembers{All: true}} }

// normalGrid fills a Grid panel's defaults. A Grid panel has no rows, columns or Pages, so a read
// carries an empty Pages list that a client can send back.
func (p *Panel) normalGrid() {
	if p.Sources == nil {
		p.Sources = allEdge()
	}
	if p.Destinations == nil {
		p.Destinations = allEdge()
	}
	p.Rows, p.Cols = 0, 0
	if p.Pages == nil {
		p.Pages = []PanelPage{}
	}
}

// checkGrid judges the type, the parts that belong to the other type, and each edge.
func checkGrid(p *Panel) error {
	switch p.Type {
	case panelTypeButtons:
		if p.Sources != nil || p.Destinations != nil {
			return errors.New("sources and destinations are for a Grid panel")
		}
		return nil
	case panelTypeGrid:
	default:
		return errors.New(`type is "buttons" or "grid"`)
	}
	if len(p.Pages) > 0 || len(p.LegacyCells) > 0 {
		return errors.New("a Grid panel has no cells and no Pages")
	}
	for _, e := range []struct {
		name string
		edge *GridEdge
	}{{"sources", p.Sources}, {"destinations", p.Destinations}} {
		if e.edge == nil {
			continue
		}
		if err := checkGridEdge(e.edge); err != nil {
			return fmt.Errorf("%s: %w", e.name, err)
		}
	}
	return nil
}

func checkGridEdge(e *GridEdge) error {
	if !e.Members.All && e.Members.List == nil {
		return errGridMembers
	}
	if !e.Members.All && len(e.Hide) > 0 {
		return errors.New(`hide is only for members "all"`)
	}
	if err := checkGridNumbers("members", e.Members.List); err != nil {
		return err
	}
	if err := checkGridNumbers("hide", e.Hide); err != nil {
		return err
	}
	for k := range e.Items {
		// One form for each number, so "7" and "007" never name one item twice.
		if n, err := strconv.Atoi(k); err != nil || n < 0 || n > dstNumberMax || strconv.Itoa(n) != k {
			return fmt.Errorf("items key %q is not a router number", k)
		}
	}
	return nil
}

func checkGridNumbers(what string, ns []int) error {
	seen := make(map[int]bool, len(ns))
	for _, n := range ns {
		if n < 0 || n > dstNumberMax {
			return fmt.Errorf("%s: %d is not a router number", what, n)
		}
		if seen[n] {
			return fmt.Errorf("%s: %d is there two times", what, n)
		}
		seen[n] = true
	}
	return nil
}

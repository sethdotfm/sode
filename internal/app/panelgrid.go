package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// Panel types (ADR 0011). A file with no type is a Buttons panel. Keep in step with web/gridpaneledges.js.
const (
	panelTypeButtons = "buttons"
	panelTypeGrid    = "grid"
)

const gridMembersAll = "all"

// GridEdge is a Grid panel's sources or destinations. Membership stays apart from per-item settings, so a
// pin or artwork never stops an "all" edge from following the router. Every entry is a router reference, which
// panelrefs.go fills.
type GridEdge struct {
	Members GridMembers `yaml:"members" json:"members"`
	// Hide is only for an "all" edge: items left out of it.
	Hide []config.Ref `yaml:"hide,omitempty" json:"hide,omitempty"`
	// Items holds one entry for each item with settings. An entry for an item that is not a member stays, like artwork.
	Items []GridItem `yaml:"items,omitempty" json:"items,omitempty"`
}

// GridItem is the settings of one source or destination on a Grid panel, with its router reference. Art, ArtB and
// HideText have the form of a cell's. HideText false is kept: it shows the name against a Default artwork that
// hides it. The reference fields are spelled out, since yaml.v3 drops config.Ref's inline map inside an inline struct.
type GridItem struct {
	Router   string `yaml:"router" json:"router"`
	N        int    `yaml:"n" json:"n"`
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`
	Pinned   bool   `yaml:"pinned,omitempty" json:"pinned,omitempty"`
	Art      string `yaml:"art,omitempty" json:"art,omitempty"`
	ArtB     string `yaml:"artB,omitempty" json:"artB,omitempty"`
	HideText *bool  `yaml:"hideText,omitempty" json:"hideText,omitempty"`
}

func (it GridItem) ref() config.Ref { return config.Ref{Router: it.Router, N: it.N, Name: it.Name} }

// GridMembers is "all" (router order, following the router) or an explicit list of references in panel order.
// A decoded list is never nil, so a nil List with All false means the members key was missing.
type GridMembers struct {
	All  bool
	List []config.Ref
}

var errGridMembers = errors.New(`members is "all" or a list of router references`)

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
		*m = GridMembers{List: []config.Ref{}}
		return json.Unmarshal(b, &m.List)
	}
	return errGridMembers
}

func (m GridMembers) MarshalYAML() (any, error) {
	if m.All {
		return gridMembersAll, nil
	}
	if m.List == nil {
		return []config.Ref{}, nil
	}
	return m.List, nil
}

func (m *GridMembers) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.Value == gridMembersAll {
		*m = GridMembers{All: true}
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		*m = GridMembers{List: []config.Ref{}}
		return n.Decode(&m.List)
	}
	return errGridMembers
}

// shows is whether item n of router uid is a member that is not hidden.
func (e *GridEdge) shows(uid string, n int) bool {
	is := func(r config.Ref) bool { return r.Router == uid && r.N == n }
	if e.Members.All {
		return !slices.ContainsFunc(e.Hide, is)
	}
	return slices.ContainsFunc(e.Members.List, is)
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
	if err := checkGridRefs("members", e.Members.List); err != nil {
		return err
	}
	if err := checkGridRefs("hide", e.Hide); err != nil {
		return err
	}
	refs := make([]config.Ref, len(e.Items))
	for i, it := range e.Items {
		refs[i] = it.ref()
	}
	return checkGridRefs("items", refs)
}

type refKey struct {
	router string
	n      int
}

func keyOf(r config.Ref) refKey { return refKey{r.Router, r.N} }

func checkGridRefs(what string, refs []config.Ref) error {
	seen := make(map[refKey]bool, len(refs))
	for _, r := range refs {
		if r.N < 0 || r.N > dstNumberMax {
			return fmt.Errorf("%s: %d is not a router number", what, r.N)
		}
		if seen[keyOf(r)] {
			return fmt.Errorf("%s: %d is there two times", what, r.N)
		}
		seen[keyOf(r)] = true
	}
	return nil
}

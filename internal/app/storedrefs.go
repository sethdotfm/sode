package app

import (
	"errors"

	"github.com/sethdotfm/sode/internal/config"
)

// Stored layouts (panels, templates, checkpoints, Checkpoint sets) never read a bare router number. A request can
// leave the router out while there is one router. refPages, refEdges and nameFixed fill it before the store writes.
// A file that leaves it out is skipped with a problem, not filled from the one router.
var errNoRouter = errors.New("a source or destination has no router")

func (l panelLayout) noRouterProblem() error {
	for _, g := range l.Pages {
		if cellsNoRouter(g.Cells) {
			return errNoRouter
		}
	}
	if cellsNoRouter(l.LegacyCells) {
		return errNoRouter
	}
	for _, e := range []*GridEdge{l.Sources, l.Destinations} {
		if e == nil {
			continue
		}
		for r := range edgeRefs(e) {
			if r.Router == "" {
				return errNoRouter
			}
		}
	}
	switch list := l.Settings[panelFixedKey].(type) {
	case nil:
	case []config.Ref:
		for _, f := range list {
			if f.Router == "" {
				return errNoRouter
			}
		}
	case []any:
		for _, x := range list {
			m, _ := x.(map[string]any)
			if r, _ := m["router"].(string); r == "" {
				return errNoRouter
			}
		}
	default:
		return errFixedDests
	}
	return nil
}

// A Source or Destination cell with no number is unassigned and needs no router.
func cellsNoRouter(cells []map[string]any) bool {
	for _, c := range cells {
		if k := c["kind"]; k != "source" && k != "destination" || c["n"] == nil {
			continue
		}
		if r, _ := c["router"].(string); r == "" {
			return true
		}
	}
	return false
}

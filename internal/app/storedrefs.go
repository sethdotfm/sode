package app

import (
	"errors"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

// Stored layouts (panels, templates, checkpoints, Checkpoint sets) never read a bare router number. A request can
// leave the router out while there is one router. refPages, refEdges and nameFixed fill it before the store writes.
// A file that leaves it out is skipped with a problem, not filled from the one router.
var errNoRouter = errors.New("a source or destination has no router")

func (l panelLayout) noRouterProblem() error {
	switch list := l.Settings[panelFixedKey].(type) {
	case nil, []config.Ref:
	case []any:
		if slices.ContainsFunc(list, func(x any) bool { m, _ := x.(map[string]any); r, _ := m["router"].(string); return r == "" }) {
			return errNoRouter
		}
	default:
		return errFixedDests
	}
	// A Source or Destination cell with no number is unassigned and needs no router.
	if slices.ContainsFunc(l.LegacyCells, func(c map[string]any) bool {
		r, ok := cellRef(c)
		return ok && cellKinds[c["kind"]] != "" && r.Router == ""
	}) {
		return errNoRouter
	}
	bare := false
	l.readRefs(func(_ refSlot, _ string, r config.Ref) { bare = bare || r.Router == "" })
	if bare {
		return errNoRouter
	}
	return nil
}

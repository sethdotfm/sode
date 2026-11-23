package app

import (
	"errors"
	"maps"
	"slices"
)

// The level subset of a panel, `settings.levels`, names its router: UI levels by router UID. A router the map does
// not name, and an empty list, mean every level. A request can send a plain list, which names the one router, and
// levelsByRouter fills it. A file must name it (noRouterProblem). The levels of a Source or Destination cell are
// levels of the cell's own `router`. A cell with levels and no source keeps its router too: refNames.pages fills the
// one router, and a template that drops the contents keeps it. Docs: docs/panels-design.md.
const panelLevelsKey = "levels"

var errPanelLevels = errors.New("settings.levels is a list of levels, or lists of levels by router UID")

// levelsByRouter puts a plain list, or the list of the blank UID, under uid and drops empty lists. It clones the map
// before a change, since a stored panel's settings are shared and never mutated.
func levelsByRouter(settings map[string]any, uid string) (map[string]any, error) {
	v, ok := settings[panelLevelsKey]
	if !ok {
		return settings, nil
	}
	by := map[string]any{}
	add := func(router string, l any) error {
		list, ok := l.([]any)
		if l != nil && !ok || slices.ContainsFunc(list, func(x any) bool { n, ok := cellInt(x); return !ok || n < 1 }) {
			return errPanelLevels
		}
		if router == "" {
			router = uid
		}
		if had, _ := by[router].([]any); len(had) > 0 {
			list = append(slices.Clone(had), list...)
		}
		if len(list) > 0 {
			by[router] = list
		}
		return nil
	}
	switch x := v.(type) {
	case nil:
	case []any:
		if err := add("", x); err != nil {
			return nil, err
		}
	case map[string]any:
		for _, router := range slices.Sorted(maps.Keys(x)) {
			if err := add(router, x[router]); err != nil {
				return nil, err
			}
		}
	default:
		return nil, errPanelLevels
	}
	settings = maps.Clone(settings)
	if len(by) == 0 {
		delete(settings, panelLevelsKey)
	} else {
		settings[panelLevelsKey] = by
	}
	return settings, nil
}

// levelsNoRouter: settings.levels is a plain list or has a list with no router UID.
func levelsNoRouter(settings map[string]any) bool {
	switch x := settings[panelLevelsKey].(type) {
	case nil:
		return false
	case map[string]any:
		_, blank := x[""]
		return blank
	}
	return true
}

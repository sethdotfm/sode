package app

import (
	"fmt"
	"maps"
	"math"
	"slices"
)

// Fixed destination: `settings.fixedDests`, the destinations a panel always patches to. Empty
// or absent means off. Docs: docs/panels-design.md, docs/api.md.

const (
	panelFixedKey = "fixedDests"
	panelFixedMax = 16
	dstNumberMax  = 0xffff // extended commands carry 16-bit numbers
)

var errFixedDests = fmt.Errorf("fixed destinations are at most %d different whole numbers from 0 to %d", panelFixedMax, dstNumberMax)

// cleanFixedDests checks the list and stores it as []int, dropping an empty one. The map is
// cloned before any change, since a stored panel's settings are shared and never mutated.
func cleanFixedDests(settings map[string]any) (map[string]any, error) {
	v, ok := settings[panelFixedKey]
	if !ok {
		return settings, nil
	}
	list, err := fixedList(v)
	if err != nil {
		return nil, err
	}
	settings = maps.Clone(settings)
	if len(list) == 0 {
		delete(settings, panelFixedKey)
	} else {
		settings[panelFixedKey] = list
	}
	return settings, nil
}

// fixedList reads JSON ([]any of float64), YAML ([]any of int) or a list already cleaned.
func fixedList(v any) ([]int, error) {
	var raw []any
	switch l := v.(type) {
	case nil:
		return nil, nil
	case []int:
		for _, n := range l {
			raw = append(raw, n)
		}
	case []any:
		raw = l
	default:
		return nil, errFixedDests
	}
	if len(raw) > panelFixedMax {
		return nil, errFixedDests
	}
	out := make([]int, 0, len(raw))
	for _, x := range raw {
		var n int
		switch x := x.(type) {
		case int:
			n = x
		case float64:
			if x != math.Trunc(x) || x < 0 || x > dstNumberMax {
				return nil, errFixedDests
			}
			n = int(x)
		default:
			return nil, errFixedDests
		}
		if n < 0 || n > dstNumberMax || slices.Contains(out, n) {
			return nil, errFixedDests
		}
		out = append(out, n)
	}
	return out, nil
}

// keepFixedDests is lay with target's own list in place of the source's. Sibling panels
// often patch to different destinations, so a Push leaves the list alone, like the name.
func keepFixedDests(lay panelLayout, target Panel) panelLayout {
	mine, has := target.Settings[panelFixedKey]
	_, theirs := lay.Settings[panelFixedKey]
	if !has && !theirs {
		return lay
	}
	lay.Settings = maps.Clone(lay.Settings)
	if lay.Settings == nil {
		lay.Settings = map[string]any{}
	}
	delete(lay.Settings, panelFixedKey)
	if has {
		lay.Settings[panelFixedKey] = mine
	}
	return lay
}

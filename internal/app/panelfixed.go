package app

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Fixed destination: `settings.fixedDests`, the destinations a panel always patches to, stored as
// router references. Empty or absent means off. Docs: docs/panels-design.md, docs/api.md.

const (
	panelFixedKey = "fixedDests"
	panelFixedMax = 16
	dstNumberMax  = 0xffff // extended commands carry 16-bit numbers
)

var errFixedDests = fmt.Errorf("fixed destinations are at most %d different destinations from 0 to %d", panelFixedMax, dstNumberMax)

// cleanFixedDests checks the list and stores it as []config.Ref, dropping an empty one. The map is
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

// fixedList reads JSON ([]any of float64 or reference objects), YAML ([]any of reference maps) or
// a list already cleaned. A bare number is the one router's destination; nameFixed fills the rest.
// A router other than this one is refused when named (nameFixed), since v2.0 drives one router.
func fixedList(v any) ([]config.Ref, error) {
	var raw []any
	switch l := v.(type) {
	case nil:
		return nil, nil
	case []config.Ref:
		for _, f := range l {
			raw = append(raw, f)
		}
	case []any:
		raw = l
	default:
		return nil, errFixedDests
	}
	if len(raw) > panelFixedMax {
		return nil, errFixedDests
	}
	out := make([]config.Ref, 0, len(raw))
	for _, x := range raw {
		f, ok := fixedRef(x)
		if !ok || f.N < 0 || f.N > dstNumberMax || slices.ContainsFunc(out, func(o config.Ref) bool { return o.N == f.N }) {
			return nil, errFixedDests
		}
		out = append(out, f)
	}
	return out, nil
}

func fixedNumber(x any) (int, bool) {
	switch x := x.(type) {
	case int:
		return x, true
	case float64:
		if x != math.Trunc(x) || x < 0 || x > dstNumberMax {
			return 0, false
		}
		return int(x), true
	}
	return 0, false
}

func fixedRef(x any) (config.Ref, bool) {
	switch x := x.(type) {
	case config.Ref:
		return x, true
	case map[string]any:
		n, ok := fixedNumber(x["n"])
		router, rok := x["router"].(string)
		name, nok := x["name"].(string)
		if !ok || (x["router"] != nil && !rok) || (x["name"] != nil && !nok) {
			return config.Ref{}, false
		}
		return config.Ref{Router: router, N: n, Name: name}, true
	}
	n, ok := fixedNumber(x)
	return config.Ref{N: n}, ok
}

// fixedNs is the destination numbers of a cleaned list.
func fixedNs(v any) []int {
	l, _ := v.([]config.Ref)
	out := make([]int, len(l))
	for i, f := range l {
		out[i] = f.N
	}
	return out
}

// nameFixed fills each reference's router and, from the router, a blank name, as namePresets does.
// settings must hold a new list from fixedList, since it is changed in place.
func (a *App) nameFixed(settings map[string]any) error {
	l, _ := settings[panelFixedKey].([]config.Ref)
	if len(l) == 0 {
		return nil
	}
	uid := a.r().cfg.UID
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range l {
		f := &l[i]
		switch {
		case f.Router == "":
			f.Router = uid
		case f.Router != uid:
			return errFixedDests
		}
		if n := a.router.dstNames[f.N]; f.Name == "" && strings.TrimSpace(n) != "" {
			f.Name = n
		}
	}
	return nil
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

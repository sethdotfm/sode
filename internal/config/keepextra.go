package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Unknown keys come only from the config file. A save that replaces a list from browser JSON
// takes each item's Extra from the saved item of the same name, so the browser can neither drop
// nor add them. A rename or a new item starts with none.

// savedMatch gives, per incoming item, the index of the saved item whose key matches (trimmed,
// any case, as the validators compare names), or -1. Each saved item matches at most once.
func savedMatch[T any](in, saved []T, key func(T) string) []int {
	out := make([]int, len(in))
	used := make([]bool, len(saved))
	for i, it := range in {
		out[i] = -1
		k := strings.TrimSpace(key(it))
		for j, s := range saved {
			if !used[j] && strings.EqualFold(k, strings.TrimSpace(key(s))) {
				used[j], out[i] = true, j
				break
			}
		}
	}
	return out
}

func WithFilterSetExtras(in, saved []FilterSet) []FilterSet {
	out := slices.Clone(in)
	for i, j := range savedMatch(out, saved, func(f FilterSet) string { return f.Name }) {
		out[i].Extra = nil
		if j >= 0 {
			out[i].Extra = saved[j].Extra
		}
	}
	return out
}

// Items have no name, so an item matches on its source and destinations; an edited item starts
// with no Extra.
func WithPresetExtras(in, saved []Preset) []Preset {
	out := ClonePresets(in)
	itemKey := func(it PresetItem) string { return fmt.Sprint(it.Src, it.Dsts) }
	for i, j := range savedMatch(out, saved, func(p Preset) string { return p.Name }) {
		p := &out[i]
		p.Extra = nil
		var savedItems []PresetItem
		if j >= 0 {
			p.Extra, savedItems = saved[j].Extra, saved[j].Items
		}
		for k, m := range savedMatch(p.Items, savedItems, itemKey) {
			p.Items[k].Extra = nil
			if m >= 0 {
				p.Items[k].Extra = savedItems[m].Extra
			}
		}
	}
	return out
}

func WithAPIClientExtras(in, saved []APIClient) []APIClient {
	out := slices.Clone(in)
	for i, j := range savedMatch(out, saved, func(c APIClient) string { return c.Name }) {
		out[i].Extra, out[i].Rights.Extra = nil, nil
		if j >= 0 {
			out[i].Extra, out[i].Rights.Extra = saved[j].Extra, saved[j].Rights.Extra
		}
	}
	return out
}

// WithExtraOf is a copy of r with saved's unknown keys.
func (r Rights) WithExtraOf(saved Rights) Rights {
	r = r.Clone()
	r.Extra = saved.Extra
	return r
}

func knownPref(k string) bool {
	_, ok := PrefValues[k]
	return ok
}

// WithUnknownPrefs keeps the prefs a browser sent that this release knows, plus the saved ones it
// doesn't.
func WithUnknownPrefs(sent, saved map[string]string) map[string]string {
	out := maps.Clone(sent)
	if out == nil {
		out = map[string]string{}
	}
	maps.DeleteFunc(out, func(k, _ string) bool { return !knownPref(k) })
	for k, v := range saved {
		if !knownPref(k) {
			out[k] = v
		}
	}
	return out
}

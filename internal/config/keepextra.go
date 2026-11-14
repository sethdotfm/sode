package config

import (
	"fmt"
	"slices"
	"strings"
)

// The With*Extras helpers back the list saves in docs/agents/settings-saves.md (Unknown keys).

// Names compare trimmed and in any case, as the validators compare them. Each saved item
// matches at most once, so duplicate names can't share one item's Extra.
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

// j is a savedMatch result; -1 means a new item, which starts with none.
func extraAt[T any](saved []T, j int, extra func(T) map[string]any) map[string]any {
	if j < 0 {
		return nil
	}
	return extra(saved[j])
}

func WithFilterSetExtras(in, saved []FilterSet) []FilterSet {
	out := slices.Clone(in)
	for i, j := range savedMatch(out, saved, func(f FilterSet) string { return f.Name }) {
		out[i].Extra = extraAt(saved, j, func(f FilterSet) map[string]any { return f.Extra })
	}
	return out
}

// Items have no name, so one matches on source and destinations.
func WithPresetExtras(in, saved []Preset) []Preset {
	out := ClonePresets(in)
	itemKey := func(it PresetItem) string { return fmt.Sprint(it.Src, it.Dsts) }
	for i, j := range savedMatch(out, saved, func(p Preset) string { return p.Name }) {
		p := &out[i]
		p.Extra = extraAt(saved, j, func(s Preset) map[string]any { return s.Extra })
		var savedItems []PresetItem
		if j >= 0 {
			savedItems = saved[j].Items
		}
		for k, m := range savedMatch(p.Items, savedItems, itemKey) {
			p.Items[k].Extra = extraAt(savedItems, m, func(it PresetItem) map[string]any { return it.Extra })
		}
	}
	return out
}

func WithAPIClientExtras(in, saved []APIClient) []APIClient {
	out := slices.Clone(in)
	for i, j := range savedMatch(out, saved, func(c APIClient) string { return c.Name }) {
		out[i].Extra = extraAt(saved, j, func(c APIClient) map[string]any { return c.Extra })
		out[i].Rights.Extra = extraAt(saved, j, func(c APIClient) map[string]any { return c.Rights.Extra })
	}
	return out
}

func (r Rights) WithExtraOf(saved Rights) Rights {
	r = r.Clone()
	r.Extra = saved.Extra
	return r
}

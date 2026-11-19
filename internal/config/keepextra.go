package config

import (
	"fmt"
	"slices"
	"strings"
)

// The With*Extras helpers back the list saves in docs/agents/settings-saves.md (Unknown keys).

// Names compare trimmed and in any letter case, as the validators compare them. Each saved item
// matches at most one time, so duplicate names cannot share one item's Extra.
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
		if j >= 0 && out[i].Broken == nil {
			out[i].Broken = slices.Clone(saved[j].Broken)
		}
	}
	return out
}

// A preset matches on its hidden id. A preset sent without an id gets the id of the saved preset
// of the same name, so a Replace keeps what names it. Items have no name, so one matches on
// source and destination numbers.
func WithPresetExtras(in, saved []Preset) []Preset {
	out := ClonePresets(in)
	sent := map[string]bool{}
	for _, p := range out {
		sent[p.ID] = p.ID != ""
	}
	for i, j := range savedMatch(out, saved, func(p Preset) string { return p.Name }) {
		if out[i].ID == "" && j >= 0 && saved[j].ID != "" && !sent[saved[j].ID] {
			out[i].ID = saved[j].ID
			sent[saved[j].ID] = true
		}
	}
	itemKey := func(it PresetItem) string { return fmt.Sprint(it.Src.N, it.DstNs()) }
	for i := range out {
		p := &out[i]
		j := slices.IndexFunc(saved, func(s Preset) bool { return s.ID != "" && s.ID == p.ID })
		p.Extra = extraAt(saved, j, func(s Preset) map[string]any { return s.Extra })
		// Browsers send no marks; the Broken check keeps them (internal/app/brokencheck.go).
		if j >= 0 && p.Broken == nil {
			p.Broken = slices.Clone(saved[j].Broken)
		}
		if j >= 0 && p.Moved == nil {
			p.Moved = slices.Clone(saved[j].Moved)
		}
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

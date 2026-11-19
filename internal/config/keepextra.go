package config

import (
	"cmp"
	"fmt"
	"maps"
	"reflect"
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

// A preset sent with no id gets the saved id of its name, so a Replace keeps what names it. Items match on numbers.
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

func (r Rights) WithExtraOf(saved Rights) Rights {
	r = r.Clone()
	r.Extra = saved.Extra
	return r
}

// Names only: a key from a newer release could hold a secret. "" when there are none.
func (c Config) UnknownKeysProblem() string {
	keys := c.UnknownKeys()
	if len(keys) == 0 {
		return ""
	}
	return "config.yaml has keys this release does not know, kept as is: " + NameKeys(keys)
}

// maxNamedKeys bounds key lists in the startup row and the import reply.
const maxNamedKeys = 10

func NameKeys(keys []string) string {
	if len(keys) == 0 {
		return "none"
	}
	if len(keys) <= maxNamedKeys {
		return strings.Join(keys, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(keys[:maxNamedKeys], ", "), len(keys)-maxNamedKeys)
}

// settingsPin: a v1 key; v2 gates on Roles (ADR 0010). Clone shares Extra, hence the copy.
var retiredKeys = []string{"settingsPin"}

func (c *Config) dropRetiredKeys() {
	if slices.ContainsFunc(retiredKeys, func(k string) bool { _, ok := c.Extra[k]; return ok }) {
		c.Extra = maps.Clone(c.Extra)
		for _, k := range retiredKeys {
			delete(c.Extra, k)
		}
	}
}

// UnknownKeys lists the keys Parse kept in Extra fields, as sorted paths named like the file
// (users[0].futureField).
func (c Config) UnknownKeys() []string {
	var out []string
	var walk func(v reflect.Value, path string)
	walk = func(v reflect.Value, path string) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				walk(v.Elem(), path)
			}
		case reflect.Slice, reflect.Array:
			for i := range v.Len() {
				walk(v.Index(i), fmt.Sprintf("%s[%d]", path, i))
			}
		case reflect.Struct:
			t := v.Type()
			for i := range t.NumField() {
				f := t.Field(i)
				if !f.IsExported() {
					continue
				}
				name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
				if f.Anonymous && f.Type == reflect.TypeOf(Router{}) {
					walk(v.Field(i), joinKey(path, "routers[0]"))
					continue
				}
				if f.Name == "Extra" {
					for k := range v.Field(i).Interface().(map[string]any) {
						out = append(out, joinKey(path, k))
					}
					continue
				}
				if name == "-" {
					continue
				}
				walk(v.Field(i), joinKey(path, cmp.Or(name, strings.ToLower(f.Name))))
			}
		}
	}
	walk(reflect.ValueOf(c), "")
	for i, u := range c.Users {
		for k := range u.Profile.Prefs {
			if !knownPref(k) {
				out = append(out, fmt.Sprintf("users[%d].profile.prefs.%s", i, k))
			}
		}
	}
	slices.Sort(out)
	return out
}

func joinKey(path, k string) string {
	if path == "" {
		return k
	}
	return path + "." + k
}

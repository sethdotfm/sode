package app

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Applying an Access preset from Settings (spec #543, Access presets): the page stages these
// values into the one Save, so the server's only parts are the values and the record's name.

// presetKeys: the one Save keys a preset writes. `api` carries only guestEnabled, since an
// object merges key by key and the other API settings are not the preset's.
var presetKeys = []string{"locks", "autoLockMinutes", "guestRole", "pinWaits", "publicPanels",
	"newPanelActs", "newUserDefaults", "guestApiScopes"}

// accessPresetView rides usersListResult (Administrator only). Users: the Persistent and No
// auto-lock a ticked user row gets.
type accessPresetView struct {
	Key    string                     `json:"key"`
	Name   string                     `json:"name"`
	Fields map[string]json.RawMessage `json:"fields"`
	Users  config.NewUserDefaults     `json:"users"`
}

// presetSaveFields: p as one Save fields, read back from a config p was applied to, so the
// values cannot drift from Apply (TestPresetSaveFieldsMatchApply).
func presetSaveFields(p config.AccessPreset) map[string]json.RawMessage {
	c := config.Defaults()
	p.Apply(&c)
	out := make(map[string]json.RawMessage, len(presetKeys)+1)
	for _, k := range presetKeys {
		out[k] = fieldJSON(c, k)
	}
	out["api"], _ = json.Marshal(map[string]bool{"guestEnabled": c.API.GuestEnabled})
	return out
}

func accessPresetViews() []accessPresetView {
	out := make([]accessPresetView, 0, len(config.AccessPresets))
	for _, p := range config.AccessPresets {
		u := p.Values.NewUserDefaults
		out = append(out, accessPresetView{p.Key, p.Name, presetSaveFields(p), config.NewUserDefaults{Persistent: u.Persistent, NoAutoLock: u.NoAutoLock}})
	}
	return out
}

// presetOfSave: the full name of the preset a save names, for its Settings saves row. Empty
// when the name is unknown or no kept field is one a preset writes.
func presetOfSave(name string, keep []string) string {
	if name == "" {
		return ""
	}
	p, err := config.AccessPresetByName(name)
	if err != nil {
		return ""
	}
	for _, k := range keep {
		if k == "api" || slices.Contains(presetKeys, k) || strings.HasPrefix(k, "user/") &&
			(strings.HasSuffix(k, "/persistent") || strings.HasSuffix(k, "/noAutoLock")) {
			return p.Name
		}
	}
	return ""
}

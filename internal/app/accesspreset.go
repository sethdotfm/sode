package app

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// `api` carries only guestEnabled: an object merges key by key, and the other API settings are not the preset's.
var presetKeys = []string{"locks", "autoLockMinutes", "guestRole", "pinWaits", "publicPanels",
	"newPanelActs", "newUserDefaults", "guestApiScopes"}

// Part of usersListResult (Administrator only). Rows is the setup sheet, for Run setup again.
type accessPresetView struct {
	Key    string                     `json:"key"`
	Name   string                     `json:"name"`
	Fields map[string]json.RawMessage `json:"fields"`
	Users  config.NewUserDefaults     `json:"users"`
	Rows   map[string]bool            `json:"rows"`
}

// Read back from a config that p was applied to, so the values cannot drift from Apply
// (TestPresetSaveFieldsMatchApply).
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
		out = append(out, accessPresetView{p.Key, p.Name, presetSaveFields(p), config.NewUserDefaults{Persistent: u.Persistent, NoAutoLock: u.NoAutoLock}, setupRows(p.Values)})
	}
	return out
}

// "" when the name is unknown or no kept field is one a preset writes.
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

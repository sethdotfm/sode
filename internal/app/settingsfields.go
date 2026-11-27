package app

import (
	"cmp"
	"reflect"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// settingField: one plain Config key of the one Save. Adding a setting is one row here
// (docs/agents/settings-saves.md). Item keys (a user's or an API client's attribute) are in
// onesavefields.go.
type settingField struct {
	key, section, label string
	// export: the settings.yaml group. It differs from section for maxPinLen and guestRights. The
	// group holds the key by its file name (statusLabel for routerShow), as config.yaml does.
	export string
	// source: written only by Manage users or the API section, never by the form.
	source bool
}

// Rows are in export order: group order, then key order in each group, as export files show.
var settingFields = []settingField{
	{"browserDefaults", "everyone-defaults", "Everyone's defaults", "everyone-defaults", false},
	{"brandText", "branding", "Logo text and page title", "branding", false},
	{"brandBorder", "branding", "Page border color", "branding", false},
	{"hiddenDsts", "history", "Hidden destinations in History", "history", false},
	{"hiddenDstsRouter", "history", "Hidden destinations router", "history", false},
	{"levelColors", "level-colors", "Level colors", "level-colors", false},
	{"confirmLockoutMs", "confirm", "Confirm lockout (ms)", "confirm", false},
	{"resultDismissSeconds", "confirm", "Success banner closes after (s)", "confirm", false},
	{"confirmEnter", "confirm", "Enter confirms takes", "confirm", false},
	{"locks", "locks", "Locks", "locks", false},
	{"autoLockMinutes", "locks", "Auto-lock after (min)", "locks", false},
	{"guestRole", "locks", "Guest role", "locks", false},
	{"pinWaits", "locks", "Wait after wrong PINs", "locks", false},
	{"maxPinLen", "users", "Longest PIN", "locks", true},
	{"newUserDefaults", "users", "New user defaults", "users", false},
	{"api", "api", "API", "api", true},
	{"guestRights", "users", "Guest rights", "api", true},
	{"guestApiScopes", "api", "Keyless API scopes", "api", true},
	{"publicPanels", "panels", "Public panels allowed", "panels", false},
	{"newPanelActs", "panels", "New-panel Authentication mode", "panels", false},
	{"routerShow", "routers", "Status label", "routers", false},
	{"tieHopWeight", "tie-lines", "Hop weight", "tie-lines", false},
	{"trustedProxies", "proxy", "Trusted proxies", "proxy", false},
	{"requireHttps", "proxy", "Require HTTPS", "proxy", false},
}

// saveFields: key to section. Every Config JSON key is here or saved apart
// (TestEveryFormFieldHasSection).
var saveFields, fieldLabels = func() (map[string]string, map[string]string) {
	sec, lab := map[string]string{}, map[string]string{}
	for _, f := range settingFields {
		sec[f.key], lab[f.key] = f.section, f.label
	}
	return sec, lab
}()

type exportSection struct {
	Name string
	Keys []string
}

// exportSections: the settings.yaml groups, which are data-section names in web/index.html. Keys are the file names
// of the config keys (the yaml tags), since settings.yaml is a part of config.yaml.
var exportSections = func() []exportSection {
	file := fileKeys()
	var out []exportSection
	for _, f := range settingFields {
		if f.export == "" {
			continue
		}
		if n := len(out); n == 0 || out[n-1].Name != f.export {
			out = append(out, exportSection{Name: f.export})
		}
		out[len(out)-1].Keys = append(out[len(out)-1].Keys, cmp.Or(file[f.key], f.key))
	}
	return out
}()

// fileKeys: the yaml name of each Config JSON key that has one. Only routerShow (statusLabel) differs today.
func fileKeys() map[string]string {
	out := map[string]string{}
	t := reflect.TypeFor[config.Config]()
	for i := range t.NumField() {
		f := t.Field(i)
		j, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		y, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if j != "" && j != "-" && y != "" && y != "-" {
			out[j] = y
		}
	}
	return out
}

// configKeys: the keys the form writes.
func configKeys() []string {
	var out []string
	for _, f := range settingFields {
		if !f.source {
			out = append(out, f.key)
		}
	}
	return out
}

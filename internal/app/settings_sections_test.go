package app

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"regexp"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// settingsSections reads each Settings section of index.html and the config keys its form fields save, in page order.
func settingsSections(t *testing.T) (order []string, keys map[string][]string) {
	t.Helper()
	b, err := fs.ReadFile(web.FS, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	fieldset := regexp.MustCompile(`(?s)<fieldset class="sode__group[^"]*"[^>]*data-section="([^"]+)"[^>]*>(.*?)</fieldset>`)
	// data-key fields, plus the form fields saveSettings reads by id (settingsform.js readForm).
	field := regexp.MustCompile(`data-key="([^"]+)"|data-gpref="|id="(cfg-default|cfg-lvlnum|cfg-colors|cfg-lock-main)"`)
	byID := map[string]string{"cfg-default": "defaultLevels", "cfg-lvlnum": "levelNumbering", "cfg-colors": "levelColors", "cfg-lock-main": "locks"}
	keys = map[string][]string{}
	for _, m := range fieldset.FindAllStringSubmatch(string(b), -1) {
		order = append(order, m[1])
		seen := map[string]bool{}
		for _, f := range field.FindAllStringSubmatch(m[2], -1) {
			k := f[1]
			switch {
			case f[2] != "":
				k = byID[f[2]]
			case k == "":
				k = "deviceDefaults"
			}
			if !seen[k] {
				seen[k] = true
				keys[m[1]] = append(keys[m[1]], k)
			}
		}
	}
	return order, keys
}

// The form fields of each Settings section still save after the move into topics. Each key is saved alone, the
// way a page sends it. Then the test reads it from the live config and from config.yaml.
func TestEverySettingsSectionSaves(t *testing.T) {
	order, keys := settingsSections(t)
	// Saved apart from the form: their own messages, or nothing to save yet.
	apart := map[string]bool{"this-browser": true, "my-user": true, "routers": true, "users": true, "api": true, "panels": true, "export-import": true, "run-setup-again": true}
	values := map[string]any{
		"deviceDefaults":       config.DeviceDefaults{Theme: "light", Layout: "desktop", Clock: "12", SetStack: "or", NextMode: "fuzzy", StackGap: "30"},
		"brandText":            "Stage B",
		"brandBorder":          config.BrandColors[1].Name,
		"hiddenDsts":           "65",
		"levelColors":          []string{"#abcdef"},
		"confirmLockoutMs":     900,
		"resultDismissSeconds": 9,
		"confirmEnter":         true,
		"router":               "127.0.0.1:1",
		"routerName":           "Studio B",
		"routerShow":           "name",
		"matrix":               2,
		"levels":               20,
		"levelWireOffset":      1,
		"levelNumbering":       "0",
		"numberOffset":         0,
		"nameRefreshSeconds":   120,
		"nameLength":           16,
		"nameRequests":         "ext",
		"defaultLevels":        "1-2",
		"tallyResyncSeconds":   30,
		"fireMode":             "native",
		"salvoGroup":           5,
		"pinMaxTries":          3,
		"locks":                config.Locks{Main: true, Log: true, Panel: true},
		"autoLockMinutes":      9,
		"trustedProxies":       []string{"10.0.0.2"},
	}
	// Saved only from an HTTPS address through a trusted proxy; settings_https tests cover it.
	skip := map[string]bool{"requireHttps": true}

	for _, sec := range order {
		if apart[sec] {
			if len(keys[sec]) > 0 {
				t.Errorf("%s is saved apart but has form fields %v", sec, keys[sec])
			}
			continue
		}
		if len(keys[sec]) == 0 {
			t.Errorf("%s has no form field: list it as saved apart", sec)
		}
		for _, k := range keys[sec] {
			if skip[k] {
				continue
			}
			want, ok := values[k]
			if !ok {
				t.Errorf("%s: no test value for %q", sec, k)
				continue
			}
			a := userApp(t, false)
			if err := pageSave(t, a, adminClient(t, a), map[string]any{k: want}); err != nil {
				t.Errorf("%s: saving %s: %v", sec, k, err)
				continue
			}
			loaded, _, err := config.Load(a.cfgPath)
			if err != nil {
				t.Fatal(err)
			}
			for where, c := range map[string]config.Config{"live": a.r().cfg, "config.yaml": loaded} {
				if got := jsonKey(t, c, k); !reflect.DeepEqual(got, jsonValue(t, want)) {
					t.Errorf("%s: %s in %s is %v, want %v", sec, k, where, got, want)
				}
			}
		}
	}
}

func jsonValue(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func jsonKey(t *testing.T, c config.Config, k string) any {
	t.Helper()
	m, _ := jsonValue(t, c).(map[string]any)
	return m[k]
}

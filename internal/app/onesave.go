package app

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/sethdotfm/sode/internal/config"
)

// The one Save (docs/agents/settings-saves.md): the page sends only changed fields, each
// with the version it loaded, and gets a result per field.

// sectionTag: who edits a Settings section, and whether lower Roles see it (Shown) or not
// (Hidden). Keep in step with SECTION_ROLES in web/settingsrolerules.js
// (TestSectionRolesMatchPage).
type sectionTag struct {
	edit  string // roleAnyone or a config Role
	shown bool
}

var sectionRoles = map[string]sectionTag{
	"this-browser":      {roleAnyone, true},
	"my-user":           {roleAnyone, true},
	"everyone-defaults": {config.RoleTrusted, true},
	"branding":          {config.RoleTrusted, true},
	"history":           {config.RoleTrusted, true},
	"level-colors":      {config.RoleAdministrator, true},
	"confirm":           {config.RoleAdministrator, true},
	"routers":           {config.RoleAdministrator, true},
	"router-matrix":     {config.RoleAdministrator, true},
	"router-names":      {config.RoleAdministrator, true},
	"takes":             {config.RoleAdministrator, true},
	"locks":             {config.RoleAdministrator, false},
	"users":             {config.RoleAdministrator, false},
	"api":               {config.RoleAdministrator, false},
	"panels":            {config.RoleAdministrator, false},
	"proxy":             {config.RoleAdministrator, false},
	"export-import":     {config.RoleAdministrator, false},
	"run-setup-again":   {config.RoleAdministrator, false},
}

// saveFields: the Config JSON keys the one Save takes, and the section of each. Every
// form-saved field (guard tag) is here (TestEveryGuardedFieldHasSection).
var saveFields = map[string]string{
	"browserDefaults":      "everyone-defaults",
	"brandText":            "branding",
	"brandBorder":          "branding",
	"hiddenDsts":           "history",
	"hiddenDstsRouter":     "history",
	"levelColors":          "level-colors",
	"confirmLockoutMs":     "confirm",
	"resultDismissSeconds": "confirm",
	"confirmEnter":         "confirm",
	"router":               "router-matrix",
	"routerName":           "router-matrix",
	"routerShow":           "router-matrix",
	"matrix":               "router-matrix",
	"levels":               "router-matrix",
	"levelWireOffset":      "router-matrix",
	"levelNumbering":       "router-matrix",
	"numberOffset":         "router-matrix",
	"nameRereadSeconds":    "router-names",
	"nameLength":           "router-names",
	"nameRequests":         "router-names",
	"defaultLevels":        "takes",
	"tallyResyncSeconds":   "takes",
	"fireMode":             "takes",
	"salvoGroup":           "takes",
	"pinWaits":             "locks",
	"locks":                "locks",
	"autoLockMinutes":      "locks",
	"trustedProxies":       "proxy",
	"requireHttps":         "proxy",
}

// Refusal reasons. The page shows them beside the field (spec #543, The one Save).
const (
	reasonRole     = "Role"
	reasonApproval = "needs Approval"
	reasonRecheck  = "needs PIN recheck"
	reasonConflict = "conflict"
	reasonPrereq   = "missing prerequisite"
	reasonInvalid  = "invalid"
)

type fieldEdit struct {
	Value   json.RawMessage `json:"value"`
	Version string          `json:"version"`
}

// fieldResult: status is saved, unchanged (nothing written) or refused. A conflict carries
// the live value and its version.
type fieldResult struct {
	Status  string          `json:"status"`
	Reason  string          `json:"reason,omitempty"`
	Err     string          `json:"err,omitempty"`
	Value   json.RawMessage `json:"value,omitempty"`
	Version string          `json:"version,omitempty"`
}

type settingsSaveResult struct {
	opResult
	Results map[string]fieldResult `json:"results"`
}

var jsonFields = sync.OnceValue(func() map[string][]int {
	out := map[string][]int{}
	for _, f := range config.Fields() {
		if name, _, _ := strings.Cut(f.Tag.Get("json"), ","); name != "" && name != "-" {
			out[name] = f.Index
		}
	}
	return out
})

func fieldOf(cfg *config.Config, key string) reflect.Value {
	return reflect.ValueOf(cfg).Elem().FieldByIndex(jsonFields()[key])
}

func fieldJSON(cfg config.Config, key string) json.RawMessage {
	b, _ := json.Marshal(fieldOf(&cfg, key).Interface())
	return b
}

// fieldVersion: a hash of the value, so it needs no state and any writer (an import, the
// API, a file edit) moves it.
func fieldVersion(cfg config.Config, key string) string {
	h := fnv.New64a()
	h.Write(fieldJSON(cfg, key))
	return fmt.Sprintf("%016x", h.Sum64())
}

// fieldVersions goes in every hello.
func fieldVersions(cfg config.Config) map[string]string {
	out := make(map[string]string, len(saveFields))
	for k := range saveFields {
		out[k] = fieldVersion(cfg, k)
	}
	return out
}

// setField decodes over the current value, as the config message does: an object merges
// key by key, a list replaces.
func setField(cfg *config.Config, key string, v json.RawMessage) error {
	return json.Unmarshal(v, fieldOf(cfg, key).Addr().Interface())
}

// fieldRefusal: the reason role cannot save a field of section sec; "" when it can.
func fieldRefusal(role, sec string) string {
	t, ok := sectionRoles[sec]
	switch {
	case !ok:
		return reasonRole
	case t.edit == roleAnyone || config.RoleAtLeast(role, t.edit):
		return ""
	case t.shown:
		return reasonApproval
	}
	return reasonRole
}

func (a *App) serveSettingsSave(c *client, m *inMsg) {
	go func() {
		defer m.finish()
		res, err := a.saveSettingsFields(c, m.Fields)
		out := settingsSaveResult{opResult: opResultFor("settingsSave", &m.ID, err), Results: res}
		n := 0
		for _, r := range res {
			if r.Status == "saved" {
				n++
			}
		}
		a.command(c, "settingsSave", outcome(err), fmt.Sprintf("%d of %d %s saved", n, len(res), plural(len(res), "field", "fields")), err)
		if err != nil {
			out.Locked = locked(err)
		}
		a.reply(c, out)
	}()
}

// saveSettingsFields checks each field on its own against live, then writes the ones that
// pass in one commit. No commit when none changes anything.
func (a *App) saveSettingsFields(c *client, edits map[string]fieldEdit) (map[string]fieldResult, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	a.mu.Lock()
	role, rechecked := a.roleMu(c), a.recheckedMu(c)
	live := a.r().cfg
	var actor config.User
	if u := c.u.user.Load(); u != nil {
		actor, _ = live.FindUserByID(u.ID)
	}
	a.mu.Unlock()
	if role == "" {
		return nil, errPageLocked
	}
	res := make(map[string]fieldResult, len(edits))
	refuse := func(key, reason string, err error) {
		r := fieldResult{Status: "refused", Reason: reason}
		if err != nil {
			r.Err = err.Error()
		}
		res[key] = r
	}
	var keep []string
	for _, key := range slices.Sorted(maps.Keys(edits)) {
		e := edits[key]
		sec, ok := saveFields[key]
		if !ok {
			refuse(key, reasonRole, fmt.Errorf("%s is not saved here", key))
			continue
		}
		// Before the conflict check, so a conflict never shows a value hidden from that Role.
		if why := fieldRefusal(role, sec); why != "" {
			refuse(key, why, roleErr{need: sectionRoles[sec].edit})
			continue
		}
		one := live.Clone()
		if err := setField(&one, key, e.Value); err != nil {
			refuse(key, reasonInvalid, err)
			continue
		}
		one.Normalize()
		if string(fieldJSON(one, key)) == string(fieldJSON(live, key)) {
			res[key] = fieldResult{Status: "unchanged"}
			continue
		}
		if e.Version != fieldVersion(live, key) {
			res[key] = fieldResult{Status: "refused", Reason: reasonConflict, Err: "someone else saved this after you opened Settings",
				Value: fieldJSON(live, key), Version: fieldVersion(live, key)}
			continue
		}
		if err := one.Validate(); err != nil {
			refuse(key, reasonInvalid, err)
			continue
		}
		if !rechecked {
			if what := criticalEdits(live, one, actor); len(what) > 0 {
				refuse(key, reasonRecheck, recheckErr{what: what})
				continue
			}
		}
		// Turning it on from a connection that is not secure would lock this page out (ADR 0005).
		if key == "requireHttps" && one.RequireHTTPS && !c.secure {
			refuse(key, reasonPrereq, errInsecureSave)
			continue
		}
		keep = append(keep, key)
	}
	if len(keep) == 0 {
		return res, nil
	}
	err := a.applyConfig(func(cur *config.Config) error {
		for _, key := range keep {
			if err := setField(cur, key, edits[key].Value); err != nil {
				return err
			}
		}
		return nil
	})
	for _, key := range keep {
		if err != nil {
			refuse(key, reasonInvalid, err)
			continue
		}
		res[key] = fieldResult{Status: "saved", Version: fieldVersion(a.r().cfg, key)}
	}
	if err == nil && slices.Contains(keep, "requireHttps") {
		a.auditRequireHTTPS(connActor(c), live.RequireHTTPS, a.r().cfg.RequireHTTPS)
	}
	return res, err
}

package app

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
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

// saveFields: the Config keys the one Save takes, and the section of each. Every Config JSON
// key is here or saved apart (TestEveryFormFieldHasSection). Item keys (a user's or an API
// client's attribute) are in onesavefields.go.
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
	"guestRole":            "locks",
	"newUserDefaults":      "users",
	"publicPanels":         "panels",
	"newPanelActs":         "panels",
	"trustedProxies":       "proxy",
	"requireHttps":         "proxy",
	"maxPinLen":            "users",
	"guestRights":          "users",
	"api":                  "api",
	"guestApiScopes":       "api",
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
	// Interruptions: found again before the commit (interruptions.go).
	Interruptions []interruption `json:"interruptions,omitempty"`
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
	f, ok := lookupField(key)
	if !ok {
		return json.RawMessage("null")
	}
	b, _ := json.Marshal(f.get(cfg))
	return b
}

// fieldVersion: a hash of the value, so it needs no state and any writer (an import, the
// API, a file edit) moves it.
func fieldVersion(cfg config.Config, key string) string {
	h := fnv.New64a()
	h.Write(fieldJSON(cfg, key))
	return fmt.Sprintf("%016x", h.Sum64())
}

// fieldVersions goes in every hello. It leaves out the versions that only an Administrator
// gets (hiddenVersions).
func fieldVersions(cfg config.Config) map[string]string {
	out := make(map[string]string, len(saveFields))
	for k := range saveFields {
		if _, hidden := hiddenFields[k]; !hidden {
			out[k] = fieldVersion(cfg, k)
		}
	}
	return out
}

// setField decodes over the current value, as the config message does: an object merges
// key by key, a list replaces.
func setField(cfg *config.Config, key string, v json.RawMessage, x *saveCtx) error {
	f, ok := lookupField(key)
	if !ok {
		return fmt.Errorf("%s is not saved here", key)
	}
	return f.set(cfg, v, x)
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
		res, stops, err := a.saveSettingsFields(c, m.Fields, m.approver, m.asRole, m.AccessPreset)
		out := settingsSaveResult{opResult: opResultFor("settingsSave", &m.ID, err), Results: res, Interruptions: stops}
		n := 0
		for _, r := range res {
			if r.Status == "saved" {
				n++
			}
		}
		note := fmt.Sprintf("%d of %d %s saved", n, len(res), plural(len(res), "field", "fields"))
		a.command(c, "settingsSave", outcome(err), approvedNote(m.approver, note), err)
		a.reply(c, out)
	}()
}

// savePlan: what a save would write. keep are the fields that pass every check against live.
type savePlan struct {
	live  config.Config
	actor config.User
	res   map[string]fieldResult
	edits map[string]fieldEdit
	keep  []string
	moves map[string]string
	x     *saveCtx
}

// next: live with the kept fields written.
func (p *savePlan) next() config.Config {
	n := p.live.Clone()
	for _, key := range p.keep {
		_ = setField(&n, key, p.edits[key].Value, p.x)
	}
	n.Normalize()
	return n
}

func (p *savePlan) refuse(key, reason string, err error) {
	r := fieldResult{Status: "refused", Reason: reason}
	if err != nil {
		r.Err = err.Error()
	}
	p.res[key] = r
}

// saveSettingsFields checks each field on its own against live, then writes the ones that
// pass in one commit. No commit when none changes anything. ap, the approver of an Approval
// (approval.go), stands in for the Role only in Shown sections; it is nil without one. asRole: a
// Request approval's requester Role when sent, used when the requester is under the Lock now.
// preset: the Access preset the page staged, named in the Settings saves row.
func (a *App) saveSettingsFields(c *client, edits map[string]fieldEdit, ap *config.User, asRole, preset string) (map[string]fieldResult, []interruption, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	plan, err := a.planSave(c, edits, ap, asRole, false)
	if err != nil {
		return nil, nil, err
	}
	res, keep, live, x, actor := plan.res, plan.keep, plan.live, plan.x, plan.actor
	if len(keep) == 0 {
		return res, nil, nil
	}
	stops := a.interruptions(live, plan.next())
	err = a.applyConfig(func(cur *config.Config) error {
		for _, key := range keep {
			if err := setField(cur, key, edits[key].Value, x); err != nil {
				return err
			}
		}
		return nil
	})
	now := a.r().cfg
	for _, key := range keep {
		if err != nil {
			plan.refuse(key, reasonInvalid, err)
			continue
		}
		r := fieldResult{Status: "saved", Version: fieldVersion(now, key)}
		if f, _ := lookupField(key); f.adds {
			r.Value, r.Version = addedResult(now, key, x), ""
		}
		res[key] = r
	}
	if err != nil {
		return res, nil, err
	}
	if slices.Contains(keep, "requireHttps") {
		a.auditRequireHTTPS(connActor(c), live.RequireHTTPS, now.RequireHTTPS)
	}
	a.recordSaveApproved(connActor(c), "save", a.savedFields(live, now, keep, x), len(criticalEdits(live, now, actor)) > 0, ap, interruptionTexts(stops),
		presetOfSave(preset, keep))
	a.auditAPISettings(connActor(c), live, now)
	a.panelsFollowUsers(live.Users, now.Users)
	if n, err := a.panels.moveOwners(plan.moves); err != nil {
		log.Printf("panels: new owner reached %d panels, then: %v", n, err)
	} else if n > 0 {
		log.Printf("panels: %d panels have a new owner", n)
	}
	return res, stops, nil
}

// planSave runs every check of a save and writes nothing. review: the review asks before the PIN
// recheck, so a field that needs one is not refused for it. Caller holds cfgMu.
func (a *App) planSave(c *client, edits map[string]fieldEdit, ap *config.User, asRole string, review bool) (*savePlan, error) {
	a.mu.Lock()
	role, rechecked := a.roleMu(c), a.recheckedMu(c) || review
	live := a.r().cfg
	var actor config.User
	if u := c.u.user.Load(); u != nil {
		actor, _ = live.FindUserByID(u.ID)
	}
	a.mu.Unlock()
	if role == "" {
		role = asRole
	}
	if role == "" {
		return nil, errPageLocked
	}
	apRole := ""
	if ap != nil {
		if apRole = approverRole(live, ap); apRole == "" {
			return nil, approvalErr{ap.Name + " changed meanwhile: ask for the Approval again"}
		}
		// The whole Approval is refused when the approver cannot approve every field that needs it.
		for key := range edits {
			if f, ok := lookupField(key); ok && fieldRefusal(role, f.section) == reasonApproval && fieldRefusal(apRole, f.section) != "" {
				return nil, cannotApprove(ap, sectionRoles[f.section].edit)
			}
		}
		actor = *ap
		rechecked = rechecked || apRole == config.RoleAdministrator
	}
	plan := &savePlan{live: live, actor: actor, res: make(map[string]fieldResult, len(edits)), edits: edits, x: &saveCtx{}}
	res, x, refuse := plan.res, plan.x, plan.refuse
	// First the checks that need no other field: section, unchanged, conflict.
	var cands []string
	for _, key := range slices.Sorted(maps.Keys(edits)) {
		e := edits[key]
		f, ok := lookupField(key)
		if !ok {
			refuse(key, reasonRole, fmt.Errorf("%s is not saved here", key))
			continue
		}
		// Before the conflict check, so a conflict never shows a value hidden from that Role.
		if why := fieldRefusal(role, f.section); why != "" && (why != reasonApproval || ap == nil) {
			refuse(key, why, roleErr{need: sectionRoles[f.section].edit})
			continue
		}
		one := live.Clone()
		if err := f.set(&one, e.Value, x); err != nil {
			// Another field of this save can make it fit, for example a higher Longest PIN. Try
			// again on top of all the other fields.
			one = live.Clone()
			for _, k := range slices.Sorted(maps.Keys(edits)) {
				if k != key {
					_ = setField(&one, k, edits[k].Value, x)
				}
			}
			if f.set(&one, e.Value, x) != nil {
				refuse(key, reasonInvalid, err)
				continue
			}
		}
		one.Normalize()
		if !f.adds && string(fieldJSON(one, key)) == string(fieldJSON(live, key)) {
			res[key] = fieldResult{Status: "unchanged"}
			continue
		}
		if !f.adds && e.Version != fieldVersion(live, key) {
			r := fieldResult{Status: "refused", Reason: reasonConflict, Err: "someone else saved this after you opened Settings",
				Version: fieldVersion(live, key)}
			if !f.secret {
				r.Value = fieldJSON(live, key)
			}
			res[key] = r
			continue
		}
		cands = append(cands, key)
	}
	// with: live plus key, plus the other candidates of its item (scope 1) or all of them (scope
	// 2). So a Role and a PIN, or a rename and a new user of the old name, validate together.
	with := func(key string, scope int) config.Config {
		one := live.Clone()
		f, _ := lookupField(key)
		for _, k := range cands {
			g, _ := lookupField(k)
			if k == key || scope == 2 || scope == 1 && f.item != "" && g.item == f.item {
				_ = setField(&one, k, edits[k].Value, x)
			}
		}
		one.Normalize()
		return one
	}
	var keep []string
	for _, key := range cands {
		var err error
		for scope := range 3 {
			if err = validateSave(live, with(key, scope)); err == nil {
				break
			}
		}
		if err != nil {
			if scoped := validateSave(live, with(key, 0)); scoped != nil {
				err = scoped // the reason of the field alone reads best
			}
			refuse(key, reasonInvalid, err)
			continue
		}
		one := with(key, 0)
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
	plan.moves, plan.keep = a.removedUsersPanels(c, live, edits, keep, refuse)
	return plan, nil
}

// validateSave: what update checks, so a field that would fail the commit is refused alone.
func validateSave(live, next config.Config) error {
	n := next.Clone()
	dropOwnerless(&n)
	if err := n.Validate(); err != nil {
		return err
	}
	return keepsAdministrator(live, n)
}

// removedUsersPanels gives the panels of each removed user a new owner. The owner is the one
// the value names ({"owner": id}), else the acting user. A user who owns panels needs a named
// owner that the save keeps, or the remove is refused.
func (a *App) removedUsersPanels(c *client, live config.Config, edits map[string]fieldEdit, keep []string,
	refuse func(key, reason string, err error)) (map[string]string, []string) {
	removed := map[string]bool{}
	for _, key := range keep {
		if id, ok := removeKeyUser(key); ok {
			removed[id] = true
		}
	}
	moves := map[string]string{}
	out := keep[:0:0]
	for _, key := range keep {
		id, ok := removeKeyUser(key)
		if !ok {
			out = append(out, key)
			continue
		}
		var v struct {
			Owner string `json:"owner"`
		}
		_ = json.Unmarshal(edits[key].Value, &v)
		_, kept := live.FindUserByID(v.Owner)
		kept = kept && !removed[v.Owner]
		if n := a.panels.ownedBy(id); n > 0 && !kept {
			u, _ := live.FindUserByID(id)
			refuse(key, reasonInvalid, fmt.Errorf("%s owns %d panels: choose a new owner for them from the users you keep", u.Name, n))
			continue
		}
		to := v.Owner
		if !kept {
			to = c.userID()
		}
		if to != "" {
			moves[id] = to
		}
		out = append(out, key)
	}
	return moves, out
}

func removeKeyUser(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, "user/")
	if !ok {
		return "", false
	}
	return strings.CutSuffix(rest, "/remove")
}

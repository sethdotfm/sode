package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"maps"
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
	"tie-lines":         {config.RoleAdministrator, false},
	"protects":          {config.RoleAdministrator, false},
	"locks":             {config.RoleAdministrator, false},
	"users":             {config.RoleAdministrator, false},
	"api":               {config.RoleAdministrator, false},
	"panels":            {config.RoleAdministrator, false},
	"proxy":             {config.RoleAdministrator, false},
	"export-import":     {config.RoleAdministrator, false},
	"run-setup-again":   {config.RoleAdministrator, false},
}

// Refusal reasons; the page shows them beside the field.
const (
	reasonRole     = "Role"
	reasonApproval = "needs Approval"
	reasonRecheck  = "needs PIN recheck"
	reasonConflict = "conflict"
	reasonPrereq   = "missing prerequisite"
	reasonInvalid  = "invalid"
	reasonOwnPin   = "own PIN" // the saver's own PIN, which only Change my PIN changes
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
	routerVersions(cfg, out)
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
		by := c.who() // before the save, which can end c's step-in
		res, stops, err := a.saveSettingsFields(c, m.Fields, m.approver, m.asRole, m.AccessPreset)
		out := settingsSaveResult{opResult: opResultFor("settingsSave", &m.ID, err), Results: res, Interruptions: stops}
		n := 0
		for _, r := range res {
			if r.Status == "saved" {
				n++
			}
		}
		note := fmt.Sprintf("%d of %d %s saved", n, len(res), plural(len(res), "field", "fields"))
		a.commandBy(by, "settingsSave", outcome(err), approvedNote(m.approver, note), err)
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

func (p *savePlan) next() config.Config { return p.build(p.keep) }

func (p *savePlan) build(keys []string) config.Config {
	n := p.live.Clone()
	for _, key := range removesLast(keys) {
		_ = setField(&n, key, p.edits[key].Value, p.x)
	}
	n.Normalize()
	return n
}

// screen refuses key for Role, a bad value or a conflict, marks it unchanged, or answers true:
// a candidate for the validation passes.
func (p *savePlan) screen(key, role string, ap *config.User) bool {
	live, e := p.live, p.edits[key]
	f, ok := lookupField(key)
	if !ok {
		p.refuse(key, reasonRole, fmt.Errorf("%s is not saved here", key))
		return false
	}
	// Before the conflict check, so a conflict never shows a value hidden from that Role.
	if why := fieldRefusal(role, f.section); why != "" && (why != reasonApproval || ap == nil) {
		p.refuse(key, why, roleErr{need: sectionRoles[f.section].edit})
		return false
	}
	one := live.Clone()
	if err := f.set(&one, e.Value, p.x); err != nil {
		// Another field of this save can make it fit, for example a higher Longest PIN.
		one = live.Clone()
		for _, k := range slices.Sorted(maps.Keys(p.edits)) {
			if k != key {
				_ = setField(&one, k, p.edits[k].Value, p.x)
			}
		}
		if f.set(&one, e.Value, p.x) != nil {
			reason := reasonInvalid
			if errors.Is(err, errOwnPinHere) {
				reason = reasonOwnPin
			}
			p.refuse(key, reason, err)
			return false
		}
	}
	one.Normalize()
	if f.adds || f.action {
		return true
	}
	if string(fieldJSON(one, key)) == string(fieldJSON(live, key)) {
		p.res[key] = fieldResult{Status: "unchanged"}
		return false
	}
	if e.Version != fieldVersion(live, key) {
		r := fieldResult{Status: "refused", Reason: reasonConflict, Err: "someone else saved this after you opened Settings",
			Version: fieldVersion(live, key)}
		if !f.secret {
			r.Value = fieldJSON(live, key)
		}
		p.res[key] = r
		return false
	}
	return true
}

// with: live plus key, plus the other candidates of its item (scope 1) or all of them (scope 2).
// So a Role and a PIN, or a rename and a new user of the old name, validate together.
func (p *savePlan) with(cands []string, key string, scope int) config.Config {
	f, _ := lookupField(key)
	return p.build(slices.DeleteFunc(slices.Clone(cands), func(k string) bool {
		g, _ := lookupField(k)
		return k != key && scope != 2 && (scope != 1 || f.item == "" || g.item != f.item)
	}))
}

// pruneTogether: fields that pass alone can fail together, so it refuses each one whose absence
// mends the rest.
func (p *savePlan) pruneTogether(keep []string) []string {
	for len(keep) > 0 {
		err := validateSave(p.live, p.build(keep))
		if err == nil {
			break
		}
		bad := slices.DeleteFunc(slices.Clone(keep), func(k string) bool {
			return validateSave(p.live, p.build(slices.DeleteFunc(slices.Clone(keep), func(o string) bool { return o == k }))) != nil
		})
		if len(bad) == 0 {
			bad, keep = keep, nil
		} else {
			keep = slices.DeleteFunc(keep, func(k string) bool { return slices.Contains(bad, k) })
		}
		for _, k := range bad {
			p.refuse(k, reasonInvalid, err)
		}
	}
	return keep
}

func (p *savePlan) refuse(key, reason string, err error) {
	r := fieldResult{Status: "refused", Reason: reason}
	if err != nil {
		r.Err = err.Error()
	}
	p.res[key] = r
}

// ap: the approver (approval.go), stands in for the Role only in Shown sections; nil without one. asRole: the requester's
// Role of a Request approval, for a requester now under the Lock. preset: the staged Access preset, for the Settings saves
// row.
func (a *App) saveSettingsFields(c *client, edits map[string]fieldEdit, ap *config.User, asRole, preset string) (map[string]fieldResult, []interruption, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	by := connActor(c) // before the commit, which can end c's step-in (stepOutsOnSaveMu)
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
		for _, key := range removesLast(keep) {
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
		} else if f.action {
			r.Version = ""
		}
		res[key] = r
	}
	if err != nil {
		return res, nil, err
	}
	if slices.Contains(keep, "requireHttps") {
		a.auditRequireHTTPS(by, live.RequireHTTPS, now.RequireHTTPS)
	}
	saved := a.savedFields(live, now, keep, x)
	if slices.Contains(keep, swapKey) {
		saved = append(saved, a.swapPanelsField(x, edits, now, true))
	}
	var goneRouters []string
	for _, key := range keep {
		if uid, ok := removeKeyRouter(key); ok {
			goneRouters = append(goneRouters, uid)
		}
	}
	a.routersGoneStores(goneRouters)
	a.recordSaveApproved(by, "save", saved, len(criticalEdits(live, now, actor)) > 0, ap, interruptionTexts(stops),
		presetOfSave(preset, keep))
	a.auditAPISettings(by, live, now)
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
	x, swapErr := a.swapCtx(live, edits)
	plan := &savePlan{live: live, actor: actor, res: make(map[string]fieldResult, len(edits)), edits: edits, x: x}
	refuse := plan.refuse
	if swapErr != nil {
		refuse(swapKey, reasonInvalid, swapErr)
		edits = maps.Clone(edits)
		delete(edits, swapKey)
		plan.edits = edits
	}
	var cands []string
	x.self = c.userID() // during a step-in, the stepped-in user
	for _, key := range slices.Sorted(maps.Keys(edits)) {
		if plan.screen(key, role, ap) {
			cands = append(cands, key)
		}
	}
	var keep []string
	for _, key := range cands {
		var err error
		for scope := range 3 {
			if err = validateSave(live, plan.with(cands, key, scope)); err == nil {
				break
			}
		}
		if err != nil {
			if scoped := validateSave(live, plan.with(cands, key, 0)); scoped != nil {
				err = scoped // the reason of the field alone reads best
			}
			refuse(key, reasonInvalid, err)
			continue
		}
		one := plan.with(cands, key, 0)
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
	keep = plan.pruneTogether(keep)
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

// removesLast: a Remove router goes after the other fields, so an edit of that router in the same save still finds it.
func removesLast(keys []string) []string {
	out := slices.Clone(keys)
	slices.SortStableFunc(out, func(a, b string) int {
		_, ra := removeKeyRouter(a)
		_, rb := removeKeyRouter(b)
		switch {
		case ra == rb:
			return 0
		case ra:
			return 1
		}
		return -1
	})
	return out
}

func removeKeyUser(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, "user/")
	if !ok {
		return "", false
	}
	return strings.CutSuffix(rest, "/remove")
}

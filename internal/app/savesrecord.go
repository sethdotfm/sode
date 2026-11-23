package app

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Values come from the config before and after, never from what the page sent, which can hold a PIN
// (docs/agents/comm-log.md).

// sectionNames and topics: the words of index.html's legends and topic headings.
var sectionNames = map[string]string{
	"my-user":           "My user",
	"everyone-defaults": "Everyone's defaults",
	"branding":          "Branding",
	"history":           "History",
	"level-colors":      "Level colors",
	"confirm":           "Confirm and results",
	"routers":           "Routers",
	"router-matrix":     "Router & matrix",
	"router-levels":     "Router levels",
	"router-names":      "Router names",
	"takes":             "Takes",
	"protects":          "Protects",
	"locks":             "Locks",
	"users":             "Users",
	"api":               "API",
	"panels":            "Panels",
	"proxy":             "Proxy",
	"export-import":     "Export & Import",
	"run-setup-again":   "Run setup again",
}

func topicOf(section string) string {
	switch section {
	case "this-browser", "my-user":
		return "Preferences"
	case "routers", "router-levels", "router-matrix", "router-names", "takes", "protects":
		return "Routers"
	case "locks", "users", "api", "panels":
		return "Access"
	case "proxy", "export-import", "run-setup-again":
		return "Server"
	}
	return "Look and feel"
}

var attrLabels = map[string]string{
	"name": "Name", "role": "Role", "pin": "PIN", "rights": "Rights", "persistent": "Never sign out",
	"noAutoLock": "No auto-lock", "remove": "Remove", "add": "Add", "scopes": "Scopes",
}

// routerAttrLabels: the labels of the router keys, as the first router's fields name them.
var routerAttrLabels = map[string]string{
	"levels": "Levels", "levelWireOffset": "Level offset", "levelNumbering": "Level numbering", "defaultLevels": "Default levels",
	"prefix": "Prefix", "title": "Title", "address": "Address", "matrix": "Matrix", "remove": "Remove",
}

// itemLabel: "User Ana: Role", "API client Bot: Scopes" or "Router B: Levels". The name is the one before the save,
// else after it.
func itemLabel(key string, was, now config.Config) string {
	if rest, ok := strings.CutPrefix(key, "user/"); ok {
		id, attr, _ := strings.Cut(rest, "/")
		u, ok := was.FindUserByID(id)
		if !ok {
			u, _ = now.FindUserByID(id)
		}
		return "User " + u.Name + ": " + attrLabels[attr]
	}
	if rest, ok := strings.CutPrefix(key, "router/"); ok {
		uid, attr, _ := strings.Cut(rest, "/")
		r, ok := was.RouterByUID(uid)
		if !ok {
			r, _ = now.RouterByUID(uid)
		}
		return "Router " + r.Prefix + ": " + routerAttrLabels[attr]
	}
	rest := strings.TrimPrefix(key, "client/")
	i := strings.LastIndex(rest, "/")
	return "API client " + rest[:i] + ": " + attrLabels[rest[i+1:]]
}

func newSavedField(key, section, label string) SavedField {
	return SavedField{Key: key, Label: label, SectionID: section, Section: sectionNames[section], Topic: topicOf(section)}
}

// rightsField: nil lists in Rights mean everything.
func rightsField(key string) bool {
	return key == "guestRights" || strings.HasSuffix(key, "/rights")
}

var allJSON = json.RawMessage(`"everything"`)

func diffValue(base SavedField, was, now json.RawMessage, rights bool) []SavedField {
	var wo, no map[string]json.RawMessage
	if json.Unmarshal(was, &wo) == nil && json.Unmarshal(now, &no) == nil && wo != nil && no != nil {
		var out []SavedField
		subs := maps.Clone(wo)
		maps.Copy(subs, no)
		for _, k := range slices.Sorted(maps.Keys(subs)) {
			if bytes.Equal(wo[k], no[k]) {
				continue
			}
			f := base
			f.Label += ": " + k
			out = append(out, diffList(f, wo[k], no[k], rights)...)
		}
		return out
	}
	return diffList(base, was, now, rights)
}

func diffList(f SavedField, was, now json.RawMessage, rights bool) []SavedField {
	isNull := func(v json.RawMessage) bool { return len(v) == 0 || string(v) == "null" }
	var wl, nl []json.RawMessage
	// A null list is empty, except in Rights, where it is everything.
	lists := json.Unmarshal(was, &wl) == nil && json.Unmarshal(now, &nl) == nil && (wl != nil || nl != nil)
	if lists && (!rights || !isNull(was) && !isNull(now)) {
		for _, v := range nl {
			if !slices.ContainsFunc(wl, func(w json.RawMessage) bool { return bytes.Equal(w, v) }) {
				f.Added = append(f.Added, v)
			}
		}
		for _, v := range wl {
			if !slices.ContainsFunc(nl, func(w json.RawMessage) bool { return bytes.Equal(w, v) }) {
				f.Removed = append(f.Removed, v)
			}
		}
		if f.Added == nil && f.Removed == nil { // only the order changed
			f.Old, f.New = was, now
		}
		return []SavedField{f}
	}
	if rights && isNull(was) && nl != nil {
		was = allJSON
	}
	if rights && isNull(now) && wl != nil {
		now = allJSON
	}
	f.Old, f.New = was, now
	return []SavedField{f}
}

// savedFields: what each key in keys wrote between was and now. x names the ids that adds made.
func (a *App) savedFields(was, now config.Config, keys []string, x *saveCtx) []SavedField {
	var out []SavedField
	for _, key := range slices.Sorted(slices.Values(keys)) {
		spec, ok := lookupField(key)
		if !ok || spec.action {
			continue
		}
		label := fieldLabels[key]
		if label == "" {
			label = itemLabel(key, was, now)
		}
		base := newSavedField(key, spec.section, label)
		switch {
		case spec.secret:
			base.Changed = true
			out = append(out, base)
		case spec.adds:
			out = append(out, a.addedFields(base, key, now, x)...)
		default:
			fs := diffValue(base, fieldJSON(was, key), fieldJSON(now, key), rightsField(key))
			if id, ok := strings.CutPrefix(key, "user/"); ok && strings.HasSuffix(id, "/rights") {
				u, _ := now.FindUserByID(strings.TrimSuffix(id, "/rights"))
				if pc := a.panelClientsOf(now, u); len(pc) > 0 && len(fs) > 0 {
					var l []string
					for _, n := range pc {
						l = append(l, n.Client+" ("+n.Name+")")
					}
					fs[0].Note = "panel clients they created: " + strings.Join(l, ", ")
				}
			}
			out = append(out, fs...)
		}
	}
	return out
}

// A new user's PIN is its own field.
func (a *App) addedFields(base SavedField, key string, now config.Config, x *saveCtx) []SavedField {
	if id, ok := x.made[key]; ok && strings.HasPrefix(key, "router/") {
		r, _ := now.RouterByUID(id)
		base.Key = "router/" + id + "/add"
		base.Label = "New router " + r.Prefix
		base.New, _ = json.Marshal(map[string]any{"prefix": r.Prefix, "title": r.Title, "address": r.Address,
			"matrix": r.Matrix, "levels": r.Levels})
		return []SavedField{base}
	}
	if id, ok := x.made[key]; ok {
		u, _ := now.FindUserByID(id)
		base.Key = "user/" + id + "/add"
		base.Label = "New user " + u.Name
		base.New, _ = json.Marshal(map[string]any{"name": u.Name, "role": u.Role, "persistent": u.Persistent,
			"noAutoLock": u.NoAutoLock, "rights": u.Rights})
		out := []SavedField{base}
		if u.Pin != "" {
			pin := newSavedField("user/"+id+"/pin", base.SectionID, "User "+u.Name+": PIN")
			pin.Changed = true
			out = append(out, pin)
		}
		return out
	}
	rest := strings.TrimPrefix(key, "client/")
	cl, _ := now.FindAPIClient(rest[:strings.LastIndex(rest, "/")])
	base.Label = "New API client " + cl.Name
	base.New, _ = json.Marshal(map[string]any{"name": cl.Name, "scopes": cl.Scopes, "rights": cl.Rights})
	return []SavedField{base}
}

// clientChanges: what a write outside the one Save did to the API client name (a panel's own).
func clientChanges(was, now config.Config, name string) []SavedField {
	o, had := was.FindAPIClient(name)
	n, has := now.FindAPIClient(name)
	field := func(attr, label string) SavedField {
		return newSavedField("client/"+name+"/"+attr, usersSection, "API client "+name+": "+label)
	}
	switch {
	case !had && has:
		f := field("add", "Add")
		f.New, _ = json.Marshal(map[string]any{"name": n.Name, "scopes": n.Scopes, "rights": n.Rights, "networks": n.Networks})
		return []SavedField{f}
	case had && !has:
		f := field("remove", "Remove")
		f.Old, f.New = json.RawMessage("false"), json.RawMessage("true")
		return []SavedField{f}
	case !had:
		return nil
	}
	var out []SavedField
	ro, _ := json.Marshal(o.Rights)
	rn, _ := json.Marshal(n.Rights)
	if !bytes.Equal(ro, rn) {
		out = append(out, diffValue(field("rights", "Rights"), ro, rn, true)...)
	}
	if !slices.Equal(o.Networks, n.Networks) {
		no, _ := json.Marshal(o.Networks)
		nn, _ := json.Marshal(n.Networks)
		out = append(out, diffList(field("networks", "Networks"), no, nn, false)...)
	}
	return out
}

// recordSave writes one Settings saves row. why is how it came: "save" (Settings) or "api".
func (a *App) recordSave(by Actor, why string, fields []SavedField, critical bool) {
	a.recordSaveApproved(by, why, fields, critical, nil, nil, "")
}

// recordSaveApproved: ap is the approver of an Approval, nil without one. stops: the Interruptions
// at save time. preset: the Access preset's full name, or "".
func (a *App) recordSaveApproved(by Actor, why string, fields []SavedField, critical bool, ap *config.User, stops []string, preset string) {
	if len(fields) == 0 {
		return
	}
	e := event(kindSettings, why, redactKeys(by.String()), "ok")
	e.Note = a.panelNote(by)
	e.Save = &SettingsSave{Fields: fields, Critical: critical, Interruptions: stops, Preset: preset}
	if ap != nil {
		e.Save.Approver = ap.Name
	}
	a.logEvent(e)
}

// hiddenSection: lower Roles do not see it in Settings, so they do not see its values here.
func hiddenSection(id string) bool {
	t, ok := sectionRoles[id]
	return !ok || !t.shown
}

// forReader: es with the values of Hidden sections taken out unless the reader is an
// Administrator. Rows that need no change are shared.
func forReader(es []Sent, admin bool) []Sent {
	if admin {
		return es
	}
	out, cloned := es, false
	for i, e := range es {
		if e.Save == nil {
			continue
		}
		if !cloned {
			out, cloned = slices.Clone(es), true
		}
		s := *e.Save
		s.Fields = slices.Clone(s.Fields)
		for j, f := range s.Fields {
			if hiddenSection(f.SectionID) {
				s.Fields[j] = SavedField{Topic: f.Topic, Section: f.Section, SectionID: f.SectionID, Key: f.Key, Label: f.Label, Changed: true}
			}
		}
		s.HiddenInterruptions, s.Interruptions = len(s.Interruptions), nil
		out[i].Save = &s
	}
	return out
}

// mergeSaves adds to a Communication Log page (newest first, seqs from before-1 down) the
// Settings saves rows anchored in it. A row anchored at k sits between seq k and k-1. With no
// more entries, every older row comes too.
func (a *App) mergeSaves(page []Sent, before int64, more bool) []Sent {
	hi := before
	if hi < 0 {
		hi = 1 << 62
	}
	lo := int64(-1 << 62)
	if more && len(page) > 0 {
		lo = page[len(page)-1].Seq
	}
	saves := a.saves.Between(lo, hi)
	if len(saves) == 0 {
		return page
	}
	out := make([]Sent, 0, len(page)+len(saves))
	i := 0
	for _, e := range page {
		for i < len(saves) && saves[i].Seq > e.Seq {
			out = append(out, saves[i])
			i++
		}
		out = append(out, e)
	}
	return append(out, saves[i:]...)
}

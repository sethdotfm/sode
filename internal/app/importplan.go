package app

import (
	"cmp"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Add import and Replace import (docs/agents/profiles.md): planImport works out what a checked export file does to this
// install, item by item. The check route answers the plan with the preselected choices. The apply route
// plans again under cfgMu with the page's choices and writes that plan.

const (
	importAdd     = "add"
	importReplace = "replace" // empties each ticked category first (importreplace.go)
)

const (
	useNew     = "new"
	useCurrent = "current"
	useSkip    = "skip"
	useRename  = "rename"
)

// importReq is the apply route's plan part. Categories none means every category in the file except
// Routers, which a file brings for its references and which changes the router only when ticked. Choices
// is by category, then by item key.
type importReq struct {
	Mode       string                       `json:"mode"`
	Categories []string                     `json:"categories"`
	Pick       map[string][]string          `json:"pick"`
	Sections   []string                     `json:"sections"`
	Choices    map[string]map[string]string `json:"choices"`
	// KeepAdmins is Keep current Administrators, for Replace import; left out means ticked.
	KeepAdmins *bool `json:"keepAdmins,omitempty"`
	// Routers: file router UID → target UID, "" for none. A file router left out gets its default target.
	Routers map[string]string `json:"routers,omitempty"`
	// Names: remapRef key → the target numbers it moves to. Two are Add both (destinations only); none is Skip.
	Names map[string][]int `json:"names,omitempty"`
	// Filters: remapSet key → the page's text for a set on a moved router, its number and range terms decided.
	Filters map[string]remapFilter `json:"filters,omitempty"`
}

// importItem is one file item. Key is the key in pick and choices. Users, presets and panels use the hidden
// id, filter sets and API clients the name. Settings use the section name, Routers the UID.
type importItem struct {
	Key    string           `json:"key"`
	Name   string           `json:"name"`
	Clash  string           `json:"clash,omitempty"` // the live item's name
	Choice string           `json:"choice,omitempty"`
	Same   bool             `json:"same,omitempty"` // equal to the live item: nothing is written
	As     string           `json:"as,omitempty"`   // the name it imports as, when not Name
	Broken []config.Missing `json:"broken,omitempty"`
	Note   string           `json:"note,omitempty"`
	Kind   string           `json:"kind,omitempty"` // panels: panel, template, checkpointSet; artwork: icon, default
	// Refused: why this item alone is not imported; the others of its category are.
	Refused string `json:"refused,omitempty"`
	// Remove: a live item that Replace import removes, since the file does not hold it.
	Remove bool `json:"remove,omitempty"`
	Size   int  `json:"size,omitempty"` // bytes of the file item before compression, images included
}

func (it importItem) writes() bool {
	return !it.Same && !it.Remove && it.Refused == "" && (it.Clash == "" || it.Choice == useNew || it.Choice == useRename)
}

func (it importItem) overwrites() bool { return it.writes() && it.Clash != "" && it.Choice == useNew }

type importCat struct {
	Items   []importItem `json:"items"`
	Refused string       `json:"refused,omitempty"`
}

type importPlan struct {
	SameInstall bool                  `json:"sameInstall"`
	Critical    bool                  `json:"critical"`
	Categories  map[string]*importCat `json:"categories"`
	// Refused: why the whole import is refused; the apply writes nothing.
	Refused string     `json:"refused,omitempty"`
	Remap   *remapInfo `json:"remap,omitempty"`
}

// plannedPanel is a panel write that waits for the config commit. Live is the panel it replaces, if any.
type plannedPanel struct {
	P      Panel
	FileID string
	Live   *Panel
	Cps    []panelCheckpoint // the file's Push checkpoints, added to the panel's
}

type importPlanner struct {
	live, next config.Config
	b          *exportBundle
	req        importReq
	same       bool
	replace    bool
	keepAdmins bool
	noHashes   bool   // no file user carries a PIN hash, so the importing Administrator stays
	noAdmin    bool   // the Users step would leave no Administrator with a PIN: the whole import is refused
	actor      string // the importing user's id; "" with no users
	secure     bool
	check      *refCheck
	names      namesState
	// waits: with no names at all, the items that hold references, by category and key, to their names.
	waits      map[string]map[string]string
	livePanels []Panel
	removed    importRemovals
	users      map[string]string // file user id → live id, "" when skipped
	presets    map[string]string // file preset id → live id, "" when skipped
	icons      map[string]string // file icon id → live id, "" when skipped
	panelIDs   map[string]string // file panel id → live id; a new panel's is added when it is stored
	panels     []plannedPanel
	// movedDefaults: Default artwork index → where the remap step moves its reference.
	movedDefaults map[int]config.Ref

	liveTemplates  []Template
	liveSets       []PanelSet
	liveIcons      []LibIcon
	liveDefaults   []ArtDefault
	liveIconFile   func(id string) ([]byte, LibIcon, bool)
	iconWrites     []plannedIcon
	defaultWrites  []plannedDefault
	templateWrites []plannedTemplate
	setWrites      []plannedSet

	written    []string // config keys the commit copies from next
	pending    []string // keys the step being planned adds to written when its category is accepted
	overwrites []string // the names of the items Use new writes over: they make the import Critical
	plan       importPlan
}

// src, dst are the live router's names.
func planImport(live config.Config, src, dst map[int]string, lv importLive, b *exportBundle, req importReq, actor string, secure bool) *importPlanner {
	p := &importPlanner{
		live: live, next: live.Clone(), b: b, req: req, actor: actor, secure: secure,
		same:  b.Manifest.Install != "" && b.Manifest.Install == live.InstallUID,
		check: &refCheck{uid: live.UID, levels: live.Levels, src: src, dst: dst, unknown: len(src) == 0 && len(dst) == 0},
		names: lv.Names, livePanels: lv.Panels, users: map[string]string{}, presets: map[string]string{},
		icons: map[string]string{}, panelIDs: map[string]string{},
		liveTemplates: lv.Templates, liveSets: lv.Sets, liveIcons: lv.Icons, liveDefaults: lv.Defaults, liveIconFile: lv.IconFile,
	}
	p.replace = req.Mode == importReplace
	p.keepAdmins = req.KeepAdmins == nil || *req.KeepAdmins
	p.noHashes = !slices.ContainsFunc(b.Users, fileHash)
	p.removed.users = map[string]bool{}
	if p.liveIconFile == nil {
		p.liveIconFile = func(string) ([]byte, LibIcon, bool) { return nil, LibIcon{}, false }
	}
	p.plan = importPlan{SameInstall: p.same, Categories: map[string]*importCat{}}
	p.planRemap(src, dst)
	if p.check.unknown {
		p.waits = p.itemsWithRefs()
	}
	steps := []struct {
		cat  string
		keys []string
		run  func(*config.Config, *importCat) error
	}{
		{catSettings, nil, p.planSettings},
		{catRouters, nil, p.planRouters},
		{catPresets, []string{"presets"}, p.planPresets}, // before Users and API clients, whose Rights map preset ids
		{catUsers, []string{"users"}, p.planUsers},
		{catFilterSets, []string{"filterSets"}, p.planFilterSets},
		{catAPIClients, []string{"apiClients"}, p.planAPIClients},
		{catArtwork, nil, p.planArtwork},
		{catPanels, nil, p.planPanelsAll},
		{catBrowser, nil, p.planBrowser},
	}
	for _, s := range steps {
		if !p.wanted(s.cat) {
			continue
		}
		cat := &importCat{Items: []importItem{}}
		p.plan.Categories[s.cat] = cat
		cand := p.next.Clone()
		p.check.waited, p.pending = false, nil
		err := s.run(&cand, cat)
		if err == nil && p.replace {
			err = p.emptyRest(s.cat, &cand, cat)
		}
		if err == nil && p.check.waited {
			err = errors.New("router names not read yet: try again after the re-read")
		}
		if err == nil && !slices.Contains([]string{catPanels, catArtwork, catBrowser}, s.cat) { // kept outside the config
			cand.Normalize()
			err = validateSave(p.live, cand)
		}
		if err != nil {
			cat.Refused = err.Error()
			p.noAdmin = p.noAdmin || errors.Is(err, errNoAdministrator)
			p.refused(s.cat)
			continue
		}
		for _, key := range slices.Sorted(maps.Keys(p.waits[s.cat])) {
			if picked(p.req.Pick, s.cat, key) {
				it := importItem{Key: key, Name: p.waits[s.cat][key], Refused: waitsForNames}
				if s.cat == catPanels {
					it.Kind = kindPanel
				}
				cat.Items = append(cat.Items, it)
			}
		}
		p.next = cand
		p.check.levels = cand.Levels
		if s.cat == catUsers {
			p.actor = p.adminFor(cand)
		}
		p.written = append(p.written, p.pending...)
		if slices.ContainsFunc(cat.Items, func(it importItem) bool { return it.writes() || it.Remove }) {
			p.written = append(p.written, s.keys...)
		}
	}
	for _, c := range p.plan.Categories {
		if c.Refused != "" {
			continue
		}
		for _, it := range c.Items {
			if it.overwrites() {
				p.overwrites = append(p.overwrites, it.Name)
			}
		}
	}
	slices.Sort(p.overwrites)
	if p.replace && len(p.overwrites) == 0 {
		p.overwrites = []string{"Replace import"}
	}
	p.plan.Critical = p.replace || len(p.overwrites) > 0
	if p.noAdmin {
		p.plan.Refused = errNoAdministrator.Error()
	}
	return p
}

// refused undoes what a refused category would have mapped, so later categories see its items as missing.
func (p *importPlanner) refused(cat string) {
	switch cat {
	case catUsers:
		p.users = map[string]string{}
		p.removed.users = map[string]bool{}
	case catPresets:
		p.presets = map[string]string{}
	case catArtwork:
		p.icons, p.iconWrites, p.defaultWrites = map[string]string{}, nil, nil
		p.removed.icons, p.removed.defaults = nil, nil
	case catPanels:
		p.panels, p.templateWrites, p.setWrites = nil, nil, nil
		p.removed.panels, p.removed.templates, p.removed.sets = nil, nil, nil
	}
}

func (p *importPlanner) wanted(cat string) bool {
	if !slices.Contains(p.b.Manifest.Categories, cat) {
		return false
	}
	if len(p.req.Categories) == 0 {
		return cat != catRouters && cat != catBrowser
	}
	return slices.Contains(p.req.Categories, cat)
}

func (p *importPlanner) picked(cat, key string) bool {
	if _, waits := p.waits[cat][key]; waits {
		return false
	}
	return picked(p.req.Pick, cat, key)
}

const waitsForNames = "it holds router references; import it after Sode reads the router names"

func (p *importPlanner) itemsWithRefs() map[string]map[string]string {
	held := map[string]bool{}
	refWalk(p.b, func(_ string, _ config.Ref, _ refUse, item string) []config.Ref {
		held[item] = true
		return nil
	})
	out := map[string]map[string]string{catUsers: {}, catPresets: {}, catPanels: {}}
	for _, s := range p.b.Presets {
		if held["/"+s.ID] {
			out[catPresets][s.ID] = s.Name
		}
	}
	for _, u := range p.b.Users {
		key := cmp.Or(u.ID, u.Name)
		if held["user/"+key] || slices.ContainsFunc(u.Profile.Presets, func(s config.Preset) bool { return held[u.ID+"/"+s.ID] }) {
			out[catUsers][key] = u.Name
		}
	}
	for _, fp := range p.b.Panels.Panels {
		if held["panel/"+fp.ID] {
			out[catPanels][fp.ID] = fp.Name
		}
	}
	return out
}

// choice is the page's choice for a clashing item, when it is a valid one. Else a file from this install
// gets Use new and a foreign file gets Rename. Replace import always uses the file's item.
func (p *importPlanner) choice(cat, key string, rename bool) string {
	c := p.req.Choices[cat][key]
	switch {
	case p.replace:
		return useNew
	case c == useNew || c == useCurrent || c == useSkip || c == useRename && rename:
		return c
	case p.same || !rename:
		return useNew
	}
	return useRename
}

// configFieldsByYAML: top-level Config fields by yaml key, for copying a Settings section.
func configFieldsByYAML() map[string][]int {
	out := map[string][]int{}
	for _, f := range reflect.VisibleFields(reflect.TypeFor[config.Config]()) {
		if f.Anonymous || len(f.Index) != 1 {
			continue
		}
		if name, _, _ := strings.Cut(f.Tag.Get("yaml"), ","); name != "" && name != "-" {
			out[name] = f.Index
		}
	}
	return out
}

func copyConfigKey(dst, src *config.Config, idx []int) {
	reflect.ValueOf(dst).Elem().FieldByIndex(idx).Set(reflect.ValueOf(src).Elem().FieldByIndex(idx))
}

// overlaySetting copies key k from the file. Of a nested key it takes only the fields the file has or leaves out when
// empty, so an older file keeps the live value of a newer field.
func (p *importPlanner) overlaySetting(dst *config.Config, k string, idx []int) {
	to, from := reflect.ValueOf(dst).Elem().FieldByIndex(idx), reflect.ValueOf(p.b.Settings).Elem().FieldByIndex(idx)
	if to.Kind() != reflect.Struct {
		to.Set(from)
		return
	}
	for i := range to.NumField() {
		name, opts, _ := strings.Cut(to.Type().Field(i).Tag.Get("yaml"), ",")
		if name == "" || p.b.SettingKeys[k+"."+name] || strings.Contains(opts, "omitempty") {
			to.Field(i).Set(from.Field(i))
		}
	}
}

// planSettings copies each file section's keys over live. A section that changes something is a Use new
// clash on global settings; Skip or Use current leaves it.
func (p *importPlanner) planSettings(cand *config.Config, cat *importCat) error {
	if p.b.Settings == nil {
		return nil
	}
	fields := configFieldsByYAML()
	for _, sec := range exportSections {
		if !slices.Contains(p.b.Sections, sec.Name) || len(p.req.Sections) > 0 && !slices.Contains(p.req.Sections, sec.Name) {
			continue
		}
		it := importItem{Key: sec.Name, Name: sectionNames[sec.Name]}
		keys := slices.DeleteFunc(slices.Clone(sec.Keys), func(k string) bool { _, ok := fields[k]; return !ok || !p.b.SettingKeys[k] })
		one := cand.Clone()
		for _, k := range keys {
			p.overlaySetting(&one, k, fields[k])
		}
		one.Normalize()
		changed := slices.ContainsFunc(keys, func(k string) bool {
			return !reflect.DeepEqual(reflect.ValueOf(one).FieldByIndex(fields[k]).Interface(), reflect.ValueOf(*cand).FieldByIndex(fields[k]).Interface())
		})
		if !changed {
			it.Same = true
			cat.Items = append(cat.Items, it)
			continue
		}
		it.Clash, it.Choice = it.Name, p.choice(catSettings, sec.Name, false)
		cat.Items = append(cat.Items, it)
		if it.Choice != useNew {
			continue
		}
		// Require HTTPS set from a connection that is not secure locks this page out (ADR 0005).
		if one.RequireHTTPS && !p.live.RequireHTTPS && !p.secure {
			return errInsecureSave
		}
		for _, k := range keys {
			copyConfigKey(cand, &one, fields[k])
		}
		p.pending = append(p.pending, keys...)
	}
	return nil
}

// planRouters: Sode drives one router, so only the file's router with the live UID applies. Another
// router is for the remap step.
func (p *importPlanner) planRouters(cand *config.Config, cat *importCat) error {
	for i, r := range p.b.Routers {
		it := importItem{Key: r.UID, Name: cmp.Or(r.Title, r.Prefix)}
		// Replace import writes over the one router in place: Sode does not add or delete routers.
		if p.replace && i == 0 {
			r.UID = p.live.UID
		}
		if r.UID != p.live.UID {
			it.Choice, it.Note = useSkip, "another router: Sode does not add routers"
			cat.Items = append(cat.Items, it)
			continue
		}
		r.Extra = p.live.Router.Extra
		it.Clash = cmp.Or(p.live.Title, p.live.Prefix)
		if sameYAML(r, p.live.Router) {
			it.Same = true
		} else {
			it.Choice = p.choice(catRouters, r.UID, false)
		}
		cat.Items = append(cat.Items, it)
		if !it.Same && it.Choice == useNew {
			cand.Router = r
			p.pending = append(p.pending, "router")
		}
	}
	return nil
}

// planUsers: a user with a PIN hash in the file brings it. A new user without one is PIN needed and keeps the
// Role. Use new on a live user keeps that user's PIN.
func (p *importPlanner) planUsers(cand *config.Config, cat *importCat) error {
	for _, u := range p.b.Users {
		key := cmp.Or(u.ID, u.Name)
		if !p.picked(catUsers, key) {
			continue
		}
		fileID := key
		u.Rights = p.mapRightsPresets(u.Rights)
		u.Profile.Presets = p.checkedPresets(u.Profile.Presets)
		for i := range u.Profile.FilterSets {
			u.Profile.FilterSets[i].Broken = p.check.filterSetBroken(u.Profile.FilterSets[i])
		}
		it := importItem{Key: key, Name: u.Name}
		hashed, role := fileHash(u), u.Role
		i := matchUser(p.same, cand.Users, u)
		if i >= 0 {
			lu := cand.Users[i]
			if !hashed {
				u.Pin, u.Len, u.PinNeeded = lu.Pin, lu.Len, lu.PinNeeded
			}
			if p.noHashes && lu.ID == p.actor {
				u.Role = lu.Role // an import without hashes keeps the importing Administrator
			}
			u.ID = lu.ID
			it.Clash = lu.Name
			if sameYAML(u, lu) {
				it.Same = true
				p.users[fileID] = lu.ID
				cat.Items = append(cat.Items, it)
				continue
			}
			it.Choice = p.choice(catUsers, key, true)
			switch it.Choice {
			case useNew:
				cand.Users[i] = u
				if hashed && u.Pin != lu.Pin {
					it.Note = "Ends " + lu.Name + "'s keys and sessions"
				}
				p.users[fileID] = lu.ID
			case useCurrent:
				p.users[fileID] = lu.ID
			case useSkip:
				p.users[fileID] = ""
			case useRename:
				u.Name = nextName(u.Name, func(s string) bool { _, ok := cand.FindUserByName(s); return ok })
				u.Role, it.As = role, u.Name
			}
			if it.Choice != useRename {
				cat.Items = append(cat.Items, it)
				continue
			}
		}
		if _, taken := cand.FindUserByID(u.ID); u.ID == "" || taken || it.Choice == useRename {
			u.ID = config.NewUserID()
		}
		if !hashed {
			u.Pin, u.Len, u.PinNeeded = "", 0, true
		}
		cand.Users = append(cand.Users, u)
		p.users[fileID] = u.ID
		cat.Items = append(cat.Items, it)
	}
	return nil
}

// mapRightsPresets moves preset ids as this import moves the presets. A skipped preset leaves the list.
func (p *importPlanner) mapRightsPresets(r config.Rights) config.Rights {
	if r.Presets == nil {
		return r
	}
	ids := []string{}
	for _, id := range *r.Presets {
		if to, ok := p.presets[id]; ok {
			id = to
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	r.Presets = &ids
	return r
}

func (p *importPlanner) checkedPresets(ps []config.Preset) []config.Preset {
	out := config.ClonePresets(ps)
	for i := range out {
		out[i].Broken = p.check.presetBroken(out[i])
	}
	return out
}

func (p *importPlanner) planPresets(cand *config.Config, cat *importCat) error {
	picked := slices.DeleteFunc(slices.Clone(p.b.Presets), func(s config.Preset) bool { return !p.picked(catPresets, s.ID) })
	for _, s := range p.checkedPresets(picked) {
		fileID := s.ID
		it := importItem{Key: s.ID, Name: s.Name, Broken: s.Broken}
		i := matchPreset(p.same, cand.Presets, s)
		if i >= 0 {
			lp := cand.Presets[i]
			s.ID = lp.ID
			it.Clash = lp.Name
			if sameYAML(s, lp) {
				it.Same = true
				p.presets[fileID] = lp.ID
				cat.Items = append(cat.Items, it)
				continue
			}
			it.Choice = p.choice(catPresets, fileID, true)
			switch it.Choice {
			case useNew:
				cand.Presets[i] = s
				p.presets[fileID] = lp.ID
			case useCurrent:
				p.presets[fileID] = lp.ID
			case useSkip:
				p.presets[fileID] = ""
			case useRename:
				s.Name = nextName(s.Name, func(n string) bool {
					return slices.ContainsFunc(cand.Presets, func(x config.Preset) bool { return strings.EqualFold(x.Name, n) })
				})
				it.As = s.Name
			}
			if it.Choice != useRename {
				cat.Items = append(cat.Items, it)
				continue
			}
		}
		if s.ID == "" || it.Choice == useRename || slices.ContainsFunc(cand.Presets, func(x config.Preset) bool { return x.ID == s.ID }) {
			s.ID = config.NewPresetID()
		}
		cand.Presets = append(cand.Presets, s)
		p.presets[fileID] = s.ID
		cat.Items = append(cat.Items, it)
	}
	return nil
}

func (p *importPlanner) planFilterSets(cand *config.Config, cat *importCat) error {
	for _, s := range p.b.FilterSets {
		if !p.picked(catFilterSets, s.Name) {
			continue
		}
		s.Broken = p.check.filterSetBroken(s)
		it := importItem{Key: s.Name, Name: s.Name, Broken: s.Broken}
		i := matchIdx(false, cand.FilterSets, "", s.Name, nil, func(x config.FilterSet) string { return x.Name })
		if i >= 0 {
			it.Clash = cand.FilterSets[i].Name
			s.Extra = cand.FilterSets[i].Extra
			if sameYAML(s, cand.FilterSets[i]) {
				it.Same = true
				cat.Items = append(cat.Items, it)
				continue
			}
			it.Choice = p.choice(catFilterSets, s.Name, true)
			switch it.Choice {
			case useNew:
				s.Name = cand.FilterSets[i].Name
				cand.FilterSets[i] = s
			case useRename:
				s.Name = nextName(s.Name, func(n string) bool {
					return slices.ContainsFunc(cand.FilterSets, func(x config.FilterSet) bool { return strings.EqualFold(x.Name, n) })
				})
				it.As = s.Name
				cand.FilterSets = append(cand.FilterSets, s)
			}
			cat.Items = append(cat.Items, it)
			continue
		}
		cand.FilterSets = append(cand.FilterSets, s)
		cat.Items = append(cat.Items, it)
	}
	return nil
}

// ownerFor maps a file user id to the user who owns the item here. When the import leaves the owner out,
// or the owner has no PIN, the importing Administrator gets the item. So no item is dropped.
func (p *importPlanner) ownerFor(cfg config.Config, fileID string, needPin bool) string {
	id, ok := p.users[fileID]
	if !ok && p.same {
		id = fileID
	}
	u, found := cfg.FindUserByID(id)
	if !found || needPin && !u.HasPin() {
		return p.actor
	}
	return id
}

// planAPIClients: keys never travel. A panel's own client stays with its panel, which needs a new key link.
func (p *importPlanner) planAPIClients(cand *config.Config, cat *importCat) error {
	for _, c := range p.b.APIClients {
		if !p.picked(catAPIClients, c.Name) {
			continue
		}
		if c.Panel != "" {
			cat.Items = append(cat.Items, importItem{Key: c.Name, Name: c.Name, Refused: "a panel's own client: the panel needs a new key link"})
			continue
		}
		if c.Owner != "" {
			c.Owner = p.ownerFor(*cand, c.Owner, true)
		}
		c.Rights = p.mapRightsPresets(c.Rights)
		it := importItem{Key: c.Name, Name: c.Name}
		i := matchIdx(false, cand.APIClients, "", c.Name, nil, func(x config.APIClient) string { return x.Name })
		if i >= 0 {
			lc := cand.APIClients[i]
			c.Name, c.Extra, c.Networks = lc.Name, lc.Extra, lc.Networks
			it.Clash = lc.Name
			if sameYAML(c, lc) {
				it.Same = true
				cat.Items = append(cat.Items, it)
				continue
			}
			it.Choice = p.choice(catAPIClients, it.Key, true)
			switch it.Choice {
			case useNew:
				cand.APIClients[i] = c
			case useRename:
				c.Name = nextName(c.Name, func(n string) bool { _, ok := cand.FindAPIClient(n); return ok })
				c.Networks = nil
				it.As = c.Name
				cand.APIClients = append(cand.APIClients, c)
			}
			cat.Items = append(cat.Items, it)
			continue
		}
		cand.APIClients = append(cand.APIClients, c)
		cat.Items = append(cat.Items, it)
	}
	return nil
}

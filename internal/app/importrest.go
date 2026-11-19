package app

import (
	"cmp"
	"log"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// The import steps for the stores outside config.yaml. Artwork is the Icon library and Default artwork. Panels
// also holds templates, Checkpoint sets and Push checkpoints. They are written after the config commit, each
// store on its own (docs/agents/profiles.md).

// Item kinds, as exportListItem's.
const (
	kindPanel    = "panel"
	kindTemplate = "template"
	kindSet      = "checkpointSet"
	kindIcon     = "icon"
	kindDefault  = "default"
)

type plannedIcon struct {
	Key  string // the file item's key, for failed
	Icon LibIcon
	Data []byte
}

type plannedDefault struct {
	Key string
	D   ArtDefault
	Img *artImage
}

type plannedTemplate struct {
	Key string
	T   Template
}

type plannedSet struct {
	Key string
	S   PanelSet
}

type importLive struct {
	Panels    []Panel
	Templates []Template
	Sets      []PanelSet
	Icons     []LibIcon
	Defaults  []ArtDefault
	IconFile  func(id string) ([]byte, LibIcon, bool)
	Names     namesState
}

func (a *App) importLive() importLive {
	a.mu.Lock()
	ns := namesState{Offline: a.status.Router != string(swp08.StateUp), SavedAt: a.router.savedAt, NamesAt: a.router.namesAt}
	a.mu.Unlock()
	return importLive{Panels: a.panels.list(), Templates: a.templates.list(), Sets: a.panels.sets.sorted(nil),
		Icons: a.icons.list(), Defaults: a.artDefaults.list(), IconFile: a.icons.file, Names: ns}
}

// libTo is the Icon A or B value s with the icon id mapped as this import moves it. "" drops it: the
// import skipped the icon.
func (p *importPlanner) libTo(s string) string {
	id, ok := strings.CutPrefix(s, "lib:")
	if !ok {
		return s
	}
	to, mapped := p.icons[id]
	switch {
	case !mapped:
		return s
	case to == "":
		return ""
	}
	return "lib:" + to
}

func (p *importPlanner) relibPages(pages []PanelPage) []PanelPage {
	if len(p.icons) == 0 || pages == nil {
		return pages
	}
	out := make([]PanelPage, len(pages))
	for i, g := range pages {
		out[i] = g
		out[i].Cells = make([]map[string]any, len(g.Cells))
		for j, c := range g.Cells {
			c = maps.Clone(c)
			for _, k := range []string{"art", "artB"} {
				if s, ok := c[k].(string); ok {
					if s = p.libTo(s); s == "" {
						delete(c, k)
					} else {
						c[k] = s
					}
				}
			}
			out[i].Cells[j] = c
		}
	}
	return out
}

func (p *importPlanner) relibEdge(e *GridEdge) *GridEdge {
	if e == nil || len(p.icons) == 0 {
		return e
	}
	out := *e
	out.Items = slices.Clone(e.Items)
	for i := range out.Items {
		out.Items[i].Art, out.Items[i].ArtB = p.libTo(out.Items[i].Art), p.libTo(out.Items[i].ArtB)
	}
	return &out
}

func (p *importPlanner) relibLayout(l panelLayout) panelLayout {
	l.Pages = p.relibPages(l.Pages)
	l.Sources, l.Destinations = p.relibEdge(l.Sources), p.relibEdge(l.Destinations)
	return l
}

func (p *importPlanner) artFile(dir, id, typ string) ([]byte, bool) {
	path, ok := p.b.Art[dir+id+"."+artExt[typ]]
	if !ok || artExt[typ] == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if got, err := checkArtBytes(data); err != nil || got != typ {
		return nil, false
	}
	return data, true
}

func (p *importPlanner) panelArt(fileID string, art PanelArt) ([]byte, bool) {
	return p.artFile("art/panels/"+fileID+"/", art.ID, art.Type)
}

// planArtwork plans the Icon library first, so Default artwork, panels and templates follow its id map.
// A file icon matches a live one by id, then by name. Rename gives it a new id.
func (p *importPlanner) planArtwork(_ *config.Config, cat *importCat) error {
	live := p.liveIcons
	taken := map[string]bool{}
	for _, ic := range live {
		taken[ic.ID] = true
	}
	for _, ic := range p.b.Artwork.Icons {
		taken[ic.ID] = true
	}
	total := 0
	for _, ic := range live {
		total += ic.Size
	}
	for _, ic := range p.b.Artwork.Icons {
		if !p.picked(catArtwork, ic.ID) {
			continue
		}
		it := importItem{Key: ic.ID, Name: ic.Name, Kind: kindIcon}
		i := slices.IndexFunc(live, func(x LibIcon) bool { return x.ID == ic.ID })
		if i < 0 {
			i = slices.IndexFunc(live, func(x LibIcon) bool { return strings.EqualFold(x.Name, strings.TrimSpace(ic.Name)) })
		}
		if i >= 0 {
			p.removed.keep(live[i].ID)
		}
		data, ok := p.artFile("art/icons/", ic.ID, ic.Type)
		if !iconIDRe.MatchString(ic.ID) || !ok || ic.Type != "image/png" && ic.Type != "image/svg+xml" {
			it.Refused = "the icon's image is not in the file or is not a PNG or SVG image"
			cat.Items = append(cat.Items, it)
			continue
		}
		ic.Size = len(data)
		if i >= 0 {
			li := live[i]
			it.Clash = li.Name
			cur, _, _ := p.liveIconFile(li.ID)
			if li.Name == ic.Name && string(cur) == string(data) {
				it.Same = true
				p.icons[ic.ID] = li.ID
				cat.Items = append(cat.Items, it)
				continue
			}
			it.Choice = p.choice(catArtwork, ic.ID, true)
			switch it.Choice {
			case useNew:
				p.icons[ic.ID] = li.ID
				total += ic.Size - li.Size
				p.iconWrites = append(p.iconWrites, plannedIcon{ic.ID, LibIcon{ID: li.ID, Name: ic.Name, Type: ic.Type}, data})
			case useCurrent:
				p.icons[ic.ID] = li.ID
			case useSkip:
				p.icons[ic.ID] = ""
			case useRename:
				id := ic.ID
				for taken[id] {
					id = newShortID("")
				}
				taken[id] = true
				it.As = nextName(ic.Name, func(n string) bool {
					return slices.ContainsFunc(live, func(x LibIcon) bool { return strings.EqualFold(x.Name, n) })
				})
				p.icons[ic.ID] = id
				total += ic.Size
				p.iconWrites = append(p.iconWrites, plannedIcon{ic.ID, LibIcon{ID: id, Name: it.As, Type: ic.Type}, data})
			}
			cat.Items = append(cat.Items, it)
			continue
		}
		p.icons[ic.ID] = ic.ID
		total += ic.Size
		p.iconWrites = append(p.iconWrites, plannedIcon{ic.ID, LibIcon{ID: ic.ID, Name: ic.Name, Type: ic.Type}, data})
		cat.Items = append(cat.Items, it)
	}
	if total > iconLibTotal {
		return errIconLibCap
	}
	return p.planDefaults(cat)
}

// planDefaults: a Preset default follows its preset through the preset map. A Source or Destination default
// on a router that the remap step leaves without a target is refused.
func (p *importPlanner) planDefaults(cat *importCat) error {
	images := map[string]PanelArt{}
	for _, im := range p.b.Artwork.DefaultImages {
		images[im.ID] = im
	}
	for i, d := range p.b.Artwork.Defaults {
		key := d.key()
		if !p.picked(catArtwork, key) {
			continue
		}
		it := importItem{Key: key, Name: d.Name, Kind: kindDefault}
		switch d.Kind {
		case artKindPreset:
			to, mapped := p.presets[d.ID]
			if !mapped && p.same {
				to = d.ID
			}
			if !slices.ContainsFunc(p.next.Presets, func(x config.Preset) bool { return to != "" && x.ID == to }) {
				it.Refused = "its preset is not here"
			}
			d.ID = to
		default:
			if x, ok := p.movedDefaults[i]; ok {
				n := x.N
				d.Router, d.N, d.Name = x.Router, &n, cmp.Or(x.Name, d.Name)
			}
			if d.Router != p.live.UID {
				it.Refused = "it is for another router"
			}
		}
		d.Art = p.libTo(d.Art)
		var img *artImage
		if artDefaultIDRe.MatchString(d.Art) {
			im := images[d.Art]
			data, ok := p.artFile("art/defaults/", d.Art, im.Type)
			if !ok {
				it.Refused = cmp.Or(it.Refused, "its image is not in the file")
			}
			img = &artImage{data, im.Type}
		}
		if it.Refused == "" && (d.check() != nil || !storedArtOK(d)) {
			it.Refused = "it is not valid"
		}
		i := slices.IndexFunc(p.liveDefaults, func(x ArtDefault) bool { return x.key() == d.key() })
		if i >= 0 {
			p.removed.keep(p.liveDefaults[i].key())
		}
		if it.Refused != "" {
			cat.Items = append(cat.Items, it)
			continue
		}
		if i >= 0 {
			it.Clash = p.liveDefaults[i].Name
			if sameYAML(d, p.liveDefaults[i]) {
				it.Same = true
				cat.Items = append(cat.Items, it)
				continue
			}
			if it.Choice = p.choice(catArtwork, key, false); it.Choice != useNew {
				cat.Items = append(cat.Items, it)
				continue
			}
		}
		p.defaultWrites = append(p.defaultWrites, plannedDefault{key, d, img})
		cat.Items = append(cat.Items, it)
	}
	return nil
}

func (p *importPlanner) planPanelsAll(cand *config.Config, cat *importCat) error {
	if err := p.planPanels(cand, cat); err != nil {
		return err
	}
	if err := p.planTemplates(cand, cat); err != nil {
		return err
	}
	return p.planSets(cand, cat)
}

// planBrowser: This browser's values are the page's, which applies them itself from the check's answer.
func (p *importPlanner) planBrowser(_ *config.Config, cat *importCat) error {
	if p.b.Browser != nil && p.picked(catBrowser, catBrowser) {
		cat.Items = append(cat.Items, importItem{Key: catBrowser, Name: "This browser", Note: "This browser applies it"})
	}
	return nil
}

func (p *importPlanner) planCheckpoints(fileID string) []panelCheckpoint {
	var out []panelCheckpoint
	for _, cp := range p.b.Panels.Checkpoints[fileID] {
		if cp.Layout.noRouterProblem() != nil {
			continue
		}
		cp.Layout = p.relibLayout(cp.Layout)
		out = append(out, cp)
	}
	return out
}

// planTemplates: a file from this install matches by id, else by name where the template is listed (its
// scope, and for Me its owner).
func (p *importPlanner) planTemplates(cand *config.Config, cat *importCat) error {
	adds := 0
	for _, t := range p.b.Panels.Templates {
		if !p.picked(catPanels, t.ID) {
			continue
		}
		fileID := t.ID
		t.Owner = p.ownerFor(*cand, t.Owner, false)
		t.Pages = p.relibPages(t.Pages)
		it := importItem{Key: fileID, Name: t.Name, Kind: kindTemplate}
		err := t.clean()
		i := -1
		if p.same {
			i = slices.IndexFunc(p.liveTemplates, func(x Template) bool { return x.ID == fileID })
		}
		if i < 0 {
			i = slices.IndexFunc(p.liveTemplates, func(x Template) bool { return sameBucket(x, t) && strings.EqualFold(x.Name, t.Name) })
		}
		if i >= 0 {
			p.removed.keep(p.liveTemplates[i].ID)
		}
		if err != nil {
			it.Refused = err.Error()
			cat.Items = append(cat.Items, it)
			continue
		}
		if !validTemplateID(t.ID) {
			t.ID = ""
		}
		if i >= 0 {
			lt := p.liveTemplates[i]
			t.ID, it.Clash = lt.ID, lt.Name
			if sameYAML(t, lt) {
				it.Same = true
				cat.Items = append(cat.Items, it)
				continue
			}
			it.Choice = p.choice(catPanels, fileID, true)
			switch it.Choice {
			case useNew:
				p.templateWrites = append(p.templateWrites, plannedTemplate{fileID, t})
			case useRename:
				t.ID = ""
				t.Name = nextName(t.Name, func(n string) bool {
					return slices.ContainsFunc(p.liveTemplates, func(x Template) bool { return sameBucket(x, t) && strings.EqualFold(x.Name, n) })
				})
				it.As = t.Name
				adds++
				p.templateWrites = append(p.templateWrites, plannedTemplate{fileID, t})
			}
			cat.Items = append(cat.Items, it)
			continue
		}
		if slices.ContainsFunc(p.liveTemplates, func(x Template) bool { return x.ID == t.ID }) {
			t.ID = ""
		}
		adds++
		p.templateWrites = append(p.templateWrites, plannedTemplate{fileID, t})
		cat.Items = append(cat.Items, it)
	}
	if len(p.liveTemplates)+adds > templateMax {
		return errTemplateCap
	}
	return nil
}

// planSets: entries name panels by id, which the write maps after the panels are stored.
func (p *importPlanner) planSets(cand *config.Config, cat *importCat) error {
	adds := 0
	for _, s := range p.b.Panels.CheckpointSets {
		if !p.picked(catPanels, s.ID) {
			continue
		}
		fileID := s.ID
		s.Owner = p.ownerFor(*cand, s.Owner, false)
		s.Entries = slices.Clone(s.Entries)
		for i := range s.Entries {
			s.Entries[i].panelLayout = p.relibLayout(s.Entries[i].panelLayout)
		}
		it := importItem{Key: fileID, Name: s.Name, Kind: kindSet}
		bucket := func(x PanelSet) bool { return x.Scope == s.Scope && (s.Scope == panelShareAll || x.Owner == s.Owner) }
		i := -1
		if p.same {
			i = slices.IndexFunc(p.liveSets, func(x PanelSet) bool { return x.ID == fileID })
		}
		if i < 0 {
			i = slices.IndexFunc(p.liveSets, func(x PanelSet) bool { return bucket(x) && strings.EqualFold(x.Name, s.Name) })
		}
		if i >= 0 {
			p.removed.keep(p.liveSets[i].ID)
		}
		if s.Scope != panelShareAll && s.Scope != panelShareMe || strings.TrimSpace(s.Name) == "" {
			it.Refused = "it is not valid"
			cat.Items = append(cat.Items, it)
			continue
		}
		if i >= 0 {
			ls := p.liveSets[i]
			s.ID, it.Clash = ls.ID, ls.Name
			if sameYAML(s, ls) {
				it.Same = true
				cat.Items = append(cat.Items, it)
				continue
			}
			it.Choice = p.choice(catPanels, fileID, true)
			switch it.Choice {
			case useNew:
				p.setWrites = append(p.setWrites, plannedSet{fileID, s})
			case useRename:
				s.ID = ""
				s.Name = nextName(s.Name, func(n string) bool {
					return slices.ContainsFunc(p.liveSets, func(x PanelSet) bool { return bucket(x) && strings.EqualFold(x.Name, n) })
				})
				it.As = s.Name
				adds++
				p.setWrites = append(p.setWrites, plannedSet{fileID, s})
			}
			cat.Items = append(cat.Items, it)
			continue
		}
		if slices.ContainsFunc(p.liveSets, func(x PanelSet) bool { return x.ID == s.ID }) || !strings.HasPrefix(s.ID, "s-") {
			s.ID = ""
		}
		adds++
		p.setWrites = append(p.setWrites, plannedSet{fileID, s})
		cat.Items = append(cat.Items, it)
	}
	if len(p.liveSets)+adds > panelSetMax {
		return errSetCap
	}
	return nil
}

// writeImportArtwork stores the planned icons, then the Default artwork that can name them.
func (a *App) writeImportArtwork(p *importPlanner, failed map[string]string) {
	for _, w := range p.iconWrites {
		if _, err := a.icons.put(w.Icon, w.Data); err != nil {
			failed[w.Key] = err.Error()
			log.Printf("import: icon %s: %v", w.Icon.Name, err)
		}
	}
	for _, w := range p.defaultWrites {
		if _, err := a.artDefaults.set(w.D, w.Img); err != nil {
			failed[w.Key] = err.Error()
			log.Printf("import: Default artwork %s: %v", w.D.Name, err)
		}
	}
}

// writeImportPanelExtras stores the templates and sets after the panels are stored, so set entries name
// the panels' ids here.
func (a *App) writeImportPanelExtras(p *importPlanner, failed map[string]string) {
	for _, w := range p.templateWrites {
		if _, err := a.templates.put(w.T); err != nil {
			failed[w.Key] = err.Error()
			log.Printf("import: template %s: %v", w.T.Name, err)
		}
	}
	for _, w := range p.setWrites {
		s := w.S
		for i := range s.Entries {
			if to, ok := p.panelIDs[s.Entries[i].Panel]; ok {
				s.Entries[i].Panel = to
			}
		}
		if _, err := a.panels.sets.put(s); err != nil {
			failed[w.Key] = err.Error()
			log.Printf("import: Checkpoint set %s: %v", s.Name, err)
		}
	}
}

// keepLivePanel finds fp's live twin and keeps it from Replace import's removals.
func (p *importPlanner) keepLivePanel(fp Panel) int {
	i := matchIdx(p.same, p.livePanels, fp.ID, fp.Name, func(x Panel) string { return x.ID }, func(x Panel) string { return x.Name })
	if i >= 0 {
		p.removed.keep(p.livePanels[i].ID)
	}
	return i
}

// planPanels plans the panel writes; they happen after the config commit, so a preset the same import
// adds is found. A new panel has no key link; Use new keeps the live panel's Authentication mode and key link.
func (p *importPlanner) planPanels(cand *config.Config, cat *importCat) error {
	adds := 0
	slugTaken := func(fp Panel, self string) bool {
		return fp.Slug != "" && slices.ContainsFunc(p.livePanels, func(x Panel) bool { return x.Slug == fp.Slug && x.ID != self })
	}
	for _, fp := range p.b.Panels.Panels {
		if !p.picked(catPanels, fp.ID) {
			continue
		}
		fileID := fp.ID
		fp.Owner = p.ownerFor(*cand, fp.Owner, false)
		fp.Pages = p.relibPages(fp.Pages)
		fp.Sources, fp.Destinations = p.relibEdge(fp.Sources), p.relibEdge(fp.Destinations)
		fp.Broken = p.panelBroken(&fp, *cand)
		fp.Rev, fp.Client = 0, ""
		fp.Share.Users = p.shareNames(*cand, fp.Share.Users)
		it := importItem{Key: fileID, Name: fp.Name, Broken: fp.Broken, Kind: kindPanel}
		cps := p.planCheckpoints(fileID)
		i := p.keepLivePanel(fp)
		if err := validatePanel(&fp); err != nil {
			it.Refused = err.Error()
		} else if fp.Acts == panelActsPublic && !cand.PublicPanels {
			it.Refused = errPublicOff.Error()
		} else if slices.ContainsFunc(fp.Art, func(art PanelArt) bool { _, ok := p.panelArt(fileID, art); return !ok }) {
			it.Refused = "an artwork image is not in the file or is not the image its type says"
		}
		if it.Refused != "" {
			cat.Items = append(cat.Items, it)
			continue
		}
		if i >= 0 {
			lp := p.livePanels[i]
			fp.ID, fp.Acts, fp.Client, fp.Rev = lp.ID, lp.Acts, lp.Client, lp.Rev
			it.Clash = lp.Name
			if sameYAML(fp, lp) {
				it.Same = true
				p.panelIDs[fileID] = lp.ID
				cat.Items = append(cat.Items, it)
				continue
			}
			it.Choice = p.choice(catPanels, fileID, true)
			switch it.Choice {
			case useNew:
				if slugTaken(fp, lp.ID) {
					fp.Slug = lp.Slug
				}
				fp.Rev = lp.Rev + 1
				p.panelIDs[fileID] = lp.ID
				p.panels = append(p.panels, plannedPanel{P: fp, FileID: fileID, Live: &lp, Cps: cps})
			case useCurrent:
				p.panelIDs[fileID] = lp.ID
			case useRename:
				fp.Name = nextName(fp.Name, func(n string) bool {
					return slices.ContainsFunc(p.livePanels, func(x Panel) bool { return strings.EqualFold(x.Name, n) })
				})
				it.As = fp.Name
			}
			if it.Choice != useRename {
				cat.Items = append(cat.Items, it)
				continue
			}
		}
		fp.Rev = 0
		if it.Choice == useRename || slices.ContainsFunc(p.livePanels, func(x Panel) bool { return x.ID == fp.ID }) {
			fp.ID = "" // createAs keeps a free id, so a link to this panel still opens it
		}
		if slugTaken(fp, "") {
			fp.Slug = ""
		}
		if actsHasClient(fp.Acts) {
			it.Note = "needs a new key link"
		}
		adds++
		p.panels = append(p.panels, plannedPanel{P: fp, FileID: fileID, Cps: cps})
		cat.Items = append(cat.Items, it)
	}
	if len(p.livePanels)+adds > panelMax && !p.replace { // Replace counts after its removals (emptyRest)
		return errPanelCap
	}
	return nil
}

// shareNames maps the user names a panel is shared with through the users this import maps. A name from
// a foreign file that the import does not map is left out, since here it can be another person.
func (p *importPlanner) shareNames(cfg config.Config, names []string) []string {
	var out []string
	for _, n := range names {
		i := slices.IndexFunc(p.b.Users, func(u config.User) bool { return strings.EqualFold(u.Name, n) })
		if i >= 0 {
			if id, ok := p.users[cmp.Or(p.b.Users[i].ID, p.b.Users[i].Name)]; ok {
				if u, found := cfg.FindUserByID(id); found {
					out = append(out, u.Name)
				}
				continue
			}
		}
		if _, ok := cfg.FindUserByName(n); ok && p.same {
			out = append(out, n)
		}
	}
	return out
}

// panelBroken points preset buttons at the presets this import maps them to and lists what the target lacks.
func (p *importPlanner) panelBroken(fp *Panel, cfg config.Config) []config.Missing {
	var b brokenList
	layoutOf(*fp).readRefs(func(s refSlot, kind string, r config.Ref) {
		if s != slotFixed {
			b.add(p.check.missing(kind, r))
		}
	})
	pages := make([]PanelPage, len(fp.Pages))
	for i, g := range fp.Pages {
		pages[i] = g
		pages[i].Cells = slices.Clone(g.Cells)
		for j, c := range g.Cells {
			if c["kind"] != "preset" || c["scope"] == "me" {
				continue
			}
			id, _ := c["id"].(string)
			name, _ := c["name"].(string)
			to, mapped := p.presets[id]
			if !mapped && p.same {
				to = id
			}
			if !slices.ContainsFunc(cfg.Presets, func(x config.Preset) bool { return to != "" && x.ID == to }) {
				b.add(config.Missing{Kind: config.MissingPreset, Ref: config.Ref{Name: name}}, id != "" || name != "")
				continue
			}
			pages[i].Cells[j] = maps.Clone(c)
			pages[i].Cells[j]["id"] = to
		}
	}
	fp.Pages = pages
	return b.out
}

// A cell with no number is unassigned.
func cellRef(c map[string]any) (config.Ref, bool) {
	var n int
	switch x := c["n"].(type) {
	case int:
		n = x
	case float64:
		n = int(x)
	default:
		return config.Ref{}, false
	}
	r, _ := c["router"].(string)
	name, _ := c["name"].(string)
	return config.Ref{Router: r, N: n, Name: name}, true
}

package app

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"reflect"
	"slices"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// The export file: one zip of YAML files and plain image files (docs/agents/profiles.md). exportFormat is
// the file's own version, apart from config.Format. A change that an older Sode would misread bumps it
// and adds a loader to exportLoaders (exportload.go) with its own fixture.
const exportFormat = 1

const (
	catSettings   = "settings"
	catRouters    = "routers"
	catUsers      = "users"
	catPresets    = "presets"
	catFilterSets = "filterSets"
	catPanels     = "panels"
	catArtwork    = "artwork"
	catAPIClients = "apiClients"
	catBrowser    = "browser"
)

// exportCategories in file order, each with its entry name.
var exportCategories = []struct{ Name, File string }{
	{catSettings, "settings.yaml"},
	{catRouters, "routers.yaml"},
	{catUsers, "users.yaml"},
	{catPresets, "presets.yaml"},
	{catFilterSets, "filtersets.yaml"},
	{catPanels, "panels.yaml"},
	{catArtwork, "artwork.yaml"},
	{catAPIClients, "apiclients.yaml"},
	{catBrowser, "browser.yaml"},
}

func categoryFile(cat string) string {
	for _, c := range exportCategories {
		if c.Name == cat {
			return c.File
		}
	}
	return ""
}

// exportSections are the Settings sections with the top-level config keys each holds. The names are the
// data-section names in web/index.html. Router fields are in routers.yaml, not here.
type exportSection struct {
	Name string
	Keys []string
}

var exportSections = []exportSection{
	{"everyone-defaults", []string{"browserDefaults"}},
	{"branding", []string{"brandText", "brandBorder"}},
	{"history", []string{"hiddenDsts", "hiddenDstsRouter"}},
	{"level-colors", []string{"levelColors"}},
	{"confirm", []string{"confirmLockoutMs", "resultDismissSeconds", "confirmEnter"}},
	{"locks", []string{"locks", "autoLockMinutes", "guestRole", "pinWaits", "maxPinLen"}},
	{"users", []string{"newUserDefaults"}},
	{"api", []string{"api", "guestRights", "guestApiScopes"}},
	{"panels", []string{"publicPanels", "newPanelActs"}},
	{"proxy", []string{"trustedProxies", "requireHttps"}},
}

// exportNotSettings: top-level config keys in another category or in no export. settingsPin is a hash
// that this release removes.
var exportNotSettings = []string{"format", "installUid", "users", "presets", "filterSets", "apiClients", "settingsPin"}

type exportManifest struct {
	Format     int            `yaml:"format"`
	Sode       string         `yaml:"sode"`
	Install    string         `yaml:"install"`
	Made       time.Time      `yaml:"made"`
	Categories []string       `yaml:"categories"`
	Routers    []exportRouter `yaml:"routers"`
}

// exportRouter is what the remap step matches a file router on. The prefix is a label: items write it, and
// this list maps it to the UID.
type exportRouter struct {
	UID            string `yaml:"uid"`
	Prefix         string `yaml:"prefix"`
	Title          string `yaml:"title"`
	Matrix         int    `yaml:"matrix"`
	Levels         int    `yaml:"levels"`
	LevelOffset    int    `yaml:"levelOffset"`
	LevelNumbering string `yaml:"levelNumbering"`
	NamesHash      string `yaml:"namesHash"`
}

type exportRouters struct {
	Routers []config.Router `yaml:"routers"`
}

type exportUsers struct {
	Users []config.User `yaml:"users"`
}

type exportPresets struct {
	Presets []config.Preset `yaml:"presets"`
}

type exportFilterSets struct {
	FilterSets []config.FilterSet `yaml:"filterSets"`
}

type exportAPIClients struct {
	APIClients []config.APIClient `yaml:"apiClients"`
}

// exportPanels: Checkpoints is by panel id. Each panel's own artwork is under art/panels/<id>/.
type exportPanels struct {
	Panels         []Panel                      `yaml:"panels,omitempty"`
	Checkpoints    map[string][]panelCheckpoint `yaml:"checkpoints,omitempty"`
	Templates      []Template                   `yaml:"templates,omitempty"`
	CheckpointSets []PanelSet                   `yaml:"checkpointSets,omitempty"`
}

// exportArtwork: Default artwork images are under art/defaults/, Icon library files under art/icons/.
type exportArtwork struct {
	Defaults      []ArtDefault `yaml:"defaults,omitempty"`
	DefaultImages []PanelArt   `yaml:"defaultImages,omitempty"`
	Icons         []LibIcon    `yaml:"icons,omitempty"`
}

// exportReq is the export route's body. A category in Pick exports only the ids listed. A category not in
// Pick exports whole. Users and presets are picked by hidden id, filter sets and API clients by name.
// Panels are picked by panel, template or Checkpoint set id. Artwork is picked by icon id or Default artwork key.
type exportReq struct {
	Categories  []string            `json:"categories"`
	Pick        map[string][]string `json:"pick"`
	Sections    []string            `json:"sections"` // Settings sections; none is every section
	IncludePins bool                `json:"includePins"`
	Browser     json.RawMessage     `json:"browser"` // This browser's values, an object the page sends
}

var artExt = map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/gif": "gif", "image/svg+xml": "svg"}

type zipFile struct {
	name string
	data []byte
}

// buildExport writes the zip for req. Items name routers by prefix, so every UID is only in manifest.yaml,
// routers.yaml and the names/ file names.
func (a *App) buildExport(req exportReq) ([]byte, error) {
	cfg := a.r().cfg
	for _, c := range req.Categories {
		if categoryFile(c) == "" {
			return nil, fmt.Errorf("no export category %q", c)
		}
	}
	for c := range req.Pick {
		if categoryFile(c) == "" {
			return nil, fmt.Errorf("no export category %q", c)
		}
	}
	want := func(c string) bool { return slices.Contains(req.Categories, c) }
	picked := func(c, id string) bool {
		ids, some := req.Pick[c]
		return !some || slices.Contains(ids, id)
	}
	routers := []config.Router{cfg.Router}
	prefix := map[string]string{}
	for _, r := range routers {
		prefix[r.UID] = r.Prefix
	}
	used := map[string]bool{}
	var files []zipFile
	add := func(cat string, v any) error {
		b, err := exportYAML(v, prefix, len(routers) == 1, used)
		if err == nil {
			files = append(files, zipFile{categoryFile(cat), b})
		}
		return err
	}

	if want(catSettings) {
		node, err := exportSettings(cfg, req.Sections)
		if err != nil {
			return nil, err
		}
		if err := add(catSettings, node); err != nil {
			return nil, err
		}
	}
	if want(catUsers) {
		var out exportUsers
		for _, u := range cfg.Users {
			if !picked(catUsers, u.ID) {
				continue
			}
			if !req.IncludePins {
				u.Pin, u.Len = "", 0
			}
			out.Users = append(out.Users, u)
		}
		if err := add(catUsers, &out); err != nil {
			return nil, err
		}
	}
	if want(catPresets) {
		var out exportPresets
		for _, p := range cfg.Presets {
			if picked(catPresets, p.ID) {
				out.Presets = append(out.Presets, p)
			}
		}
		if err := add(catPresets, &out); err != nil {
			return nil, err
		}
	}
	if want(catFilterSets) {
		var out exportFilterSets
		for _, s := range cfg.FilterSets {
			if picked(catFilterSets, s.Name) {
				out.FilterSets = append(out.FilterSets, s)
			}
		}
		if err := add(catFilterSets, &out); err != nil {
			return nil, err
		}
	}
	if want(catPanels) {
		out := exportPanels{Checkpoints: map[string][]panelCheckpoint{}}
		for _, p := range a.panels.list() {
			if !picked(catPanels, p.ID) {
				continue
			}
			p.Rev = 0
			for _, art := range p.Art {
				b, meta, ok := a.panels.artData(p.ID, art.ID)
				if ext := artExt[meta.Type]; ok && ext != "" {
					files = append(files, zipFile{"art/panels/" + p.ID + "/" + art.ID + "." + ext, b})
				}
			}
			out.Panels = append(out.Panels, p)
			if cps, _ := a.panels.checkpoints(p.ID); len(cps) > 0 {
				out.Checkpoints[p.ID] = cps
			}
		}
		for _, t := range a.templates.list() {
			if picked(catPanels, t.ID) {
				out.Templates = append(out.Templates, t)
			}
		}
		for _, s := range a.panels.sets.sorted(nil) {
			if picked(catPanels, s.ID) {
				out.CheckpointSets = append(out.CheckpointSets, s)
			}
		}
		if err := add(catPanels, &out); err != nil {
			return nil, err
		}
	}
	if want(catArtwork) {
		var out exportArtwork
		images := map[string]bool{}
		for _, d := range a.artDefaults.list() {
			if !picked(catArtwork, d.key()) {
				continue
			}
			out.Defaults = append(out.Defaults, d)
			if artDefaultIDRe.MatchString(d.Art) && !images[d.Art] {
				images[d.Art] = true
				b, meta, ok := a.artDefaults.image(d.Art)
				if ext := artExt[meta.Type]; ok && ext != "" {
					out.DefaultImages = append(out.DefaultImages, meta)
					files = append(files, zipFile{"art/defaults/" + meta.ID + "." + ext, b})
				}
			}
		}
		for _, ic := range a.icons.list() {
			if !picked(catArtwork, ic.ID) {
				continue
			}
			if b, ic, ok := a.icons.file(ic.ID); ok {
				out.Icons = append(out.Icons, ic)
				files = append(files, zipFile{"art/icons/" + ic.fileName(), b})
			}
		}
		if err := add(catArtwork, &out); err != nil {
			return nil, err
		}
	}
	if want(catAPIClients) {
		var out exportAPIClients
		for _, c := range cfg.APIClients {
			if picked(catAPIClients, c.Name) {
				out.APIClients = append(out.APIClients, c)
			}
		}
		if err := add(catAPIClients, &out); err != nil {
			return nil, err
		}
	}
	if want(catBrowser) {
		var values map[string]any
		if len(req.Browser) > 0 {
			if err := json.Unmarshal(req.Browser, &values); err != nil {
				return nil, errors.New("this browser's values must be a JSON object")
			}
		}
		if values == nil {
			values = map[string]any{}
		}
		if err := add(catBrowser, values); err != nil {
			return nil, err
		}
	}

	// A router goes with every export that names it, ticked or not.
	man := exportManifest{Format: exportFormat, Sode: BuildLabel(), Install: cfg.InstallUID, Made: time.Now().UTC().Truncate(time.Second)}
	var withRouters []config.Router
	for _, r := range routers {
		if want(catRouters) || used[r.UID] {
			withRouters = append(withRouters, r)
		}
	}
	if len(withRouters) > 0 {
		out := exportRouters{Routers: withRouters}
		for i := range out.Routers {
			out.Routers[i].Extra = nil
		}
		if err := add(catRouters, &out); err != nil {
			return nil, err
		}
		for _, r := range withRouters {
			table := a.liveNamesTable(r.UID)
			b, err := yaml.Marshal(table)
			if err != nil {
				return nil, err
			}
			files = append(files, zipFile{"names/" + r.UID + ".yaml", b})
			man.Routers = append(man.Routers, exportRouter{
				UID: r.UID, Prefix: r.Prefix, Title: r.Title, Matrix: r.Matrix, Levels: r.Levels,
				LevelOffset: r.LevelWireOffset, LevelNumbering: r.LevelNumbering, NamesHash: namesHash(table),
			})
		}
	}
	for _, c := range exportCategories {
		if slices.ContainsFunc(files, func(f zipFile) bool { return f.name == c.File }) {
			man.Categories = append(man.Categories, c.Name)
		}
	}
	mb, err := yaml.Marshal(man)
	if err != nil {
		return nil, err
	}
	zb, err := writeZip(append([]zipFile{{"manifest.yaml", mb}}, files...))
	if err == nil && int64(len(zb)) > exportMaxUpload {
		return nil, fmt.Errorf("the export is over %d MiB, which an import does not accept; pick fewer items", exportMaxUpload>>20)
	}
	return zb, err
}

func writeZip(files []zipFile) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		method := zip.Deflate
		if ext := path.Ext(f.name); ext == ".png" || ext == ".jpg" || ext == ".gif" {
			method = zip.Store // already compressed
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.name, Method: method, Modified: time.Now()})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// exportSettings is the Settings sections asked for (none is all), each a mapping of its config keys.
func exportSettings(cfg config.Config, sections []string) (*yaml.Node, error) {
	for _, s := range sections {
		if !slices.ContainsFunc(exportSections, func(x exportSection) bool { return x.Name == s }) {
			return nil, fmt.Errorf("no Settings section %q", s)
		}
	}
	cfg.Extra = nil
	var top yaml.Node
	if err := top.Encode(cfg); err != nil {
		return nil, err
	}
	values := map[string]*yaml.Node{}
	for i := 0; i+1 < len(top.Content); i += 2 {
		values[top.Content[i].Value] = top.Content[i+1]
	}
	out := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, s := range exportSections {
		if len(sections) > 0 && !slices.Contains(sections, s.Name) {
			continue
		}
		sec := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, k := range s.Keys {
			if v, ok := values[k]; ok {
				sec.Content = append(sec.Content, strNode(k), v)
			}
		}
		out.Content = append(out.Content, strNode(s.Name), sec)
	}
	return out, nil
}

func strNode(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }

// exportYAML encodes v with every key this release does not know dropped and router UIDs written as prefixes. used gets each
// router v names.
func exportYAML(v any, prefix map[string]string, one bool, used map[string]bool) ([]byte, error) {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return nil, err
	}
	if sec, ok := v.(*yaml.Node); ok {
		pruneSettings(sec, nil)
		n = *sec
	} else {
		pruneUnknown(&n, reflect.TypeOf(v), "", nil)
	}
	toPrefixes(&n, prefix, one, used)
	return yaml.Marshal(&n)
}

// exportRouterKeys hold a router UID in Sode's stores.
var exportRouterKeys = map[string]bool{"router": true, "hiddenDstsRouter": true}

// toPrefixes writes each router UID as its prefix. With one router a reference (a mapping with n) leaves it
// out. A filter set's router and hiddenDstsRouter keep the prefix, since blank means "no router" there.
func toPrefixes(n *yaml.Node, prefix map[string]string, one bool, used map[string]bool) {
	if n.Kind != yaml.MappingNode {
		for _, c := range n.Content {
			toPrefixes(c, prefix, one, used)
		}
		return
	}
	isRef := false
	for i := 0; i+1 < len(n.Content); i += 2 {
		isRef = isRef || n.Content[i].Value == "n"
	}
	for i := 0; i+1 < len(n.Content); {
		k, v := n.Content[i], n.Content[i+1]
		if exportRouterKeys[k.Value] && v.Kind == yaml.ScalarNode {
			if p, ok := prefix[v.Value]; ok {
				used[v.Value] = true
				if one && isRef {
					n.Content = slices.Delete(n.Content, i, i+2)
					continue
				}
				v.Value = p
			}
		}
		toPrefixes(v, prefix, one, used)
		i += 2
	}
}

// liveNamesTable is the router's names as Sode has them now: read from the router, or the saved table
// until the first read.
func (a *App) liveNamesTable(uid string) namesTable {
	a.mu.Lock()
	defer a.mu.Unlock()
	rs := a.router
	return namesTable{Router: uid, At: rs.namesAt.UTC(), Sources: tableList(rs.srcNames), Destinations: tableList(rs.dstNames)}
}

// namesHash depends on the names only, not on when they were read, so the remap step can match a router
// whose UID differs.
func namesHash(t namesTable) string {
	b, _ := yaml.Marshal(struct {
		Sources      []namedItem `yaml:"sources"`
		Destinations []namedItem `yaml:"destinations"`
	}{t.Sources, t.Destinations})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:16])
}

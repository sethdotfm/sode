package app

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// Caps for an uploaded export file. Vars so tests can make them small. One entry can be as large as one
// panel's or one store's artwork total.
var (
	exportMaxUpload  int64 = 512 << 20
	exportMaxEntries       = 20000
	exportMaxEntry   int64 = panelArtTotal
	// Images do not compress, so a real file unpacks to about its zip size; this stops a zip bomb.
	exportMaxTotal int64 = 2 * exportMaxUpload
)

// exportEntryRe is every entry name a file can hold: the fixed names and the art and names directories, built on
// the stores' own id forms. Anything else, such as "..", a backslash or an absolute path, is refused.
var exportEntryRe = regexp.MustCompile(`^(?:manifest\.yaml` +
	`|(?:settings|routers|users|presets|filtersets|bookmarks|panels|artwork|apiclients|browser)\.yaml` +
	`|names/[A-Za-z0-9_-]{1,64}\.yaml` +
	`|art/panels/p-[0-9a-f]{8}/a-[0-9a-f]{8}\.(?:png|jpg|gif|svg)` +
	`|art/defaults/d-[0-9a-f]{8}\.(?:png|jpg|gif|svg)` +
	`|art/icons/[0-9a-f]{8}\.(?:png|svg)` +
	`|(?:names|art|art/panels|art/defaults|art/icons|art/panels/p-[0-9a-f]{8})/)$`)

type exportErr struct {
	status int
	code   string
	msg    string
}

func (e *exportErr) Error() string { return e.msg }

func badExport(format string, args ...any) error {
	return &exportErr{400, "bad_file", fmt.Sprintf(format, args...)}
}

// exportBundle is a checked export file. Routers in items are UIDs again; a reference with no router in a
// file with one router names that router. Art maps each art entry name to its file in the staging directory.
type exportBundle struct {
	Manifest exportManifest
	Unknown  []string // keys this release does not know, already dropped
	Settings *config.Config
	Sections []string
	// SettingKeys: the config keys the file sets, and "key.sub" for a nested one. An omitempty key counts, since the
	// export leaves it out when empty.
	SettingKeys map[string]bool
	Routers     []config.Router
	Names       map[string]namesTable // by router UID
	Users       []config.User
	Presets     []config.Preset
	FilterSets  []config.FilterSet
	Bookmarks   []config.Bookmark // Everyone bookmarks
	Panels      exportPanels
	Artwork     exportArtwork
	APIClients  []config.APIClient
	Browser     map[string]any
	Art         map[string]string
}

var exportLoaders = map[int]func(dir string) (*exportBundle, error){
	1: loadExportV1,
}

// dir must exist and be empty.
func stageExport(zipPath, dir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return badExport("the file is not a zip file")
	}
	defer zr.Close()
	if len(zr.File) > exportMaxEntries {
		return badExport("the file holds %d entries; an export holds at most %d", len(zr.File), exportMaxEntries)
	}
	seen := map[string]bool{}
	left := exportMaxTotal
	for _, f := range zr.File {
		if !exportEntryRe.MatchString(f.Name) {
			if err := newerFormat(zr.File); err != nil {
				return err
			}
			return badExport("the file holds %q, which is not part of a Sode export", f.Name)
		}
		if seen[f.Name] {
			return badExport("the file holds %q twice", f.Name)
		}
		seen[f.Name] = true
		dst := filepath.Join(dir, filepath.FromSlash(f.Name))
		if strings.HasSuffix(f.Name, "/") {
			if err := os.MkdirAll(dst, 0o700); err != nil {
				return err
			}
			continue
		}
		n, err := unpackEntry(f, dst, left)
		if err != nil {
			return err
		}
		left -= n
	}
	return nil
}

// unpackEntry reads at most exportMaxEntry bytes, and at most left, whatever the zip header says.
func unpackEntry(f *zip.File, dst string, left int64) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, badExport("%s cannot be read: %v", f.Name, err)
	}
	defer rc.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return 0, err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(rc, min(exportMaxEntry, left)+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	switch {
	case n > exportMaxEntry:
		return n, badExport("%s is over %d MiB", f.Name, exportMaxEntry>>20)
	case n > left:
		return n, badExport("the file unpacks to over %d MiB", exportMaxTotal>>20)
	case err != nil:
		return n, badExport("%s cannot be read: %v", f.Name, err)
	}
	return n, nil
}

// newerFormat: a newer Sode can add entry names, so its file answers newer_format, not bad_file.
func newerFormat(files []*zip.File) error {
	i := slices.IndexFunc(files, func(f *zip.File) bool { return f.Name == "manifest.yaml" })
	if i < 0 {
		return nil
	}
	rc, err := files[i].Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	b, _ := io.ReadAll(io.LimitReader(rc, exportMaxEntry))
	if v := manifestFormat(b); v > exportFormat {
		return newerErr(v)
	}
	return nil
}

func manifestFormat(b []byte) int {
	var head struct {
		Format int `yaml:"format"`
	}
	yaml.Unmarshal(b, &head)
	return head.Format
}

func newerErr(v int) error {
	return &exportErr{422, "newer_format", fmt.Sprintf("made by a newer Sode: the file is export format %d, and this Sode reads up to %d", v, exportFormat)}
}

func loadExport(dir string) (*exportBundle, error) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		return nil, badExport("the file has no manifest.yaml, so it is not a Sode export")
	}
	v := manifestFormat(b)
	if v < 1 {
		return nil, badExport("manifest.yaml has no format version, so the file is not a Sode export")
	}
	load, ok := exportLoaders[v]
	switch {
	case !ok && v > exportFormat:
		return nil, newerErr(v)
	case !ok:
		return nil, badExport("export format %d is not one this Sode reads", v)
	}
	return load(dir)
}

// readExportYAML reads one entry into a node with unknown keys removed (listed in unknown) and router
// prefixes made UIDs. A missing entry is (nil, nil).
func readExportYAML(dir, name string, prune func(*yaml.Node), uidOf map[string]string, only string) (*yaml.Node, error) {
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil {
		return nil, badExport("%s is not valid YAML: %v", name, err)
	}
	if prune != nil {
		prune(&n)
	}
	if uidOf != nil {
		if err := toUIDs(&n, uidOf, only); err != nil {
			return nil, badExport("%s: %v", name, err)
		}
	}
	return &n, nil
}

// toUIDs undoes toPrefixes. only is the file's one router, else "": a reference (a mapping with n) without
// a router names it. A raw router UID is a router not in the manifest, which toPrefixes writes as is.
func toUIDs(n *yaml.Node, uidOf map[string]string, only string) error {
	if n.Kind != yaml.MappingNode {
		for _, c := range n.Content {
			if err := toUIDs(c, uidOf, only); err != nil {
				return err
			}
		}
		return nil
	}
	hasRouter, hasN := false, false
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i].Value, n.Content[i+1]
		switch {
		case exportRouterKeys[k] && v.Kind == yaml.ScalarNode:
			hasRouter = hasRouter || k == "router"
			if v.Value == "" {
				break
			}
			if uid, ok := uidOf[v.Value]; ok {
				v.Value = uid
			} else if !rawRouterUIDRe.MatchString(v.Value) {
				return fmt.Errorf("router %q is not in the manifest", v.Value)
			}
		case k == "n" && v.Kind == yaml.ScalarNode:
			hasN = true
		}
		if err := toUIDs(v, uidOf, only); err != nil {
			return err
		}
	}
	if hasN && !hasRouter && only != "" {
		n.Content = append(n.Content, strNode("router"), strNode(only))
	}
	return nil
}

// rawRouterUIDRe is config's generated router UID form.
var rawRouterUIDRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

func decodeInto(n *yaml.Node, name string, v any) error {
	if err := n.Decode(v); err != nil {
		return badExport("%s: %v", name, err)
	}
	return nil
}

// loadExportV1 reads format 1. Do not change it after release: a change to the file adds a new version.
func loadExportV1(dir string) (*exportBundle, error) {
	out := &exportBundle{Names: map[string]namesTable{}, Art: map[string]string{}}
	pruneAs := func(t reflect.Type) func(*yaml.Node) {
		return func(n *yaml.Node) { pruneUnknown(n, t, "", &out.Unknown) }
	}
	man, err := readExportYAML(dir, "manifest.yaml", func(n *yaml.Node) {
		pruneUnknown(n, reflect.TypeFor[exportManifest](), "manifest", &out.Unknown)
	}, nil, "")
	if err != nil {
		return nil, err
	}
	if err := decodeInto(man, "manifest.yaml", &out.Manifest); err != nil {
		return nil, err
	}
	uidOf := map[string]string{}
	uids := map[string]bool{}
	for _, r := range out.Manifest.Routers {
		switch {
		case !routerUIDRe.MatchString(r.UID):
			return nil, badExport("manifest.yaml: router UID %q is not valid", r.UID)
		case uids[r.UID]:
			return nil, badExport("two routers in the file share the UID %s", r.UID)
		case r.Prefix == "" || uidOf[r.Prefix] != "":
			return nil, badExport("manifest.yaml: router %s needs its own prefix", r.UID)
		}
		uids[r.UID], uidOf[r.Prefix] = true, r.UID
	}
	only := ""
	if len(out.Manifest.Routers) == 1 {
		only = out.Manifest.Routers[0].UID
	}

	type entry struct {
		file  string
		prune func(*yaml.Node)
		into  any
	}
	var users exportUsers
	var presets exportPresets
	var sets exportFilterSets
	var bookmarks exportBookmarks
	var clients exportAPIClients
	var routers exportRouters
	entries := []entry{
		{"routers.yaml", pruneAs(reflect.TypeFor[exportRouters]()), &routers},
		{"users.yaml", pruneAs(reflect.TypeFor[exportUsers]()), &users},
		{"presets.yaml", pruneAs(reflect.TypeFor[exportPresets]()), &presets},
		{"filtersets.yaml", pruneAs(reflect.TypeFor[exportFilterSets]()), &sets},
		{"bookmarks.yaml", pruneAs(reflect.TypeFor[exportBookmarks]()), &bookmarks},
		{"panels.yaml", pruneAs(reflect.TypeFor[exportPanels]()), &out.Panels},
		{"artwork.yaml", pruneAs(reflect.TypeFor[exportArtwork]()), &out.Artwork},
		{"apiclients.yaml", pruneAs(reflect.TypeFor[exportAPIClients]()), &clients},
		{"browser.yaml", nil, &out.Browser},
	}
	for _, e := range entries {
		n, err := readExportYAML(dir, e.file, e.prune, uidOf, only)
		if err != nil {
			return nil, err
		}
		if n != nil {
			if err := decodeInto(n, e.file, e.into); err != nil {
				return nil, err
			}
		}
	}
	out.Routers, out.Users, out.Presets, out.FilterSets, out.APIClients = routers.Routers, users.Users, presets.Presets, sets.FilterSets, clients.APIClients
	out.Bookmarks = bookmarks.Bookmarks
	seen := map[string]bool{}
	for _, r := range out.Routers {
		switch {
		case seen[r.UID]:
			return nil, badExport("two routers in the file share the UID %s", r.UID)
		case !uids[r.UID]:
			return nil, badExport("routers.yaml: router %s is not in the manifest", r.UID)
		}
		seen[r.UID] = true
	}

	settings, err := readExportYAML(dir, "settings.yaml", func(n *yaml.Node) { pruneSettings(n, &out.Unknown) }, uidOf, only)
	if err != nil {
		return nil, err
	}
	if settings != nil {
		if err := settingsFromSections(settings, out); err != nil {
			return nil, err
		}
	}

	for _, r := range out.Manifest.Routers {
		name := "names/" + r.UID + ".yaml"
		n, err := readExportYAML(dir, name, pruneAs(reflect.TypeFor[namesTable]()), nil, "")
		if err != nil {
			return nil, err
		}
		if n == nil {
			continue
		}
		var t namesTable
		if err := decodeInto(n, name, &t); err != nil {
			return nil, err
		}
		if t.Router != r.UID {
			return nil, badExport("%s names router %q", name, t.Router)
		}
		out.Names[r.UID] = t
	}
	ents, _ := filepath.Glob(filepath.Join(dir, "names", "*.yaml"))
	for _, p := range ents {
		if uid := strings.TrimSuffix(filepath.Base(p), ".yaml"); !uids[uid] {
			return nil, badExport("names/%s.yaml is for a router that is not in the manifest", uid)
		}
	}
	err = filepath.WalkDir(filepath.Join(dir, "art"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out.Art[filepath.ToSlash(rel)] = p
		return nil
	})
	return out, err
}

func settingsFromSections(n *yaml.Node, out *exportBundle) error {
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return badExport("settings.yaml must be a mapping of sections")
	}
	flat := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	out.SettingKeys = map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		out.Sections = append(out.Sections, n.Content[i].Value)
		if sec := n.Content[i+1]; sec.Kind == yaml.MappingNode {
			flat.Content = append(flat.Content, sec.Content...)
			for j := 0; j+1 < len(sec.Content); j += 2 {
				key := sec.Content[j].Value
				out.SettingKeys[key] = true
				if sub := sec.Content[j+1]; sub.Kind == yaml.MappingNode {
					for k := 0; k < len(sub.Content); k += 2 {
						out.SettingKeys[key+"."+sub.Content[k].Value] = true
					}
				}
			}
		}
	}
	for _, f := range reflect.VisibleFields(reflect.TypeFor[config.Config]()) {
		if name, opts, _ := strings.Cut(f.Tag.Get("yaml"), ","); strings.Contains(opts, "omitempty") {
			out.SettingKeys[name] = true
		}
	}
	cfg := config.Defaults()
	if err := flat.Decode(&cfg); err != nil {
		return badExport("settings.yaml: %v", err)
	}
	out.Settings = &cfg
	return nil
}

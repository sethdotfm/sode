package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// Templates of the panel editor: a saved layout, optionally with settings and cell contents (Sources, Destinations).
// Stored apart from panels, one YAML file per template under templates/everyone/ or
// templates/me/ beside the config (memory only with no config path). Docs: docs/panels-design.md.

const (
	templatesDir     = "templates"
	templateMe       = "me"
	templateEveryone = "everyone"
	templateMax      = 200
	templateNameMax  = 60
)

var (
	tplIDRe        = regexp.MustCompile(`^t-[0-9a-f]{8}$`)
	errTemplateCap = errors.New("too many templates")
	errTemplateDup = errors.New("there is already a template with that name")

	// contentKeys are what fills a cell once its type is set. A template leaves them out unless
	// asked, so a new show's Sources and Destinations are chosen afresh.
	contentKeys = []string{"n", "scope", "name", "caption"}
)

// Template is a file's content. Owner is the user a Me template belongs to, and for an Everyone
// template the user who saved it. Settings is nil when the template carries none. Sources keeps
// Source and Preset contents, Dests keeps Destination contents. Contents is the single flag from
// before the split: a save or file that sets it means both, and it is never stored or sent back.
type Template struct {
	ID       string      `yaml:"id" json:"id"`
	Name     string      `yaml:"name" json:"name"`
	Scope    string      `yaml:"scope" json:"scope"`
	Owner    string      `yaml:"owner" json:"owner"`
	Sources  bool        `yaml:"sources" json:"sources"`
	Dests    bool        `yaml:"dests" json:"dests"`
	Contents bool        `yaml:"contents,omitempty" json:"contents,omitempty"`
	Rows     int         `yaml:"rows" json:"rows"`
	Cols     int         `yaml:"cols" json:"cols"`
	Pages    []PanelPage `yaml:"pages" json:"pages"`
	// LegacyCells is a template from before Pages; migrate makes it one Page.
	LegacyCells []map[string]any `yaml:"cells,omitempty" json:"cells,omitempty"`
	Settings    map[string]any   `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// templateIn is the body of a save. Pin is only read for Everyone, and never stored.
type templateIn struct {
	Template
	Pin string `json:"pin"`
}

// keepsContents reports whether a cell of this kind keeps its Source, Destination or Preset and its
// caption. A Take, Clear or Current caption is layout, so either option keeps it.
func (t *Template) keepsContents(kind any) bool {
	switch kind {
	case "source", "preset":
		return t.Sources
	case "destination":
		return t.Dests
	}
	return t.Sources || t.Dests
}

// migrate turns the old single flag into the two it became, and an old grid into one Page.
func (t *Template) migrate() {
	if t.Contents {
		t.Sources, t.Dests = true, true
	}
	t.Contents = false
	t.Pages = normalPages(t.Pages, t.LegacyCells, t.ID)
	t.LegacyCells = nil
}

func validTemplateID(id string) bool { return tplIDRe.MatchString(id) }

// clean fits a template to what the store keeps. Artwork ids belong to the panel they came
// from, so they never travel; an icon does.
func (t *Template) clean() error {
	t.Name = cleanText(strings.TrimSpace(t.Name), templateNameMax)
	t.migrate()
	switch {
	case t.Name == "":
		return errors.New("a template needs a name")
	case t.Scope != templateMe && t.Scope != templateEveryone:
		return fmt.Errorf("scope is %q or %q", templateMe, templateEveryone)
	}
	if t.Rows == 0 {
		t.Rows = 4
	}
	if t.Cols == 0 {
		t.Cols = 8
	}
	if t.Rows < 1 || t.Rows > panelRowsMax || t.Cols < 1 || t.Cols > panelColsMax {
		return fmt.Errorf("rows and columns are 1 to %d and 1 to %d", panelRowsMax, panelColsMax)
	}
	pages, err := checkPages(t.Pages, t.Rows, t.Cols)
	if err != nil {
		return err
	}
	for i, g := range pages {
		cells := make([]map[string]any, 0, len(g.Cells))
		for _, c := range g.Cells {
			c = maps.Clone(c)
			if !t.keepsContents(c["kind"]) {
				for _, k := range contentKeys {
					delete(c, k)
				}
			}
			dropArtwork(c, "art")
			cells = append(cells, c)
		}
		pages[i].Cells = cells
	}
	t.Pages = pages
	if t.Settings != nil {
		t.Settings = maps.Clone(t.Settings)
		dropArtwork(t.Settings, "backdrop")
		if !t.Dests {
			delete(t.Settings, panelFixedKey)
		}
		settings, err := cleanSettings(t.Settings)
		if err != nil {
			return err
		}
		t.Settings = settings
	}
	return nil
}

func dropArtwork(m map[string]any, key string) {
	if s, ok := m[key].(string); ok && artIDRe.MatchString(s) {
		delete(m, key)
	}
}

type templateStore struct {
	yamlDir[Template]
}

// loadTemplates runs once from New. A file that does not parse, or whose id or folder
// differs from what it holds, is logged and skipped.
func loadTemplates(cfgPath string) *templateStore {
	s := &templateStore{}
	s.by = map[string]Template{}
	s.prefix = "t-"
	s.idOf = func(t *Template) *string { return &t.ID }
	s.nameOf = func(t Template) string { return t.Name }
	s.pathOf = func(dir string, t Template) string { return filepath.Join(dir, t.Scope, t.ID+".yaml") }
	if cfgPath == "" {
		return s
	}
	s.dir = filepath.Join(filepath.Dir(cfgPath), templatesDir)
	for _, scope := range []string{templateMe, templateEveryone} {
		err := s.loadDir(filepath.Join(s.dir, scope), func(path, id string, b []byte, err error) (Template, bool) {
			var t Template
			if err == nil {
				err = yaml.Unmarshal(b, &t)
			}
			t.migrate()
			if err != nil || t.ID != id || !validTemplateID(id) || t.Scope != scope {
				log.Printf("templates: skipped %s: not a template of this folder (%v)", path, err)
				return t, false
			}
			return t, true
		})
		if err != nil && !os.IsNotExist(err) {
			log.Printf("templates: skipped %s: %v", s.dir, err)
		}
	}
	return s
}

func (s *templateStore) list() []Template { return s.sorted(nil) }

// sameBucket: one name once per place it is listed, Everyone for all or Me for one owner.
func sameBucket(a, b Template) bool {
	return a.Scope == b.Scope && (a.Scope == templateEveryone || strings.EqualFold(a.Owner, b.Owner))
}

func (s *templateStore) create(t Template) (Template, error) {
	vet := func(t Template) error {
		for _, o := range s.by {
			if sameBucket(o, t) && strings.EqualFold(o.Name, t.Name) {
				return errTemplateDup
			}
		}
		return nil
	}
	return s.insert(t, templateMax, errTemplateCap, vet, func(t *Template) error { return s.writeLocked(*t) })
}

func (s *templateStore) delete(id string) (bool, error) {
	return s.remove(id, nil)
}

// followUsers carries a Users rename to the owner of Me templates. A Me template of a removed
// user stays on disk, owned by nobody who can sign in, like their panels.
func (s *templateStore) followUsers(renames map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.by {
		to, ok := renames[strings.ToLower(t.Owner)]
		if !ok || to == t.Owner {
			continue
		}
		t.Owner = to
		if err := s.writeLocked(t); err != nil {
			log.Printf("templates: owner of %s not renamed: %v", id, err)
			continue
		}
		s.by[id] = t
	}
}

func init() {
	const read, manage = config.ScopePanels, config.ScopePanelsManage
	registerAPI(
		apiRoute{"GET", apiBase + "/panel-templates", read, (*App).apiTemplateList},
		apiRoute{"POST", apiBase + "/panel-templates", manage, (*App).apiTemplateCreate},
		apiRoute{"GET", apiBase + "/panel-templates/{id}", read, (*App).apiTemplateRead},
		apiRoute{"DELETE", apiBase + "/panel-templates/{id}", manage, (*App).apiTemplateDelete},
	)
}

// templateReach: a Me template is its owner's; an Everyone template is for every caller to
// use, and the owner or a Trusted user change it. Trusted is read live from the config.
func (a *App) templateReach(cl *apiCtx, t Template) (read, edit bool) {
	trusted := cl.Actor.Kind == actorUser && a.trusted(cl.Actor.Name)
	own := cl.Actor.Kind == actorUser && strings.EqualFold(t.Owner, cl.Actor.Name)
	if t.Scope == templateMe {
		return own, own
	}
	return true, own || trusted
}

func (a *App) templateOut(cl *apiCtx, t Template) map[string]any {
	_, edit := a.templateReach(cl, t)
	b, _ := json.Marshal(t)
	m := map[string]any{}
	json.Unmarshal(b, &m)
	m["canEdit"] = edit
	return m
}

func (a *App) apiTemplateList(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	out := []map[string]any{}
	for _, t := range a.templates.list() {
		if read, _ := a.templateReach(cl, t); read {
			out = append(out, a.templateOut(cl, t))
		}
	}
	apiJSON(w, map[string]any{"templates": out})
}

func (a *App) templateFor(w http.ResponseWriter, r *http.Request, cl *apiCtx, needEdit bool) (Template, bool) {
	id := r.PathValue("id")
	t, ok := a.templates.get(id)
	read, edit := false, false
	if ok && validTemplateID(id) {
		read, edit = a.templateReach(cl, t)
	}
	if refuseReach(w, read, edit, needEdit, "no template "+id+" that this key may use",
		"this template is use only for this key: only its owner or a Trusted user may delete it") {
		return Template{}, false
	}
	return t, true
}

func (a *App) apiTemplateRead(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if t, ok := a.templateFor(w, r, cl, false); ok {
		apiJSON(w, a.templateOut(cl, t))
	}
}

// mayEveryone is the rule of story 80, for Everyone templates and Checkpoint sets: a Trusted
// user, or whoever holds the Advanced Settings PIN. A REST call has no connection to unlock,
// so the PIN rides in the body and counts as a wrong try of the same pool as the Settings
// dialog's. msg is the refusal when no PIN is set or sent.
func (a *App) mayEveryone(cl *apiCtx, pin, msg string) error {
	if a.trusted(cl.Actor.Name) {
		return nil
	}
	hash := a.r().cfg.SettingsPin
	if hash == "" || pin == "" {
		return &panelClientFail{http.StatusForbidden, "trusted_required", msg}
	}
	if err := a.tryPin(&a.pin, cl.IP, func() bool { return config.CheckPin(hash, pin) }); err != nil {
		return &panelClientFail{http.StatusForbidden, "trusted_required", err.Error()}
	}
	if a.r().cfg.SettingsPin != hash {
		return &panelClientFail{http.StatusForbidden, "trusted_required", msg}
	}
	return nil
}

func (a *App) apiTemplateCreate(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	if cl.Actor.Kind != actorUser {
		apiFail(w, http.StatusForbidden, "user_required", "a template is owned by a user: use a user's key to save one")
		return
	}
	body, ok := readBody(w, r, panelLayoutBodyMax, "template")
	if !ok {
		return
	}
	var in templateIn
	if err := json.Unmarshal(body, &in); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", "send a template as JSON: "+err.Error())
		return
	}
	t := in.Template
	t.ID, t.Owner = "", cl.Actor.Name
	if err := t.clean(); err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if t.Scope == templateEveryone {
		if err := a.mayEveryone(cl, in.Pin, "only a Trusted user, or the Advanced Settings PIN, may save a template to Everyone"); err != nil {
			failWith(w, err)
			return
		}
	}
	made, err := a.templates.create(t)
	switch {
	case errors.Is(err, errTemplateCap), errors.Is(err, errTemplateDup):
		apiFail(w, http.StatusConflict, "conflict", err.Error())
		return
	case err != nil:
		apiFail(w, http.StatusInternalServerError, "internal", "the template could not be saved: "+err.Error())
		return
	}
	a.apiEvent(cl.Actor, "templateCreate", "ok", "template "+made.ID+" "+made.Name+" ("+made.Scope+")", nil)
	apiCreated(w, a.templateOut(cl, made))
}

func (a *App) apiTemplateDelete(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	t, ok := a.templateFor(w, r, cl, true)
	if !ok {
		return
	}
	if _, err := a.templates.delete(t.ID); err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the template could not be deleted: "+err.Error())
		return
	}
	a.apiEvent(cl.Actor, "templateDelete", "ok", "template "+t.ID+" "+t.Name, nil)
	w.WriteHeader(http.StatusNoContent)
}

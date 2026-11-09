package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// Templates of the panel editor: a saved layout, optionally with settings and cell contents.
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
// template the user who saved it. Settings is nil when the template carries none.
type Template struct {
	ID       string           `yaml:"id" json:"id"`
	Name     string           `yaml:"name" json:"name"`
	Scope    string           `yaml:"scope" json:"scope"`
	Owner    string           `yaml:"owner" json:"owner"`
	Contents bool             `yaml:"contents" json:"contents"`
	Rows     int              `yaml:"rows" json:"rows"`
	Cols     int              `yaml:"cols" json:"cols"`
	Cells    []map[string]any `yaml:"cells" json:"cells"`
	Settings map[string]any   `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// templateIn is the body of a save. Pin is only read for Everyone, and never stored.
type templateIn struct {
	Template
	Pin string `json:"pin"`
}

func validTemplateID(id string) bool { return tplIDRe.MatchString(id) }

// clean fits a template to what the store keeps. Artwork ids belong to the panel they came
// from, so they never travel; an icon does.
func (t *Template) clean() error {
	t.Name = cleanText(strings.TrimSpace(t.Name), templateNameMax)
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
	if len(t.Cells) > panelRowsMax*panelColsMax {
		return errors.New("too many cells for the grid")
	}
	cells := make([]map[string]any, 0, len(t.Cells))
	for _, c := range t.Cells {
		c = maps.Clone(c)
		if !t.Contents {
			for _, k := range contentKeys {
				delete(c, k)
			}
		}
		dropArtwork(c, "art")
		cells = append(cells, c)
	}
	t.Cells = cells
	if t.Settings != nil {
		t.Settings = maps.Clone(t.Settings)
		dropArtwork(t.Settings, "backdrop")
	}
	return nil
}

func dropArtwork(m map[string]any, key string) {
	if s, ok := m[key].(string); ok && artIDRe.MatchString(s) {
		delete(m, key)
	}
}

type templateStore struct {
	mu  sync.Mutex
	dir string // "" = memory only
	by  map[string]Template
}

// loadTemplates runs once from New. A file that does not parse, or whose id or folder
// differs from what it holds, is logged and skipped.
func loadTemplates(cfgPath string) *templateStore {
	s := &templateStore{by: map[string]Template{}}
	if cfgPath == "" {
		return s
	}
	s.dir = filepath.Join(filepath.Dir(cfgPath), templatesDir)
	for _, scope := range []string{templateMe, templateEveryone} {
		ents, err := os.ReadDir(filepath.Join(s.dir, scope))
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("templates: skipped %s: %v", s.dir, err)
			}
			continue
		}
		for _, e := range ents {
			id, ok := strings.CutSuffix(e.Name(), ".yaml")
			if !ok || e.IsDir() {
				continue
			}
			path := filepath.Join(s.dir, scope, e.Name())
			b, err := os.ReadFile(path)
			var t Template
			if err == nil {
				err = yaml.Unmarshal(b, &t)
			}
			if err != nil || t.ID != id || !validTemplateID(id) || t.Scope != scope {
				log.Printf("templates: skipped %s: not a template of this folder (%v)", path, err)
				continue
			}
			s.by[id] = t
		}
	}
	return s
}

func (s *templateStore) get(id string) (Template, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.by[id]
	return t, ok
}

func (s *templateStore) list() []Template {
	s.mu.Lock()
	out := make([]Template, 0, len(s.by))
	for _, t := range s.by {
		out = append(out, t)
	}
	s.mu.Unlock()
	slices.SortFunc(out, func(a, b Template) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func (s *templateStore) path(t Template) string {
	return filepath.Join(s.dir, t.Scope, t.ID+".yaml")
}

// sameBucket: one name once per place it is listed, Everyone for all or Me for one owner.
func sameBucket(a, b Template) bool {
	return a.Scope == b.Scope && (a.Scope == templateEveryone || strings.EqualFold(a.Owner, b.Owner))
}

func (s *templateStore) create(t Template) (Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.by) >= templateMax {
		return Template{}, errTemplateCap
	}
	for _, o := range s.by {
		if sameBucket(o, t) && strings.EqualFold(o.Name, t.Name) {
			return Template{}, errTemplateDup
		}
	}
	for {
		b := make([]byte, 4)
		rand.Read(b)
		t.ID = "t-" + hex.EncodeToString(b)
		if _, taken := s.by[t.ID]; !taken {
			break
		}
	}
	if s.dir != "" {
		b, err := yaml.Marshal(t)
		if err != nil {
			return Template{}, err
		}
		if err := os.MkdirAll(filepath.Join(s.dir, t.Scope), 0o700); err != nil {
			return Template{}, err
		}
		if err := config.AtomicWrite(s.path(t), b); err != nil {
			return Template{}, err
		}
	}
	s.by[t.ID] = t
	return t, nil
}

func (s *templateStore) delete(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.by[id]
	if !ok {
		return false, nil
	}
	if s.dir != "" {
		if err := os.Remove(s.path(t)); err != nil && !os.IsNotExist(err) {
			return true, err
		}
	}
	delete(s.by, id)
	return true, nil
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
		if s.dir != "" {
			b, err := yaml.Marshal(t)
			if err == nil {
				err = config.AtomicWrite(s.path(t), b)
			}
			if err != nil {
				log.Printf("templates: owner of %s not renamed: %v", id, err)
				continue
			}
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

// mayShareEveryone is the rule of story 80: a Trusted user, or whoever holds the Advanced
// Settings PIN. A REST call has no connection to unlock, so the PIN rides in the body and
// counts as a wrong try of the same pool as the Settings dialog's.
func (a *App) mayShareEveryone(cl *apiCtx, pin string) error {
	if a.trusted(cl.Actor.Name) {
		return nil
	}
	const msg = "only a Trusted user, or the Advanced Settings PIN, may save a template to Everyone"
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
	body, ok := readBody(w, r, panelBodyMax, "template")
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
		if err := a.mayShareEveryone(cl, in.Pin); err != nil {
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

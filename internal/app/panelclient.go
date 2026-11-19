package app

import (
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// A panel that acts as an API client has one in the config (ADR 0006), found by its Panel id.
// It is made and removed with the panel. It keeps the panel's name for life, so History rows
// and keys never point at a renamed client. Docs: docs/panels-design.md.

func panelClientName(id string) string { return "Panel " + id }

func clientOfPanel(cfg config.Config, p Panel) (config.APIClient, bool) {
	if p.Client == "" {
		return config.APIClient{}, false
	}
	c, ok := cfg.FindAPIClient(p.Client)
	return c, ok && c.Panel == p.ID
}

// panelClients lists a user's panels that have a client, for the Rights change notice. Wider
// means the client now grants more than the user's own Rights do.
type panelClientNote struct {
	Client string `json:"client"`
	Panel  string `json:"panel"`
	Name   string `json:"name"` // the panel's name
	Wider  bool   `json:"wider,omitempty"`
}

func (a *App) panelClientsOf(cfg config.Config, u config.User) []panelClientNote {
	var out []panelClientNote
	for _, p := range a.panels.list() {
		if p.Owner != u.ID {
			continue
		}
		if c, ok := clientOfPanel(cfg, p); ok {
			out = append(out, panelClientNote{Client: c.Name, Panel: p.ID, Name: p.Name, Wider: !c.Rights.Within(u.Rights)})
		}
	}
	return out
}

type panelClientFail struct {
	status int
	code   string
	msg    string
}

func (f *panelClientFail) Error() string { return f.msg }

// failStore answers a store's sentinel (a *panelClientFail, maybe wrapped) with its status and anything else as a
// disk fault. Unlike failWith it keeps the wrapped detail and never answers 400 by default.
func failStore(w http.ResponseWriter, err error, prefix string) {
	var f *panelClientFail
	if errors.As(err, &f) {
		apiFail(w, f.status, f.code, err.Error())
		return
	}
	apiFail(w, http.StatusInternalServerError, "internal", prefix+": "+err.Error())
}

func failWith(w http.ResponseWriter, err error) {
	var f *panelClientFail
	if errors.As(err, &f) {
		apiFail(w, f.status, f.code, f.msg)
		return
	}
	apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
}

// clientRightsFor decides the Rights a panel's client ends up with, or why not. cur is nil for
// a new client. A request that leaves them out or repeats the stored ones changes nothing, so a
// save never fails over Rights an Administrator widened. Anything else needs the key of a user
// who can edit the panel (any Role, ADR 0006) and must be within the caller's own Rights.
func (a *App) clientRightsFor(cl *apiCtx, want *config.Rights, cur *config.Rights) (config.Rights, bool, error) {
	if cur != nil && (want == nil || reflect.DeepEqual(clean(*want), *cur)) {
		return *cur, false, nil
	}
	if cl.Actor.Kind != actorUser {
		return config.Rights{}, false, &panelClientFail{http.StatusForbidden, "user_required", "a panel's API client is made with a user's key"}
	}
	r := cl.Rights.Clone()
	if want != nil {
		r = clean(*want)
	}
	if !r.Within(cl.Rights) {
		return config.Rights{}, false, apiAdminOnly("rights wider than your own")
	}
	return r, true, nil
}

// apiAdminOnly refuses an Administrator-only change: a key never acts as an Administrator (keyRole).
func apiAdminOnly(what string) error {
	return &panelClientFail{http.StatusForbidden, "administrator_only",
		what + " need an Administrator, and a key never acts as one: an Administrator makes this change in Settings"}
}

func clean(r config.Rights) config.Rights {
	r = r.Clone()
	r.Visibility = strings.ToLower(strings.TrimSpace(r.Visibility))
	return r
}

// putPanelClient adds, replaces the rights of, or (rights nil) removes the client of panel id. Removing it ends its
// keys and open streams through the config commit. nets: a new client's allowed networks (Save a copy keeps its
// source's).
func (a *App) putPanelClient(by Actor, id string, rights *config.Rights, nets []string) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return a.putPanelClientLocked(by, id, rights, nets)
}

// Caller holds cfgMu.
func (a *App) putPanelClientLocked(by Actor, id string, rights *config.Rights, nets []string) error {
	name := panelClientName(id)
	was := a.r().cfg
	defer func() { a.recordSave(by, "api", clientChanges(was, a.r().cfg, name), false) }()
	return a.applyConfig(func(cur *config.Config) error {
		cur.APIClients = slices.Clone(cur.APIClients)
		i := slices.IndexFunc(cur.APIClients, func(c config.APIClient) bool { return strings.EqualFold(c.Name, name) })
		switch {
		case rights == nil && i >= 0 && cur.APIClients[i].Panel == id:
			cur.APIClients = slices.Delete(cur.APIClients, i, i+1)
		case rights == nil:
		case i >= 0 && cur.APIClients[i].Panel == id:
			cur.APIClients[i].Rights = rights.Clone()
		case i >= 0:
			return errors.New("an API client is already named " + name)
		default:
			cur.APIClients = append(cur.APIClients, config.APIClient{Name: name, Scopes: slices.Clone(config.PanelClientScopes), Rights: rights.Clone(), Panel: id, Networks: slices.Clone(nets)})
		}
		return nil
	})
}

// dropOrphanPanelClients removes clients of panels that no longer exist (a panel file deleted
// by hand, or a config imported without its panels). It also removes clients that a loaded
// panel no longer names. A panel file that is on disk but failed to load keeps its client.
// A repair of the file then brings the panel back still signed in. Without this, key reconcile
// would end its keys.
func dropOrphanPanelClients(cfg *config.Config, panels *panelStore) {
	if panels.dirFailed {
		return
	}
	cfg.APIClients = slices.DeleteFunc(slices.Clone(cfg.APIClients), func(c config.APIClient) bool {
		if c.Panel == "" {
			return false
		}
		if p, ok := panels.get(c.Panel); ok {
			return !strings.EqualFold(p.Client, c.Name)
		}
		return !panels.skipped[c.Panel]
	})
}

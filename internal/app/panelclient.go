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
// It is made and removed with the panel and keeps the panel's name for life, so History rows
// and keys never point at a renamed client. Docs: docs/panels-design.md.

func panelClientName(id string) string { return "Panel " + id }

// clientOfPanel is the panel's own API client, if it has one.
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
		if !strings.EqualFold(p.Owner, u.Name) {
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
// save never fails over Rights an administrator widened. Anything else needs a Trusted caller
// (read live) and must be within the caller's own Rights.
func (a *App) clientRightsFor(cl *apiCtx, want *config.Rights, cur *config.Rights) (config.Rights, bool, error) {
	if cur != nil && (want == nil || reflect.DeepEqual(clean(*want), *cur)) {
		return *cur, false, nil
	}
	if !a.trusted(cl.Actor.Name) {
		return config.Rights{}, false, &panelClientFail{http.StatusForbidden, "trusted_required", "only a Trusted user may make a panel act as an API client or change its rights"}
	}
	r := cl.Rights.Clone()
	if want != nil {
		r = clean(*want)
	}
	if !r.Within(cl.Rights) {
		return config.Rights{}, false, &panelClientFail{http.StatusForbidden, "outside_rights", "the panel's rights may not be wider than your own"}
	}
	return r, true, nil
}

func clean(r config.Rights) config.Rights {
	r = r.Clone()
	r.Visibility = strings.ToLower(strings.TrimSpace(r.Visibility))
	return r
}

// setPanelClient adds, replaces the rights of, or (rights nil) removes the client of panel id.
// Removing it ends its keys and open streams through the config commit.
func (a *App) setPanelClient(id string, rights *config.Rights) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	name := panelClientName(id)
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
			cur.APIClients = append(cur.APIClients, config.APIClient{Name: name, Scopes: slices.Clone(config.PanelClientScopes), Rights: rights.Clone(), Panel: id})
		}
		return nil
	})
}

// dropOrphanPanelClients removes clients of panels that no longer exist (a panel file deleted
// by hand, or a config imported without its panels) and clients a loaded panel no longer names.
// A panel file that is on disk but failed to load keeps its client, so repairing the file
// brings the panel back still signed in; key reconcile would otherwise end its keys.
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

// keepPanelClients folds an administrator's client list into the config's: a panel client in
// the request takes its Rights and Scopes (the Advanced Settings PIN is what widens them) but
// keeps its panel link and name, and one left out stays. Plain clients replace the old ones.
func keepPanelClients(cur []config.APIClient, req []config.APIClient) []config.APIClient {
	var out []config.APIClient
	seen := map[string]bool{}
	for _, c := range req {
		key := strings.ToLower(strings.TrimSpace(c.Name))
		if i := slices.IndexFunc(cur, func(o config.APIClient) bool { return o.Panel != "" && strings.EqualFold(o.Name, c.Name) }); i >= 0 {
			if seen[key] {
				continue
			}
			seen[key] = true
			o := cur[i]
			o.Rights, o.Scopes = c.Rights.Clone(), slices.Clone(c.Scopes)
			out = append(out, o)
			continue
		}
		c.Panel, c.Rights = "", c.Rights.Clone()
		out = append(out, c)
	}
	for _, o := range cur {
		if o.Panel != "" && !seen[strings.ToLower(o.Name)] {
			out = append(out, o)
		}
	}
	return out
}

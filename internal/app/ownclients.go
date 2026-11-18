package app

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// API clients a user makes from My user (owner rulings on #543). The user owns the client
// (config.APIClient.Owner): it saves at once, needs no Administrator while it stays within the
// user's own Rights, is capped live by those Rights (resolveKey) and is deleted with the user.
// Settings > Access > API stays Administrator only.

// ownClientScopes: what the owner's own key has, less `panelsmanage`, since a client owns nothing.
var ownClientScopes = slices.DeleteFunc(slices.Clone(userKeyScopes), func(s string) bool { return s == config.ScopePanelsManage })

type myClientReq struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
	// Rights: nil keeps the stored ones, or for a new client copies the owner's.
	Rights *config.Rights `json:"rights"`
}

type myClientView struct {
	Name   string        `json:"name"`
	Scopes []string      `json:"scopes"`
	Rights config.Rights `json:"rights"`
}

type myClientsMsg struct {
	T       string         `json:"t"` // "myClientsResult"
	ID      int            `json:"id"`
	OK      bool           `json:"ok"`
	Err     string         `json:"err,omitempty"`
	Clients []myClientView `json:"clients"`
	Keys    []apiKey       `json:"keys"`
}

// ownMaker is c's live user, if they can own API clients: signed in, not under the Lock, with a
// PIN, since a user without one holds no lasting keys.
func (a *App) ownMaker(c *client) (config.User, error) {
	u, err := a.ownUser(c)
	if err != nil {
		return u, err
	}
	if u.Pin == "" {
		return u, errors.New("set a PIN before you make an API client")
	}
	return u, nil
}

// ownedBy: a client of user id, found by name ignoring case.
func ownedBy(cfg config.Config, id, name string) (config.APIClient, bool) {
	cl, ok := cfg.FindAPIClient(name)
	return cl, ok && id != "" && cl.Owner == id
}

func (a *App) serveMyClients(c *client, m *inMsg) {
	res := myClientsMsg{T: "myClientsResult", ID: m.ID, Clients: []myClientView{}, Keys: []apiKey{}}
	u, err := a.ownUser(c)
	if err != nil {
		res.Err = err.Error()
		a.reply(c, res)
		return
	}
	res.OK = true
	cfg := a.r().cfg
	for _, cl := range cfg.APIClients {
		if cl.Owner == u.ID {
			res.Clients = append(res.Clients, myClientView{cl.Name, slices.Clone(cl.Scopes), cl.Rights.Clone()})
			for _, k := range a.keys.list() {
				if k.Owner.is(keyOwner{Kind: actorClient, Name: cl.Name}) {
					res.Keys = append(res.Keys, k)
				}
			}
		}
	}
	a.reply(c, res)
}

// SaveOwnClient adds or changes one of c's user's clients. The name is the client: a new name
// makes a new one, so a rename never ends keys by surprise.
func (a *App) SaveOwnClient(c *client, req myClientReq) error {
	for _, s := range req.Scopes {
		if !slices.Contains(ownClientScopes, s) {
			return fmt.Errorf("your API clients cannot have the %q scope", s)
		}
	}
	name := strings.TrimSpace(req.Name)
	if err := userNameRefusal(name); err != nil {
		return err
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	u, err := a.ownMaker(c) // under cfgMu, so a Rights edit cannot slip between check and save
	if err != nil {
		return err
	}
	return a.applyConfig(func(cur *config.Config) error {
		cur.APIClients = slices.Clone(cur.APIClients)
		i := slices.IndexFunc(cur.APIClients, func(o config.APIClient) bool { return strings.EqualFold(o.Name, name) })
		if i >= 0 && cur.APIClients[i].Owner != u.ID {
			return fmt.Errorf("an API client or panel is already named %q", name)
		}
		var stored *config.Rights
		if i >= 0 {
			stored = &cur.APIClients[i].Rights
		}
		rights, err := ownClientRights(u, req.Rights, stored)
		if err != nil {
			return err
		}
		scopes := slices.Clone(req.Scopes)
		if scopes == nil {
			scopes = []string{}
		}
		if i >= 0 {
			cur.APIClients[i].Scopes, cur.APIClients[i].Rights = scopes, rights
			return nil
		}
		cur.APIClients = append(cur.APIClients, config.APIClient{Name: name, Scopes: scopes, Rights: rights, Owner: u.ID})
		return nil
	})
}

// ownClientRights is clientRightsFor for a user's own client: unchanged Rights stay, so a save
// never fails over Rights an Administrator widened; new ones must be within the user's.
func ownClientRights(u config.User, want, stored *config.Rights) (config.Rights, error) {
	switch {
	case want == nil && stored != nil:
		return *stored, nil
	case want == nil:
		return u.Rights.Clone(), nil
	case stored != nil && reflect.DeepEqual(clean(*want), *stored):
		return *stored, nil
	}
	r := clean(*want)
	if !r.Within(u.Rights) {
		return config.Rights{}, errors.New("an API client's Rights must be within your own; an Administrator gives wider Rights in Settings")
	}
	return r, nil
}

// DeleteOwnClient removes one of c's user's clients. Its keys end through the config commit.
func (a *App) DeleteOwnClient(c *client, name string) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	u, err := a.ownUser(c)
	if err != nil {
		return err
	}
	if _, ok := ownedBy(a.r().cfg, u.ID, name); !ok {
		return errors.New("you have no API client of that name")
	}
	return a.applyConfig(func(cur *config.Config) error {
		cur.APIClients = slices.DeleteFunc(slices.Clone(cur.APIClients), func(o config.APIClient) bool {
			return o.Owner == u.ID && strings.EqualFold(o.Name, name)
		})
		return nil
	})
}

// CreateOwnClientKey makes a key for one of c's user's clients. It asks the user's PIN, as
// CreateOwnKey does. The key does not end with that PIN: it is the client's.
func (a *App) CreateOwnClientKey(c *client, client, pin, keyName string, expiresMs int64) (string, apiKey, error) {
	u, err := a.ownMaker(c)
	if err != nil {
		return "", apiKey{}, err
	}
	if err := a.tryPin(&a.userPin, c.ip, func() bool { return config.CheckPin(u.Pin, pin) }); err != nil {
		return "", apiKey{}, err
	}
	exp, err := keyExpiry(expiresMs, time.Now())
	if err != nil {
		return "", apiKey{}, err
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	cur := a.r().cfg
	cl, ok := ownedBy(cur, u.ID, client)
	if !ok {
		return "", apiKey{}, errors.New("you have no API client of that name")
	}
	return a.keys.createCapped(keyOwner{Kind: actorClient, Name: cl.Name}, keyName, exp, cur.API.MaxClientKeys, "")
}

// RevokeOwnClientKey ends a key of one of c's user's clients; any other key reads as no such key.
func (a *App) RevokeOwnClientKey(c *client, id string) (apiKey, error) {
	u, err := a.ownUser(c)
	if err != nil {
		return apiKey{}, err
	}
	cfg := a.r().cfg
	gone := a.keys.dropWhere(func(r keyRec) bool {
		_, mine := ownedBy(cfg, u.ID, r.Owner.Name)
		return r.ID == id && r.Owner.Kind == actorClient && mine
	})
	if len(gone) == 0 {
		return apiKey{}, errors.New("no such key")
	}
	a.endStreams(gone, "key revoked")
	return gone[0], nil
}

func (a *App) serveMyClientSave(c *client, m *inMsg) {
	err := errors.New("nothing to save")
	if m.MyClient != nil {
		err = a.SaveOwnClient(c, *m.MyClient)
	}
	a.apiEvent(connActor(c), "myClientSave", outcome(err), "", err)
	a.reply(c, opResultFor("myClientSave", &m.ID, err))
}

func (a *App) serveMyClientDelete(c *client, m *inMsg) {
	err := a.DeleteOwnClient(c, m.Name)
	a.apiEvent(connActor(c), "myClientDelete", outcome(err), m.Name, err)
	a.reply(c, opResultFor("myClientDelete", &m.ID, err))
}

func (a *App) serveMyClientKey(c *client, m *inMsg) {
	secret, rec, err := a.CreateOwnClientKey(c, m.Name, m.Pin, m.KeyName, m.Expires)
	a.replyKeyCreate(c, "myClientKeyResult", m.ID, secret, rec, err)
}

func (a *App) serveMyClientKeyRevoke(c *client, m *inMsg) {
	k, err := a.RevokeOwnClientKey(c, m.KeyID)
	a.logKeyRevoked(connActor(c), []apiKey{k}, err)
	a.reply(c, opResultFor("myClientKeyRevoke", &m.ID, err))
}

// dropOwnerless removes API clients whose owner is no longer a user, in the same commit, so
// pruneKeys ends their keys with it.
func dropOwnerless(cfg *config.Config) {
	cfg.APIClients = slices.DeleteFunc(slices.Clone(cfg.APIClients), func(c config.APIClient) bool {
		_, ok := cfg.FindUserByID(c.Owner)
		return c.Owner != "" && !ok
	})
}

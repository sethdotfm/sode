package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Key lifecycle messages (ADR 0003). Rights and key lists reach only connections that
// passed the policy table: never the hello or lock state.

var errUnlockFirst = errors.New("unlock with your PIN first")

// keyExpiry turns a request's unix ms into an expiry; 0 means never.
func keyExpiry(ms int64, now time.Time) (time.Time, error) {
	if ms == 0 {
		return time.Time{}, nil
	}
	t := time.UnixMilli(ms).UTC()
	if !t.After(now) {
		return time.Time{}, errors.New("the expiry is already past")
	}
	return t, nil
}

// keyOwnerExists: a key whose user or API client is gone must not outlive it.
func keyOwnerExists(cfg config.Config, o keyOwner) bool {
	switch o.Kind {
	case actorUser:
		_, ok := cfg.FindUserByID(o.User)
		return ok
	case actorClient:
		_, ok := cfg.FindAPIClient(o.Name)
		return ok
	}
	return false
}

type keyCreateMsg struct {
	T   string  `json:"t"` // "keyCreateResult" or "keyCreateForResult"
	ID  int     `json:"id"`
	OK  bool    `json:"ok"`
	Err string  `json:"err,omitempty"`
	Key string  `json:"key,omitempty"` // the secret, shown once
	Rec *apiKey `json:"info,omitempty"`
}

type keyListMsg struct {
	T    string   `json:"t"` // "keyListOwnResult" or "keyListAllResult"
	ID   int      `json:"id"`
	OK   bool     `json:"ok"`
	Keys []apiKey `json:"keys"`
}

// ownUser is the live user c is signed in as, not under the Lock.
func (a *App) ownUser(c *client) (config.User, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.r().cfg
	if !a.userOKMu(c, cfg) {
		return config.User{}, errUnlockFirst
	}
	u, _ := cfg.FindUserByID(c.u.user.Load().ID)
	return u, nil
}

// CreateOwnKey makes a key that acts as c's user. It asks the user's own PIN, counted toward their
// name's wait, so an unattended signed-in browser cannot mint a lasting key.
func (a *App) CreateOwnKey(c *client, pin, keyName string, expiresMs int64) (string, apiKey, error) {
	u, err := a.ownUser(c)
	if err != nil {
		return "", apiKey{}, err
	}
	if !u.HasPin() {
		return "", apiKey{}, errors.New("set a PIN before you make an API key")
	}
	if err := a.tryUserPin(&u, c.ip, func() bool { return config.CheckPin(u.Pin, pin) }); err != nil {
		return "", apiKey{}, err
	}
	exp, err := keyExpiry(expiresMs, time.Now())
	if err != nil {
		return "", apiKey{}, err
	}
	// cfgMu: a PIN change prunes keys under it, so the check and the insert cannot straddle one.
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	cur := a.r().cfg
	if nu, ok := cur.FindUserByID(u.ID); !ok || nu.Pin != u.Pin {
		return "", apiKey{}, errors.New("your PIN changed; unlock again")
	}
	return a.keys.createCapped(userOwner(u), keyName, exp, userKeyMax, pinFP(u.Pin))
}

func (a *App) replyKeyCreate(c *client, t string, id int, secret string, rec apiKey, err error) {
	res := keyCreateMsg{T: t, ID: id, OK: err == nil}
	act := connActor(c)
	if err != nil {
		res.Err = err.Error()
		a.apiEvent(act, "keyCreate", "refused", "", err)
	} else {
		res.Key, res.Rec = secret, &rec
		a.apiEvent(act, "keyCreate", "ok", fmt.Sprintf("key %q made for %s", rec.Name, rec.Owner.Name), nil)
	}
	a.reply(c, res)
}

func (a *App) serveKeyCreate(c *client, m *inMsg) {
	secret, rec, err := a.CreateOwnKey(c, m.Pin, m.KeyName, m.Expires)
	a.replyKeyCreate(c, "keyCreateResult", m.ID, secret, rec, err)
}

func (a *App) serveKeyListOwn(c *client, m *inMsg) {
	u, err := a.ownUser(c)
	res := keyListMsg{T: "keyListOwnResult", ID: m.ID, OK: err == nil, Keys: []apiKey{}}
	if err == nil {
		me := userOwner(u)
		for _, k := range a.keys.list() {
			if k.Owner.is(me) {
				res.Keys = append(res.Keys, k)
			}
		}
		res.Keys = withLiveNames(a.r().cfg, res.Keys)
	}
	a.reply(c, res)
}

// RevokeOwnKey ends one of c's user's keys; another owner's key id reads as no such key.
func (a *App) RevokeOwnKey(c *client, id string) (apiKey, error) {
	u, err := a.ownUser(c)
	if err != nil {
		return apiKey{}, err
	}
	me := userOwner(u)
	if gone := a.keys.dropWhere(func(r keyRec) bool { return r.ID == id && r.Owner.is(me) }); len(gone) > 0 {
		a.endStreams(gone, "key revoked")
		return gone[0], nil
	}
	return apiKey{}, errors.New("no such key")
}

func (a *App) serveKeyRevoke(c *client, m *inMsg) {
	k, err := a.RevokeOwnKey(c, m.KeyID)
	a.logKeyRevoked(connActor(c), []apiKey{k}, err)
	a.reply(c, opResultFor("keyRevoke", &m.ID, err))
}

func (a *App) logKeyRevoked(act Actor, ks []apiKey, err error) {
	if err != nil {
		a.apiEvent(act, "keyRevoke", "refused", "", err)
		return
	}
	for _, k := range ks {
		a.apiEvent(act, "keyRevoke", "ok", fmt.Sprintf("key %q of %s", k.Name, k.Owner.Name), nil)
	}
}

func (a *App) serveKeyListAll(c *client, m *inMsg) {
	a.reply(c, keyListMsg{T: "keyListAllResult", ID: m.ID, OK: true, Keys: withLiveNames(a.r().cfg, a.keys.list())})
}

// CreateKeyFor makes a key for an API client or another user; Administrator only.
func (a *App) CreateKeyFor(c *client, o keyOwner, keyName string, expiresMs int64) (string, apiKey, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, config.RoleAdministrator); err != nil {
		return "", apiKey{}, err
	}
	exp, err := keyExpiry(expiresMs, time.Now())
	if err != nil {
		return "", apiKey{}, err
	}
	cfg := a.r().cfg
	var max int
	var fp string
	switch o.Kind {
	case actorUser:
		u, ok := cfg.FindUserByID(o.User)
		if !ok {
			return "", apiKey{}, errors.New("no such user")
		}
		if !u.HasPin() {
			return "", apiKey{}, fmt.Errorf("%s has no PIN, so holds no API keys: give them a PIN first", u.Name)
		}
		o, max, fp = userOwner(u), userKeyMax, pinFP(u.Pin)
	case actorClient:
		cl, ok := cfg.FindAPIClient(o.Name)
		if !ok {
			return "", apiKey{}, errors.New("no such API client")
		}
		o.Name, max = cl.Name, clientKeyMax
	default:
		return "", apiKey{}, errors.New("a key belongs to a user or an API client")
	}
	return a.keys.createCapped(o, keyName, exp, max, fp)
}

func (a *App) serveKeyCreateFor(c *client, m *inMsg) {
	var o keyOwner
	if m.Owner != nil {
		o = *m.Owner
	}
	secret, rec, err := a.CreateKeyFor(c, o, m.KeyName, m.Expires)
	a.replyKeyCreate(c, "keyCreateForResult", m.ID, secret, rec, err)
}

// RevokeKeys ends the keys match picks; Administrator only.
func (a *App) RevokeKeys(c *client, match func(keyRec) bool) ([]apiKey, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, config.RoleAdministrator); err != nil {
		return nil, err
	}
	gone := a.keys.dropWhere(match)
	a.endStreams(gone, "key revoked")
	return gone, nil
}

func (a *App) serveKeyRevokeAny(c *client, m *inMsg) {
	gone, err := a.RevokeKeys(c, func(r keyRec) bool { return r.ID == m.KeyID })
	if err == nil && len(gone) == 0 {
		err = errors.New("no such key")
	}
	a.logKeyRevoked(connActor(c), gone, err)
	a.reply(c, opResultFor("keyRevokeAny", &m.ID, err))
}

func (a *App) serveKeyRevokeAll(c *client, m *inMsg) {
	var o keyOwner
	if m.Owner != nil {
		o = *m.Owner
	}
	gone, err := a.RevokeKeys(c, func(r keyRec) bool { return r.Owner.is(o) })
	a.logKeyRevoked(connActor(c), gone, err)
	a.reply(c, opResultFor("keyRevokeAll", &m.ID, err))
}

// pruneKeys ends keys whose owner is gone or, for a user, whose PIN changed since was. A user
// rename keeps them (by id); a client rename ends them. Client keys ignore user PINs. Caller holds cfgMu.
func (a *App) pruneKeys(was, now config.Config) {
	gone := a.keys.dropWhere(func(r keyRec) bool {
		if !keyOwnerExists(now, r.Owner) {
			return true
		}
		if r.Owner.Kind == actorUser {
			ou, ok := was.FindUserByID(r.Owner.User)
			nu, _ := now.FindUserByID(r.Owner.User)
			return ok && ou.Pin != nu.Pin
		}
		return false
	})
	a.endStreams(gone, "key ended: its owner changed or was removed")
	a.endSessionKeysAt(now)
	for _, k := range gone {
		why := "its API client was removed or renamed"
		if k.Owner.Kind == actorUser {
			why = "its user's PIN changed, or the user was removed"
		}
		a.apiEvent(sodeActor, "keyEnded", "ok", fmt.Sprintf("key %q of %s ended: %s", k.Name, k.Owner.label(was), why), nil)
	}
}

// expireKeys removes keys past their expiry, one row each. lookup already refuses them.
func (a *App) expireKeys(now time.Time) {
	a.endSessionKeys()
	// Public keys end often and quietly: a row each would bury the log.
	a.endStreams(a.public.expire(now), "key expired")
	exp := a.keys.expire(now)
	a.endStreams(exp, "key expired")
	for _, k := range exp {
		a.apiEvent(sodeActor, "keyExpired", "ok", fmt.Sprintf("key %q of %s", k.Name, k.Owner.Name), nil)
	}
}

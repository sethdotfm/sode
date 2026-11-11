package app

import (
	"errors"
	"fmt"
	"time"
)

// Session keys: a signed-in browser trades its session for a user key so a Session-mode
// panel can call /api/v1 as that user (docs/panels-design.md). The key is bound to the
// session token, so it ends with the sign-out, the token's expiry or a PIN change (ADR
// 0003). It needs no PIN: the signed-in connection is the proof.

const sessionKeyName = "Session"

// SessionKey makes the key for c's session, replacing any earlier one for it. A connection
// with no session token gets a tab token first, so a reload in that tab keeps the key.
func (a *App) SessionKey(c *client, keyName string) (string, apiKey, error) {
	u, err := a.ownUser(c)
	if err != nil {
		return "", apiKey{}, err
	}
	a.mu.Lock()
	good := a.sessionGoodMu(c)
	tok := c.u.tokenMu()
	a.mu.Unlock()
	if !good {
		if a.issueTokenFor(c, true, true) == "" {
			return "", apiKey{}, errUnlockFirst
		}
		a.mu.Lock()
		tok = c.u.tokenMu()
		a.mu.Unlock()
	}
	if keyName == "" {
		keyName = sessionKeyName
	}
	// cfgMu: a PIN change prunes keys under it, so the check and the insert can't straddle one.
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	cur := a.r().cfg
	nu, ok := cur.FindUser(u.Name)
	if !ok || nu.Pin != u.Pin {
		return "", apiKey{}, errors.New("your PIN changed; unlock again")
	}
	if !a.tokens.live(tokenKey(tok), u.Name, cur) {
		return "", apiKey{}, errUnlockFirst
	}
	return a.keys.create(keyOwner{Kind: actorUser, Name: u.Name}, keyName, time.Time{}, 0, pinFP(u.Pin), tokenKey(tok))
}

func (a *App) serveSessionKey(c *client, m *inMsg) {
	secret, rec, err := a.SessionKey(c, m.KeyName)
	a.replyKeyCreate(c, "sessionKeyResult", m.ID, secret, rec, err)
}

// endSessionKeys ends the keys whose session token is gone, expired or no longer the
// owner's. Never with App.mu held (writes the file).
func (a *App) endSessionKeys() {
	cfg := a.r().cfg
	gone := a.keys.dropWhere(func(r keyRec) bool {
		return r.Sess != "" && !a.tokens.live(r.Sess, r.Owner.Name, cfg)
	})
	a.endStreams(gone, "key ended: its session ended")
	for _, k := range gone {
		a.apiEvent(sodeActor, "keyEnded", "ok", fmt.Sprintf("key %q of %s ended: its session ended", k.Name, k.Owner.Name), nil)
	}
}

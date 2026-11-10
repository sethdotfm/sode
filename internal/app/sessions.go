package app

import "time"

// Sessions: tokens issued to signed-in connections and the server-held Lock that rides them
// (docs/adr/0001-server-held-lock.md). users.go has the access state they act on.

// session: the token and Lock that connections signed in with one token share, under
// App.mu. A connection without a token has a session of its own. The signed-in user stays
// per connection (connUser.user, read without mu).
type session struct {
	token  string
	locked bool
	seen   time.Time // last activity (idle.go); zero until first looked at
}

// sessMu: c's session, made on first use so a zero client works. Caller holds mu.
func (c *client) sessMu() *session {
	if c.u.sess == nil {
		c.u.sess = &session{}
	}
	return c.u.sess
}

// Caller holds mu.
func (u *connUser) tokenMu() string {
	if u.sess == nil {
		return ""
	}
	return u.sess.token
}

// Caller holds mu.
func (u *connUser) lockedMu() bool { return u.sess != nil && u.sess.locked }

// adoptMu: c joins the session another connection holds for tok, else starts one. Caller
// holds mu.
func (a *App) adoptMu(c *client, tok string, locked bool) {
	for cl := range a.clients {
		if cl != c && cl.u.sess != nil && cl.u.sess.token == tok {
			c.u.sess = cl.u.sess
			return
		}
	}
	c.u.sess = &session{token: tok, locked: locked}
}

// eachOnSessionMu runs fn for every connection on c's session, c last (c may not be
// registered). The one loop over connections sharing a token. Caller holds mu.
func (a *App) eachOnSessionMu(c *client, fn func(*client)) {
	s := c.sessMu()
	for cl := range a.clients {
		if cl != c && cl.u.sess == s {
			fn(cl)
		}
	}
	fn(c)
}

func (a *App) syncSessionMu(c *client) { a.eachOnSessionMu(c, a.syncAccessMu) }

// Caller holds mu.
func (a *App) signOutMu(cl *client) {
	cl.u.user.Store(nil)
	cl.u.sess = nil
	a.syncAccessMu(cl)
}

// issueToken gives c's signed-in user a token, "" when none applies. wantTab: a tab
// token for a non-persistent user's sign-in with no page lock. A locked session without a
// token gets a tab token so a reload still shows the Lock (ADR 0001). Does disk I/O: never under mu.
func (a *App) issueToken(c *client, wantTab bool) string {
	return a.issueTokenFor(c, wantTab, false)
}

// issueTokenFor: forKey also gives a held-only user a tab token, which Hi still refuses
// after the Lock ends, so only a session key rides on it (sessionkey.go).
func (a *App) issueTokenFor(c *client, wantTab, forKey bool) string {
	a.mu.Lock()
	who := c.u.user.Load()
	s := c.sessMu()
	locked, page, had := s.locked, c.u.page, s.token != ""
	cfg := a.r().cfg
	a.mu.Unlock()
	if who == nil {
		return ""
	}
	u, ok := cfg.FindUser(who.Name)
	if !ok || u.Pin != who.Pin {
		return ""
	}
	tab := false
	switch {
	case locked:
		if u.Persistent || had {
			return "" // a persistent user's Lock rides the ordinary token, or none to give
		}
		tab = true
	case u.Persistent:
	default:
		if !wantTab || (heldOnly(u, page, cfg.Locks) && !forKey) {
			return ""
		}
		tab = true
	}
	t := a.tokens.issue(u, cfg, tab, locked)
	a.mu.Lock()
	// Signed in again, locked or unlocked meanwhile, or a racing call already issued one:
	// forgotten after mu goes (that writes the file).
	stale := c.u.user.Load() != who || c.u.sess != s || s.locked != locked
	if !stale {
		if locked {
			stale = s.token != ""
		} else {
			stale = a.sessionGoodMu(c)
		}
	}
	if !stale {
		at := s.seen
		if at.IsZero() {
			at = a.now()
		}
		c.u.sess = &session{token: t, locked: locked, seen: at}
		a.tokens.seenAt(t, at)
	}
	a.mu.Unlock()
	if stale {
		a.tokens.forget(t)
		return ""
	}
	return t
}

// Caller holds mu.
func (a *App) sessionGoodMu(c *client) bool {
	tok := c.u.tokenMu()
	if tok == "" {
		return false
	}
	_, ok := a.tokens.user(tok, a.r().cfg)
	return ok
}

type sessionMsg struct {
	T     string `json:"t"`
	Token string `json:"token"`
	Tab   bool   `json:"tab,omitempty"` // for this tab only
}

// Renews open connections' tokens so a never-reloading page outlasts tokenMaxAge. Off mu.
func (a *App) touchSessions() {
	a.mu.Lock()
	cfg := a.r().cfg
	var held []string
	for cl := range a.clients {
		if tok := cl.u.tokenMu(); tok != "" && a.userSameMu(cl, cfg) {
			held = append(held, tok)
		}
	}
	a.mu.Unlock()
	a.tokens.touch(cfg, held...)
	a.keys.flush()
}

// Issues tokens to persistent-user connections holding none or a dead one (tokens are
// otherwise issued only at PIN entry, e.g. after Never sign out is ticked or a PIN change).
// Off mu: writes the token file.
func (a *App) catchUpSessions() {
	a.mu.Lock()
	cfg := a.r().cfg
	var todo []*client
	for cl := range a.clients {
		if who := cl.u.user.Load(); who != nil && a.userOKMu(cl, cfg) && !a.sessionGoodMu(cl) {
			if u, ok := cfg.FindUser(who.Name); ok && u.Persistent {
				todo = append(todo, cl)
			}
		}
	}
	a.mu.Unlock()
	for _, cl := range todo {
		if t := a.issueToken(cl, false); t != "" {
			a.reply(cl, sessionMsg{T: "session", Token: t})
		}
	}
}

// setHeld raises (on) or ends the Lock of c's session for every connection on it; the flag
// rides the token (ADR 0001), so a reload comes back locked. Ending it is a resume.
func (a *App) setHeld(c *client, on bool) {
	a.mu.Lock()
	if !a.userSameMu(c, a.r().cfg) {
		a.syncAccessMu(c)
		a.mu.Unlock()
		return
	}
	tok, drop := a.holdMu(c, on)
	a.mu.Unlock()
	a.finishHold(c, on, tok, drop)
}

// holdMu changes the session's Lock and tells its connections. drop: a resume under a PIN
// lock ends a non-persistent token, which existed only to carry the Lock. Caller holds mu;
// the caller then runs finishHold off it.
func (a *App) holdMu(c *client, on bool) (tok string, drop bool) {
	if !on {
		a.touchMu(c, a.now()) // a resume is activity, or the next sweep locks again
	}
	s := c.sessMu()
	tok = s.token
	cfg := a.r().cfg
	if who := c.u.user.Load(); !on && tok != "" && who != nil {
		u, _ := cfg.FindUser(who.Name)
		drop = heldOnly(u, c.u.page, cfg.Locks)
	}
	s.locked = on
	if drop {
		// The token dies, so the connections sharing it part: each keeps its own
		// tokenless session, and a later Lock or sign-out of one cannot reach the rest.
		a.eachOnSessionMu(c, func(cl *client) {
			cl.u.sess = &session{locked: on, seen: s.seen}
			a.syncAccessMu(cl)
		})
		return tok, drop
	}
	if tok != "" {
		a.tokens.setLocked(tok, on)
	}
	a.syncSessionMu(c)
	return tok, drop
}

// finishHold: off mu, because it writes the file. A Lock without a token (non-persistent
// user) gets a tab token here so a reload still shows it.
func (a *App) finishHold(c *client, on bool, tok string, drop bool) {
	switch {
	case drop:
		a.tokens.forget(tok)
		a.endSessionKeys()
	case tok != "":
		a.tokens.save()
	case on:
		if t := a.issueToken(c, true); t != "" {
			a.reply(c, sessionMsg{T: "session", Token: t, Tab: true})
		}
	}
}

// SignOut ends c's session. A reload cannot undo it: the token dies and connections
// sharing it sign out too (ADR 0001).
func (a *App) SignOut(c *client) {
	a.mu.Lock()
	tok := c.u.tokenMu()
	a.eachOnSessionMu(c, a.signOutMu)
	a.mu.Unlock()
	a.tokens.forget(tok)
	a.endSessionKeys()
}

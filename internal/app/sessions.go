package app

// Sessions: tokens issued to signed-in connections and the server-held Lock that rides them
// (docs/adr/0001-server-held-lock.md). users.go has the access state they act on.

// issueSession does disk I/O: never under mu. "" for a non-persistent user.
func (a *App) issueSession(c *client) string { return a.issueSessionTab(c, false) }

// tab: a no-lock login's token for a non-persistent user, marked tab in the store, kept in
// sessionStorage.
func (a *App) issueSessionTab(c *client, tab bool) string {
	who := c.u.user.Load()
	cfg := a.r().cfg
	if who == nil {
		return ""
	}
	a.mu.Lock()
	held := c.u.locked
	a.mu.Unlock()
	if held {
		return "" // a locked session already has its token, or none to give
	}
	u, ok := cfg.FindUser(who.Name)
	if !ok || u.Pin != who.Pin {
		return ""
	}
	if !u.Persistent {
		a.mu.Lock()
		page := c.u.page
		a.mu.Unlock()
		if !tab || pageGates[page].on(cfg.Locks) {
			return ""
		}
	} else {
		tab = false // a persistent user's token is the ordinary one
	}
	t := a.tokens.issue(u, cfg, tab)
	a.mu.Lock()
	// Locked meanwhile, or a racing call already issued one: forgotten after mu goes (that
	// writes the file).
	stale := c.u.user.Load() != who || a.sessionGoodMu(c)
	if !stale {
		c.u.token = t
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
	if c.u.token == "" {
		return false
	}
	_, ok := a.tokens.user(c.u.token, a.r().cfg)
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
		if cl.u.token != "" && a.userSameMu(cl, cfg) {
			held = append(held, cl.u.token)
		}
	}
	a.mu.Unlock()
	a.tokens.touch(cfg, held...)
}

// Issues tokens to persistent-user connections holding none or a dead one (tokens are
// otherwise issued only at PIN entry, e.g. after Never log out is ticked or a PIN change).
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
		if t := a.issueSession(cl); t != "" {
			a.reply(cl, sessionMsg{T: "session", Token: t})
		}
	}
}

// UserLock raises the signed-in user's Lock and keeps the session (ADR 0001). With nobody
// signed in it does nothing: that Lock is page-only.
func (a *App) UserLock(c *client) {
	a.setHeld(c, true)
}

// setHeld: the flag rides the token (ADR 0001), so a reload comes back locked.
func (a *App) setHeld(c *client, on bool) {
	a.mu.Lock()
	if !a.userSameMu(c, a.r().cfg) {
		a.syncAccessMu(c)
		a.mu.Unlock()
		return
	}
	tok := c.u.token
	c.u.locked = on
	if tok != "" {
		for cl := range a.clients {
			if cl != c && cl.u.token == tok {
				cl.u.locked = on
				a.syncAccessMu(cl)
			}
		}
		a.tokens.setLocked(tok, on)
	}
	a.syncAccessMu(c)
	a.mu.Unlock()
	if tok != "" {
		a.tokens.persist()
	} else if on {
		a.issueHeldTab(c)
	}
}

// issueHeldTab gives a held Lock without a token (non-persistent user) a tab token, so a
// reload still shows it. Off mu: writes the token file.
func (a *App) issueHeldTab(c *client) {
	cfg := a.r().cfg
	a.mu.Lock()
	who := c.u.user.Load()
	ok := who != nil && c.u.locked && c.u.token == ""
	a.mu.Unlock()
	if !ok {
		return
	}
	u, found := cfg.FindUser(who.Name)
	if !found || u.Pin != who.Pin || u.Persistent {
		return
	}
	t := a.tokens.issue(u, cfg, true)
	a.mu.Lock()
	stale := c.u.user.Load() != who || !c.u.locked || c.u.token != ""
	if !stale {
		c.u.token = t
		a.tokens.setLocked(t, true)
	}
	a.mu.Unlock()
	if stale {
		a.tokens.forget(t)
		return
	}
	a.tokens.persist()
	a.reply(c, sessionMsg{T: "session", Token: t, Tab: true})
}

// SignOut ends c's session. A reload cannot undo it: the token dies and connections
// sharing it sign out too (ADR 0001).
func (a *App) SignOut(c *client) {
	a.mu.Lock()
	tok := c.u.token
	c.u.user.Store(nil)
	c.u.token = ""
	c.u.locked = false
	if tok != "" {
		for cl := range a.clients {
			if cl != c && cl.u.token == tok {
				cl.u.user.Store(nil)
				cl.u.token = ""
				cl.u.locked = false
				a.syncAccessMu(cl)
			}
		}
	}
	a.syncAccessMu(c)
	a.mu.Unlock()
	a.tokens.forget(tok)
}

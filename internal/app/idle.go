package app

import (
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Server auto-lock: the idle clock belongs to the session, so it runs for every page type
// (main, log, panel) and with no connection at all. The page's own timer stays the fast
// path; this one catches a closed or sleeping browser. docs/agents/users-locks.md, Auto-lock.

const defaultIdleEvery = 10 * time.Second

// idleLimit: how long u may go without activity, 0 = never.
func idleLimit(cfg config.Config, u config.User) time.Duration {
	if cfg.AutoLockMinutes <= 0 || u.NoAutoLock {
		return 0
	}
	return time.Duration(cfg.AutoLockMinutes) * time.Minute
}

// touchMu records activity on c's session and its token. Caller holds mu.
func (a *App) touchMu(c *client, now time.Time) {
	s := c.sessMu()
	s.seen = now
	a.tokens.seenAt(s.token, now)
}

// Activity is the page's ping while its user is active. Not logged: every tab sends one
// about every 30 s.
func (a *App) Activity(c *client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.userOKMu(c, a.r().cfg) {
		a.touchMu(c, a.now())
	}
}

// overdueMu: c's session is signed in, not locked, and idle past its user's limit. The
// token's clock counts too, since a sibling's ping or a closed tab's last one lives there.
// Caller holds mu.
func (a *App) overdueMu(c *client, cfg config.Config, now time.Time) bool {
	if !a.userOKMu(c, cfg) {
		return false
	}
	u, _ := cfg.FindUser(c.u.user.Load().Name)
	lim := idleLimit(cfg, u)
	if lim == 0 {
		return false
	}
	s := c.sessMu()
	last := s.seen
	if s.token != "" {
		if t := a.tokens.lastSeen(s.token); t.After(last) {
			last = t
		}
	}
	if last.IsZero() {
		s.seen = now
		return false
	}
	return now.Sub(last) >= lim
}

type heldJob struct {
	c   *client
	tok string
}

// idleSweep raises the Lock on every idle session, open or not. Open sessions get the
// same lockState push as userLock.
func (a *App) idleSweep() {
	now := a.now()
	a.mu.Lock()
	cfg := a.r().cfg
	open := map[string]bool{}
	done := map[*session]bool{}
	var jobs []heldJob
	for cl := range a.clients {
		s := cl.sessMu()
		if s.token != "" {
			open[tokenKey(s.token)] = true
		}
		if done[s] {
			continue
		}
		done[s] = true
		if a.overdueMu(cl, cfg, now) {
			tok, _ := a.holdMu(cl, true)
			jobs = append(jobs, heldJob{cl, tok})
		}
	}
	save := a.tokens.lockIdle(cfg, now, open)
	a.mu.Unlock()
	for _, j := range jobs {
		a.finishHold(j.c, true, j.tok, false)
	}
	if save {
		a.tokens.save()
	}
}

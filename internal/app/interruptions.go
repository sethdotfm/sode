package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Interruptions (GLOSSARY.md): what a Settings save ends or narrows on the floor. The review asks
// for them (settingsReview). The one Save finds them again before its commit, because after the
// commit the things that ended are gone. A save ends only what a changed field governs (spec
// #543, The one Save). The endings happen in commit and the prune paths.

// interruption: one line of the list. The counts name the Save button (web/savereviewrules.js).
// Ends: something on the line ends for good, so the review says it does not come back.
type interruption struct {
	Text     string `json:"text"`
	Panels   int    `json:"panels,omitempty"`
	Streams  int    `json:"streams,omitempty"`
	Sessions int    `json:"sessions,omitempty"`
	Keys     int    `json:"keys,omitempty"`
	Browsers int    `json:"browsers,omitempty"`
	Ends     bool   `json:"ends,omitempty"`
}

func interruptionTexts(l []interruption) []string {
	var out []string
	for _, x := range l {
		out = append(out, x.Text)
	}
	return out
}

func howMany(n int, one, many string) string { return fmt.Sprintf("%d %s", n, plural(n, one, many)) }

var lockNames = []struct {
	g    gate
	name string
}{{gateMain, "Main page lock"}, {gateLog, "Communication Log page lock"}, {gatePanel, "Panel pages lock"}}

// interruptions: what a commit from live to next ends or narrows now. Caller holds cfgMu, not mu.
func (a *App) interruptions(live, next config.Config) []interruption {
	var out []interruption
	out = append(out, a.publicStops(live, next)...)
	out = append(out, a.keylessStops(live, next)...)
	out = append(out, a.userStops(live, next)...)
	out = append(out, a.lockStops(live, next)...)
	if live.GuestRole != next.GuestRole && config.RoleAtLeast(live.GuestRole, next.GuestRole) {
		out = append(out, interruption{Text: fmt.Sprintf("Guest role goes down from %s to %s.",
			config.RoleLabel(live.GuestRole), config.RoleLabel(next.GuestRole))})
	}
	if live.AutoLockMinutes > 0 && next.AutoLockMinutes == 0 {
		out = append(out, interruption{Text: fmt.Sprintf("This save turns auto-lock off (was %d min).", live.AutoLockMinutes)})
	}
	return out
}

func (a *App) publicStops(live, next config.Config) []interruption {
	if !live.PublicPanels || next.PublicPanels {
		return nil
	}
	var out []interruption
	for _, p := range a.panels.list() {
		if p.Acts != panelActsPublic {
			continue
		}
		name := cmpOr(p.Name, p.ID)
		out = append(out, interruption{Panels: 1, Ends: true, Text: fmt.Sprintf(
			"Public panel %s stops: %s %s it open now. Its page comes back by itself when the panel is Public again.",
			name, howMany(a.mon.liveCount(p.ID), "tablet", "tablets"), plural(a.mon.liveCount(p.ID), "has", "have"))})
	}
	return out
}

// keylessStops follows refreshStreams: a keyless stream ends when the keyless API goes off or it
// loses a scope one of its topics needs. Every other keyless stream narrows to the scopes left.
func (a *App) keylessStops(live, next config.Config) []interruption {
	if !live.API.GuestEnabled {
		return nil
	}
	off := !next.API.GuestEnabled
	removed := slices.ContainsFunc(live.GuestAPIScopes, func(s string) bool { return !slices.Contains(next.GuestAPIScopes, s) })
	if !off && !removed {
		return nil
	}
	has := func(s string) bool { return slices.Contains(next.GuestAPIScopes, s) }
	ends, narrows := 0, 0
	a.mu.Lock()
	for st := range a.streams {
		if st.keyID != "" || st.ended != "" {
			continue
		}
		lost := off || st.detail && !has(config.ScopeLog)
		for t := range st.topics {
			lost = lost || !has(streamTopics[t])
		}
		if lost {
			ends++
		} else {
			narrows++
		}
	}
	a.mu.Unlock()
	switch {
	case off && ends > 0:
		return []interruption{{Streams: ends, Ends: true, Text: "The keyless API goes off: " + howMany(ends, "keyless stream ends.", "keyless streams end.")}}
	case off || ends+narrows == 0:
		return nil
	}
	return []interruption{{Streams: ends, Ends: ends > 0, Text: fmt.Sprintf("Keyless scopes removed: %s, %d narrows to the scopes left.",
		howMany(ends, "keyless stream ends", "keyless streams end"), narrows)}}
}

// userStops: an unticked Never sign out ends that user's sessions, and a PIN reset ends their
// sessions and keys. Ticking Never sign out ends nothing (tokenStore.promoteTabs).
func (a *App) userStops(live, next config.Config) []interruption {
	var out []interruption
	for _, nu := range next.Users {
		ou, ok := live.FindUserByID(nu.ID)
		if !ok {
			continue
		}
		switch {
		case ou.Pin != nu.Pin:
			sess := a.tokens.countLive(live, func(r tokenRec) bool { return r.User == nu.ID })
			keys := a.keys.countWhere(func(r keyRec) bool { return r.Owner.Kind == actorUser && r.Owner.User == nu.ID })
			if sess+keys > 0 {
				out = append(out, interruption{Sessions: sess, Keys: keys, Ends: true, Text: fmt.Sprintf(
					"User %s: the PIN change ends %s and %s.", nu.Name, howMany(sess, "session", "sessions"), howMany(keys, "key", "keys"))})
			}
		case ou.Persistent && !nu.Persistent:
			sess := a.tokens.countLive(live, func(r tokenRec) bool { return r.User == nu.ID && !r.Tab })
			if sess > 0 {
				out = append(out, interruption{Sessions: sess, Ends: true, Text: fmt.Sprintf(
					"User %s: Never sign out goes off and ends %s, with the Session panels on them.", nu.Name, howMany(sess, "session", "sessions"))})
			}
		}
	}
	return out
}

// lockStops: for each lock turned on, Guest browsers and browsers of users without a PIN on that
// page type drop to a lock screen. The users without a PIN cannot open that page type. The Panel
// pages lock also stops the Session panels of users without a PIN (endNoPinSessionKeys).
func (a *App) lockStops(live, next config.Config) []interruption {
	var noPin []string
	for _, u := range next.Users {
		if !u.HasPin() {
			noPin = append(noPin, u.Name)
		}
	}
	var out []interruption
	for _, l := range lockNames {
		if l.g.on(live.Locks) || !l.g.on(next.Locks) {
			continue
		}
		n := 0
		a.mu.Lock()
		for cl := range a.clients {
			if pageGates[cmpOr(cl.u.page, "main")] != l.g {
				continue
			}
			if u := cl.u.user.Load(); u == nil || !u.HasPin() {
				n++
			}
		}
		a.mu.Unlock()
		text := fmt.Sprintf("The %s puts %s on a lock screen: Guest browsers and browsers of users without a PIN.", l.name,
			howMany(n, "browser", "browsers"))
		if len(noPin) > 0 {
			text += " It shuts out the users without a PIN: " + strings.Join(noPin, ", ") + "."
		}
		if n > 0 || len(noPin) > 0 {
			out = append(out, interruption{Browsers: n, Text: text})
		}
		if l.g == gatePanel {
			if k := a.keys.countWhere(func(r keyRec) bool { return noPinSessionKey(r, next) }); k > 0 {
				out = append(out, interruption{Panels: k, Ends: true, Text: fmt.Sprintf(
					"The Panel pages lock stops %s of users without a PIN.", howMany(k, "Session panel", "Session panels"))})
			}
		}
	}
	return out
}

func noPinSessionKey(r keyRec, cfg config.Config) bool {
	if r.Sess == "" || r.Owner.Kind != actorUser {
		return false
	}
	u, ok := cfg.FindUserByID(r.Owner.User)
	return ok && !u.HasPin()
}

// endNoPinSessionKeys: the Panel pages lock went on. A user without a PIN cannot open a panel
// page under it, so their Session panels stop. Never with mu held (writes the file).
func (a *App) endNoPinSessionKeys() {
	cfg := a.r().cfg
	gone := a.keys.dropWhere(func(r keyRec) bool { return noPinSessionKey(r, cfg) })
	a.endStreams(gone, "key ended: the Panel pages lock went on")
	for _, k := range gone {
		a.apiEvent(sodeActor, "keyEnded", "ok", fmt.Sprintf("key %q of %s ended: the Panel pages lock went on", k.Name, k.Owner.label(cfg)), nil)
	}
}

// countLive: the valid tokens under cfg that match.
func (s *tokenStore) countLive(cfg config.Config, match func(tokenRec) bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.toks {
		if _, ok := valid(r, cfg); ok && match(r) {
			n++
		}
	}
	return n
}

// promoteTabs: this commit ticks Never sign out for some users. Each tab token of theirs that an
// open page holds becomes an ordinary token. So the sign-in and the Session panel keys on it go
// on, as ticking Never sign out must end nothing. A tab token no open page holds still
// dies in prune, so it cannot skip a lock later. It returns the token keys it changed. Never
// with App.mu held (writes the file).
func (s *tokenStore) promoteTabs(was, now config.Config, open map[string]bool) map[string]bool {
	ticked := map[string]bool{}
	for _, nu := range now.Users {
		if ou, ok := was.FindUserByID(nu.ID); ok && !ou.Persistent && nu.Persistent && ou.Pin == nu.Pin {
			ticked[nu.ID] = true
		}
	}
	if len(ticked) == 0 {
		return nil
	}
	out := map[string]bool{}
	s.mu.Lock()
	for k, r := range s.toks {
		if r.Tab && ticked[r.User] && open[k] && time.Since(r.At) < tabTokenMaxAge {
			r.Tab, r.At = false, time.Now()
			s.toks[k] = r
			out[k] = true
		}
	}
	s.mu.Unlock()
	if len(out) > 0 {
		s.save()
	}
	return out
}

// openTokens: the token keys that open connections hold. Acquires mu.
func (a *App) openTokens() map[string]bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[string]bool{}
	for cl := range a.clients {
		if tok := cl.u.tokenMu(); tok != "" {
			out[tokenKey(tok)] = true
		}
	}
	return out
}

func (s *keyStore) countWhere(match func(keyRec) bool) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.recs {
		if !r.Setup && match(r) {
			n++
		}
	}
	return n
}

// serveSettingsReview answers the review with the Interruptions of the fields the save would
// write. It writes nothing and asks no PIN recheck, which comes after the review.
func (a *App) serveSettingsReview(c *client, m *inMsg) {
	go func() {
		a.cfgMu.Lock()
		plan, err := a.planSave(c, m.Fields, true)
		var list []interruption
		if err == nil {
			list = a.interruptions(plan.live, plan.next())
		}
		a.cfgMu.Unlock()
		a.reply(c, settingsReviewResult{opResult: opResultFor("settingsReview", &m.ID, err), Interruptions: list})
	}()
}

type settingsReviewResult struct {
	opResult
	Interruptions []interruption `json:"interruptions"`
}

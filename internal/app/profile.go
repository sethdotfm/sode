package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// User profiles (#161): a user's This-device prefs, filter sets and presets,
// kept on the config.User. A profile is never in a hello: the server pushes
// it as a `profile` message to the connections unlocked as that user (and
// only those), whenever their access changes (pushAccessLocked) and after a
// save. What may save one is the policy table (policy.go): the user's own
// connection, else a trusted user or the settings PIN; guardProfile repeats
// the check under cfgMu.

var errNoSuchUser = errors.New("no such user")

// profileMsg is the `profile` the server pushes. An empty Name clears what
// the browser holds (it is no longer logged in as anyone).
type profileMsg struct {
	T    string `json:"t"` // "profile"
	Name string `json:"name"`
	config.Profile
}

// profileReq is a profileSave: the parts to replace. A nil part is kept as
// it is; an empty list (or {}) clears it.
type profileReq struct {
	Name       string
	Prefs      map[string]string
	FilterSets []config.FilterSet
	Presets    []config.Preset
}

// profileGetResult answers profileGet (failures are an opResult of the
// same type).
type profileGetResult struct {
	T       string         `json:"t"` // "profileGetResult"
	ID      int            `json:"id"`
	OK      bool           `json:"ok"`
	Name    string         `json:"name"`
	Profile config.Profile `json:"profile"`
}

// isUserLocked: c is unlocked as the user called name (who still exists with
// the same PIN). Caller holds mu.
func (a *App) isUserLocked(c *client, name string) bool {
	if name == "" || !a.userOKLocked(c, a.r().cfg) {
		return false
	}
	return strings.EqualFold(c.u.user.Load().Name, name)
}

// guardProfile refuses c a save of name's profile unless c is that user or
// trusted (or has the settings PIN). Caller holds cfgMu, so the users can't
// change between this check and the save.
func (a *App) guardProfile(c *client, name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isUserLocked(c, name) && !a.trustedLocked(c) {
		return errLocked
	}
	return nil
}

// pushProfileLocked tells c its profile, if it is unlocked as a user and what
// it holds is not what it was last sent, or clears it if c was sent one and
// no longer is that user. Caller holds mu.
func (a *App) pushProfileLocked(c *client) {
	cfg := a.r().cfg
	var b []byte
	if a.userOKLocked(c, cfg) {
		u, _ := cfg.FindUser(c.u.user.Load().Name)
		b, _ = json.Marshal(profileMsg{T: "profile", Name: u.Name, Profile: u.Profile.Normalized()})
		if string(b) == c.u.profile {
			return
		}
		c.u.profile = string(b)
	} else {
		if c.u.profile == "" {
			return // never sent one, nothing to clear
		}
		b, _ = json.Marshal(profileMsg{T: "profile", Profile: config.Profile{}.Normalized()})
		c.u.profile = ""
	}
	a.deliver(c, b)
}

// ProfileSave replaces the given parts of the named user's profile, and
// returns how many items it was given.
func (a *App) ProfileSave(c *client, req profileReq) (int, error) {
	n := len(req.Prefs) + len(req.FilterSets) + len(req.Presets)
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardProfile(c, req.Name); err != nil {
		return n, err
	}
	var owner string
	_, nr, err := a.update(func(cfg *config.Config) error {
		for i := range cfg.Users {
			u := &cfg.Users[i]
			if !strings.EqualFold(u.Name, strings.TrimSpace(req.Name)) {
				continue
			}
			owner = u.Name
			if req.Prefs != nil {
				u.Profile.Prefs = maps.Clone(req.Prefs)
			}
			if req.FilterSets != nil {
				u.Profile.FilterSets = slices.Clone(req.FilterSets)
			}
			if req.Presets != nil {
				u.Profile.Presets = config.ClonePresets(req.Presets)
			}
			return nil
		}
		return errNoSuchUser
	})
	if err != nil {
		return n, err
	}
	a.mu.Lock()
	a.rt.Store(nr)
	for cl := range a.clients {
		if a.isUserLocked(cl, owner) {
			a.pushProfileLocked(cl)
		}
	}
	a.mu.Unlock()
	return n, nil
}

// serveProfileSave handles a profileSave the policy admitted. The log note
// says whose profile and how many things; never a value.
func (a *App) serveProfileSave(c *client, m *inMsg) {
	n, err := a.ProfileSave(c, profileReq{Name: m.Name, Prefs: m.Prefs, FilterSets: m.FilterSets, Presets: m.Presets})
	note := fmt.Sprintf("%s: %d %s", cleanPresetName(m.Name), n, plural(n, "item", "items"))
	a.replyOpNote(c, "profileSave", m.ID, note, err)
}

// serveProfileGet answers profileGet (policy: trusted or the PIN): any
// user's profile, for the Users editor.
func (a *App) serveProfileGet(c *client, m *inMsg) {
	u, ok := a.r().cfg.FindUser(strings.TrimSpace(m.Name))
	if !ok {
		a.replyOp(c, "profileGet", m.ID, 0, "", errNoSuchUser)
		return
	}
	a.reply(c, profileGetResult{T: "profileGetResult", ID: m.ID, OK: true, Name: u.Name, Profile: u.Profile.Normalized()})
}

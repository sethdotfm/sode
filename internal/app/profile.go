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

// User profiles: a user's This-device prefs, filter sets and presets, kept on config.User.
// Never in a hello: pushed as a profile message only to connections unlocked as that user,
// when access changes (pushAccessLocked) and after a save. Who may save is in policy.go;
// guardProfile repeats it under cfgMu.

var errNoSuchUser = errors.New("no such user")

// profileMsg: empty Name clears what the browser holds.
type profileMsg struct {
	T    string `json:"t"` // "profile"
	Name string `json:"name"`
	config.Profile
}

// profileReq: a nil part is kept; an empty list or {} clears it.
type profileReq struct {
	Name       string
	Prefs      map[string]string
	FilterSets []config.FilterSet
	Presets    []config.Preset
	PinOrder   []string
}

type profileGetResult struct {
	T       string         `json:"t"` // "profileGetResult"
	ID      int            `json:"id"`
	OK      bool           `json:"ok"`
	Name    string         `json:"name"`
	Profile config.Profile `json:"profile"`
}

// Caller holds mu.
func (a *App) isUserLocked(c *client, name string) bool {
	if name == "" || !a.userOKLocked(c, a.r().cfg) {
		return false
	}
	return strings.EqualFold(c.u.user.Load().Name, name)
}

// Caller holds cfgMu, so users can't change between check and save.
func (a *App) guardProfile(c *client, name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isUserLocked(c, name) && !a.trustedLocked(c) {
		return errLocked
	}
	return nil
}

// Caller holds mu.
func (a *App) pushProfileLocked(c *client) {
	rt := a.r()
	cfg := rt.cfg
	var b []byte
	if a.userOKLocked(c, cfg) {
		u, _ := cfg.FindUser(c.u.user.Load().Name)
		if a.profMemoRT != rt || a.profMemo == nil {
			a.profMemoRT, a.profMemo = rt, map[string]string{}
		}
		msg, ok := a.profMemo[u.Name]
		if !ok {
			j, _ := json.Marshal(profileMsg{T: "profile", Name: u.Name, Profile: u.Profile.Normalized()})
			msg = string(j)
			a.profMemo[u.Name] = msg
		}
		if msg == c.u.profile {
			return
		}
		c.u.profile = msg
		b = []byte(msg)
	} else {
		if c.u.profile == "" {
			return // never sent one, nothing to clear
		}
		b, _ = json.Marshal(profileMsg{T: "profile", Profile: config.Profile{}.Normalized()})
		c.u.profile = ""
	}
	a.deliver(c, b)
}

func (a *App) ProfileSave(c *client, req profileReq) (int, error) {
	n := len(req.Prefs) + len(req.FilterSets) + len(req.Presets) + len(req.PinOrder)
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardProfile(c, req.Name); err != nil {
		return n, err
	}
	var owner string
	err := a.commit(func(cfg *config.Config) error {
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
			if req.PinOrder != nil {
				u.Profile.PinOrder = slices.Clone(req.PinOrder)
			}
			return nil
		}
		return errNoSuchUser
	}, func(_, _ *runtime) {
		for cl := range a.clients {
			if a.isUserLocked(cl, owner) {
				a.pushProfileLocked(cl)
			}
		}
	})
	return n, err
}

// Logs whose profile and how many things, never a value.
func (a *App) serveProfileSave(c *client, m *inMsg) {
	n, err := a.ProfileSave(c, profileReq{Name: m.Name, Prefs: m.Prefs, FilterSets: m.FilterSets, Presets: m.Presets, PinOrder: m.PinOrder})
	note := fmt.Sprintf("%s: %d %s", cleanPresetName(m.Name), n, plural(n, "item", "items"))
	a.replyOpNote(c, "profileSave", m.ID, note, err)
}

func (a *App) serveProfileGet(c *client, m *inMsg) {
	u, ok := a.r().cfg.FindUser(strings.TrimSpace(m.Name))
	if !ok {
		a.replyOp(c, "profileGet", m.ID, 0, "", errNoSuchUser)
		return
	}
	a.reply(c, profileGetResult{T: "profileGetResult", ID: m.ID, OK: true, Name: u.Name, Profile: u.Profile.Normalized()})
}

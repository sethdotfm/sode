package app

import (
	"cmp"
	"fmt"
	"regexp"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

const (
	actorUser   = "user"
	actorClient = "client"
	actorSode   = "sode" // Sode itself, for rows nobody caused
)

// Key is the key name, set when the key was made, so it is verified. `Via` is the caller's own text, not verified.
type Actor struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	// User is the id of a user actor, for ownership checks; History keeps only the name.
	User string `json:"-"`
	Key  string `json:"key,omitempty"`
	// Owner is who made a panel's own API client. It only adds a person's name to the client's name.
	Owner string `json:"owner,omitempty"`
	Via   string `json:"via,omitempty"`

	// For Communication Log rows only; unexported, so History keeps its own names. A panel's own key proves panel.
	// panelSaid marks a panel that a page with a user's key named in `X-Sode-Panel`. instance is the page's
	// `X-Sode-Instance`.
	panel, instance string
	panelSaid       bool
}

func panelShort(p Panel) string {
	if p.Slug != "" {
		return p.Slug
	}
	return p.ID
}

// "panel wall (Lobby Wall), instance Front desk"
func panelTag(p Panel, instance string) string {
	s := "panel " + panelShort(p)
	if p.Name != "" {
		s += " (" + p.Name + ")"
	}
	if instance != "" {
		s += ", instance " + instance
	}
	return s
}

// Recorded at the take, so a later rename leaves them. Both are "" for a call no panel page made.
func (a *App) panelNames(act Actor) (panel, instance string) {
	if act.panel == "" {
		return "", ""
	}
	p, ok := a.panels.get(act.panel)
	if !ok {
		p = Panel{ID: act.panel}
	}
	if p.Name != "" {
		return p.Name, act.instance
	}
	return panelShort(p), act.instance
}

// A panel deleted since auth is named by its id.
func (a *App) panelNote(act Actor) string {
	if act.panel == "" {
		return ""
	}
	p, ok := a.panels.get(act.panel)
	if !ok {
		p = Panel{ID: act.panel}
	}
	tag := panelTag(p, act.instance)
	if act.panelSaid {
		tag = "unverified " + tag
	}
	return tag
}

// "Seth (Panel 21)", the bare name with no key, "Sode (system)" for Sode itself.
func (a Actor) String() string {
	if a.Kind == actorSode {
		return a.Name + " (system)"
	}
	if a.Owner != "" {
		return a.Owner + " (" + a.Name + ")"
	}
	if a.Key == "" {
		return a.Name
	}
	return a.Name + " (" + a.Key + ")"
}

// Client carries the actor's text, so nothing that reads it shows an address.
func (e *Entry) setActor(a Actor) {
	e.Actor = &a
	e.Client = a.String()
}

const kindAPI = "api" // an API command, refusal, key event, stream attach or failed sign-in

// apiEvent redacts any key in text, as a last defence.
var keyShape = regexp.MustCompile(keyPrefix + `(?:ses\.)?[A-Za-z0-9_-]{20,}`)

func redactKeys(s string) string { return keyShape.ReplaceAllString(s, "[key]") }

// The one writer of `api` rows. Note is the detail, then the panel tag, then the `Via` text. Never pass a key, hash
// or PIN.
func (a *App) apiEvent(act Actor, why, result, note string, err error) {
	e := event(kindAPI, why, redactKeys(act.String()), result)
	e.Note = redactKeys(note)
	add := func(s string) {
		if e.Note != "" {
			e.Note += "; "
		}
		e.Note += s
	}
	if tag := a.panelNote(act); tag != "" {
		add(redactKeys(tag))
	}
	if act.Via != "" {
		add("unverified via: " + redactKeys(act.Via))
	}
	if err != nil {
		e.Err = redactKeys(err.Error())
	}
	a.logEvent(e)
}

// For rows no connection caused. Its own kind keeps a user named Sode from passing for it in the log.
var sodeActor = Actor{Kind: actorSode, Name: "Sode"}

func connActor(c *client) Actor { return Actor{Kind: actorUser, Name: c.who(), User: c.userID()} }

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (a *App) auditAPISettings(act Actor, wasCfg, nowCfg config.Config) {
	was, now := wasCfg.API, nowCfg.API
	if !sameSet(wasCfg.GuestAPIScopes, nowCfg.GuestAPIScopes) {
		a.apiEvent(act, "keylessScopes", "ok", "keyless scopes: "+cmp.Or(strings.Join(nowCfg.GuestAPIScopes, ", "), "none"), nil)
	}
	if was.GuestEnabled != now.GuestEnabled {
		a.apiEvent(act, "guestApi", onOff(now.GuestEnabled), "", nil)
	}
	if was.LogReads != now.LogReads {
		a.apiEvent(act, "logApiReads", onOff(now.LogReads), "", nil)
	}
	if was.RateLimit != now.RateLimit || was.TakeCap != now.TakeCap {
		a.apiEvent(act, "apiLimits", "ok", fmt.Sprintf("rate limit %d a minute, take cap %d", now.RateLimit, now.TakeCap), nil)
	}
}

func (a *App) auditRequireHTTPS(act Actor, was, now bool) {
	if was != now {
		a.apiEvent(act, "requireHttps", onOff(now), "", nil)
	}
}

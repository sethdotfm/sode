package app

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Actor kinds.
const (
	actorUser   = "user"
	actorClient = "client" // an API client
	actorSode   = "sode"   // Sode itself, for rows nobody caused
)

// Actor is who did something over the API or a user's key. Key is the key name, set when
// the key was made, so it is verified. Via is the caller's own free text: unverified.
type Actor struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
	// Owner is who made a panel's own API client; it only gives the label a person to read.
	Owner string `json:"owner,omitempty"`
	Via   string `json:"via,omitempty"`

	// Which panel page made the call, for Communication Log rows only (unexported, so History keeps
	// its label). panel is the id. A panel's own key proves the panel; panelSaid marks one the page
	// named itself in `X-Sode-Panel`, with a user's key. instance is the page's word (`X-Sode-Instance`),
	// as the keepalive's device name is.
	panel, instance string
	panelSaid       bool
}

// panelShort is how log rows name a panel: its short name, else its id.
func panelShort(p Panel) string {
	if p.Slug != "" {
		return p.Slug
	}
	return p.ID
}

// panelTag is the one wording for a panel in a row's Note: "panel wall (Lobby Wall), instance Front desk".
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

// panelNote is the actor's panel tag, or "" for a call no panel page made. A panel deleted since
// auth is named by its id.
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

// String is "Seth (Panel 21)", or the bare name with no key. Sode itself reads "Sode (system)".
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

// setActor records an API actor on an entry. Client carries the label so nothing that reads
// it shows an address.
func (e *Entry) setActor(a Actor) {
	e.Actor = &a
	e.Client = a.String()
}

const kindAPI = "api" // an API command, refusal, key event, stream attach or failed sign-in

// keyShape matches a key anywhere in text; apiEvent redacts it as a last line of defence.
var keyShape = regexp.MustCompile(keyPrefix + `(?:ses\.)?[A-Za-z0-9_-]{20,}`)

func redactKeys(s string) string { return keyShape.ReplaceAllString(s, "[key]") }

// apiEvent is the one writer of `api` rows: By is the actor and key name, Note the detail, then
// the panel tag, then the via text. Callers never pass a key, hash or PIN; keys are redacted anyway.
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

// sodeActor is the actor on rows no connection caused (a key that expired or died with a PIN).
// Its own kind keeps a user named Sode from passing for it in the log.
var sodeActor = Actor{Kind: actorSode, Name: "Sode"}

// connActor is the signed-in user, or the address, behind a browser connection.
func connActor(c *client) Actor { return Actor{Kind: actorUser, Name: c.who()} }

// auditRights writes a row per actor whose rights, scopes or No auto-lock a users save
// changed, so widened rights can be found later.
func (a *App) auditRights(c *client, was, now config.Config) {
	act := connActor(c)
	row := func(why, note string) { a.apiEvent(act, why, "ok", note, nil) }
	for _, u := range now.Users {
		ou, existed := was.FindUser(u.Name)
		if !reflect.DeepEqual(ou.Rights, u.Rights) && (existed || !u.Rights.IsOpen()) {
			note := "rights of user " + u.Name
			if pc := a.panelClientsOf(now, u); len(pc) > 0 {
				var l []string
				for _, n := range pc {
					l = append(l, n.Client+" ("+n.Name+")")
				}
				note += "; panel clients they created: " + strings.Join(l, ", ")
			}
			row("rightsEdit", note)
		}
		if ou.NoAutoLock != u.NoAutoLock {
			row("noAutoLockEdit", fmt.Sprintf("No auto-lock of %s is now %v", u.Name, u.NoAutoLock))
		}
	}
	for _, cl := range now.APIClients {
		ocl, existed := was.FindAPIClient(cl.Name)
		switch {
		case !existed:
			row("clientEdit", "API client "+cl.Name+" added")
		case !reflect.DeepEqual(ocl.Rights, cl.Rights) || !reflect.DeepEqual(ocl.Scopes, cl.Scopes):
			row("clientEdit", "API client "+cl.Name+" changed")
		}
	}
	for _, ocl := range was.APIClients {
		if _, ok := now.FindAPIClient(ocl.Name); !ok {
			row("clientEdit", "API client "+ocl.Name+" removed")
		}
	}
	if !reflect.DeepEqual(was.GuestRights, now.GuestRights) || !reflect.DeepEqual(was.GuestAPIScopes, now.GuestAPIScopes) {
		row("rightsEdit", "Guest rights or scopes")
	}
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (a *App) auditAPISettings(act Actor, was, now config.APISettings) {
	if was.Enabled != now.Enabled {
		a.apiEvent(act, "apiEnabled", onOff(now.Enabled), "", nil)
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

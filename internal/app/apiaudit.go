package app

import "regexp"

// Actor kinds.
const (
	actorUser   = "user"
	actorClient = "client" // an API client
)

// Actor is who did something over the API or a user's key. Key is the key name, set when
// the key was made, so it is verified. Via is the caller's own free text: unverified.
type Actor struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
	Via  string `json:"via,omitempty"`
}

// String is "Seth (Panel 21)", or the bare name with no key.
func (a Actor) String() string {
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
var keyShape = regexp.MustCompile(keyPrefix + `[A-Za-z0-9_-]{20,}`)

func redactKeys(s string) string { return keyShape.ReplaceAllString(s, "[key]") }

// apiEvent is the one writer of `api` rows: By is the actor and key name, Note the detail.
// Callers never pass a key, hash or PIN; keys are redacted anyway.
func (a *App) apiEvent(act Actor, why, result, note string, err error) {
	e := event(kindAPI, why, redactKeys(act.String()), result)
	e.Note = redactKeys(note)
	if act.Via != "" {
		if e.Note != "" {
			e.Note += "; "
		}
		e.Note += "unverified via: " + redactKeys(act.Via)
	}
	if err != nil {
		e.Err = redactKeys(err.Error())
	}
	a.logEvent(e)
}

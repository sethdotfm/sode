package app

import "strings"

// A cell's or a default's art string. Keep in step with web/panellook.js.
type artRefKind int

const (
	artRefNone    artRefKind = iota // "", or a string that is none of the forms
	artRefIcon                      // icon:<name>, a vendored icon
	artRefLib                       // lib:<id>, an Icon library icon
	artRefDefault                   // d-<hex>, a Default artwork image
	artRefPanel                     // a-<hex>, the panel's own artwork
)

type artRef struct {
	kind artRefKind
	id   string // after "icon:" or "lib:"; the whole d- or a- id
}

// parseArtRef is false for a string that is none of the forms. "" is true, with kind artRefNone.
func parseArtRef(s string) (artRef, bool) {
	switch {
	case s == "":
		return artRef{}, true
	case artIconRe.MatchString(s):
		return artRef{artRefIcon, s[len("icon:"):]}, true
	case artDefaultIDRe.MatchString(s):
		return artRef{artRefDefault, s}, true
	case artIDRe.MatchString(s):
		return artRef{artRefPanel, s}, true
	}
	if id, ok := strings.CutPrefix(s, "lib:"); ok && iconIDRe.MatchString(id) {
		return artRef{artRefLib, id}, true
	}
	return artRef{}, false
}

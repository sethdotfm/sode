package config

import (
	"reflect"
	"slices"
)

// Some settings are saved apart from the Settings form: by Recent's Filters,
// the Presets dialog, the Users dialog, the PIN dialogs.
// Those Config fields carry the tag `owned:"true"`, and a Settings save
// keeps the live values of all of them (KeepOwned) whatever the browser sent,
// so a stale copy can't undo them and a new field can't be forgotten:
// TestEveryFieldChooses makes every field choose.
//
// A field the Settings form saves instead carries `guard:"..."`, its role:
// "router" (a change reconnects to a different router or reads it
// differently: levels, matrix, names), "pin" (only the settings PIN may change
// it: a trusted user may not) or "open" (anyone who may save settings).
// Changed answers whether any field of given roles differs, so the reconnect
// check and the trusted-user guard read the same declarations.

// KeepOwned copies every owned field of from into c.
func (c *Config) KeepOwned(from Config) {
	dst, src := reflect.ValueOf(c).Elem(), reflect.ValueOf(from)
	t := dst.Type()
	for i := range t.NumField() {
		if t.Field(i).Tag.Get("owned") == "true" {
			dst.Field(i).Set(src.Field(i))
		}
	}
}

// Changed reports whether any field whose guard role is one of roles differs
// between a and b. Empty slices count as equal to nil ones.
func Changed(a, b Config, roles ...string) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	t := va.Type()
	for i := range t.NumField() {
		if !slices.Contains(roles, t.Field(i).Tag.Get("guard")) {
			continue
		}
		fa, fb := va.Field(i), vb.Field(i)
		if fa.Kind() == reflect.Slice && fa.Len() == 0 && fb.Len() == 0 {
			continue
		}
		if !reflect.DeepEqual(fa.Interface(), fb.Interface()) {
			return true
		}
	}
	return false
}

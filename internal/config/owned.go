package config

import (
	"reflect"
	"slices"
)

// Fields saved outside the Settings form carry owned:"true"; a Settings save keeps their live values (KeepOwned).
// Form-saved fields carry guard:"router|pin|open": reconnect, only the Advanced Settings PIN, anyone. TestEveryFieldChooses enforces one.

func (c *Config) KeepOwned(from Config) {
	dst, src := reflect.ValueOf(c).Elem(), reflect.ValueOf(from)
	t := dst.Type()
	for i := range t.NumField() {
		if t.Field(i).Tag.Get("owned") == "true" {
			dst.Field(i).Set(src.Field(i))
		}
	}
}

// Empty slices equal nil ones.
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

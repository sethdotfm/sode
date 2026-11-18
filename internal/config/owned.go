package config

import (
	"reflect"
	"slices"
)

// Router's fields count as Config's own (configFields).
// Fields saved outside the Settings form carry owned:"true"; a Settings save keeps their live values (KeepOwned).
// Form-saved fields carry guard:"router|pin|open": reconnect, only the Advanced Settings PIN, anyone. TestEveryFieldChooses enforces one.

func (c *Config) KeepOwned(from Config) {
	dst, src := reflect.ValueOf(c).Elem(), reflect.ValueOf(from)
	for _, f := range configFields() {
		if f.Tag.Get("owned") == "true" {
			dst.FieldByIndex(f.Index).Set(src.FieldByIndex(f.Index))
		}
	}
}

// Empty slices equal nil ones.
func Changed(a, b Config, roles ...string) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	for _, f := range configFields() {
		if !slices.Contains(roles, f.Tag.Get("guard")) {
			continue
		}
		fa, fb := va.FieldByIndex(f.Index), vb.FieldByIndex(f.Index)
		if fa.Kind() == reflect.Slice && fa.Len() == 0 && fb.Len() == 0 {
			continue
		}
		if !reflect.DeepEqual(fa.Interface(), fb.Interface()) {
			return true
		}
	}
	return false
}

// configFields lists Config's fields with the embedded Router's in place of Router itself.
// Not reflect.VisibleFields: Config.Extra hides Router.Extra there.
func configFields() []reflect.StructField {
	var out []reflect.StructField
	t := reflect.TypeOf(Config{})
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.Anonymous {
			out = append(out, f)
			continue
		}
		for j := range f.Type.NumField() {
			rf := f.Type.Field(j)
			rf.Index = []int{i, j}
			out = append(out, rf)
		}
	}
	return out
}

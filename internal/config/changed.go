package config

import "reflect"

// Embedded Router fields count as Config's own; empty slices equal nil ones.
func ReconnectChanged(a, b Config) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	for _, f := range configFields() {
		if f.Tag.Get("reconnect") != "true" {
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

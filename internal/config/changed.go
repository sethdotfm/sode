package config

import "reflect"

// ReconnectChanged: a field that needs a new link changed on some router, or a router came or went. Routers match
// by UID; the first router matches the first router.
func ReconnectChanged(a, b Config) bool {
	ra, rb := a.Routers(), b.Routers()
	if len(ra) != len(rb) || RouterReconnectChanged(a.Router, b.Router) {
		return true
	}
	for _, r := range ra[1:] {
		o, ok := b.RouterByUID(r.UID)
		if !ok || RouterReconnectChanged(r, o) {
			return true
		}
	}
	return false
}

// RouterReconnectChanged: a field of the router that needs a new link (tag reconnect) changed.
func RouterReconnectChanged(a, b Router) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	t := va.Type()
	for i := range t.NumField() {
		if t.Field(i).Tag.Get("reconnect") != "true" {
			continue
		}
		if !reflect.DeepEqual(va.Field(i).Interface(), vb.Field(i).Interface()) {
			return true
		}
	}
	return false
}

// Not reflect.VisibleFields: Config.Extra hides Router.Extra there.
func Fields() []reflect.StructField {
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

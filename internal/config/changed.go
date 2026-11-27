package config

import "reflect"

// ReconnectChanged: a field that needs a new link changed on some router, or a router came or went. Routers match
// by UID; the default router matches the default router.
func ReconnectChanged(a, b Config) bool {
	ra, rb := a.Routers, b.Routers
	if len(ra) != len(rb) || RouterReconnectChanged(a.DefaultRouter(), b.DefaultRouter()) {
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

// Fields gives Config's own fields in order.
func Fields() []reflect.StructField {
	t := reflect.TypeFor[Config]()
	out := make([]reflect.StructField, t.NumField())
	for i := range out {
		out[i] = t.Field(i)
	}
	return out
}

// FieldOf gives the field of c at a Fields index, to read.
func (c Config) FieldOf(index []int) reflect.Value { return reflect.ValueOf(c).FieldByIndex(index) }

// FieldToSet gives the field of c at a Fields index, to write.
func (c *Config) FieldToSet(index []int) reflect.Value {
	return reflect.ValueOf(c).Elem().FieldByIndex(index)
}

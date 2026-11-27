package config

import "reflect"

// ReconnectChanged: a field that needs a new link changed on some router, or a router came or went. Routers match
// by UID, so a new order is no change. A default router whose UID b does not have (a restore gave it a new UID)
// matches the default router of b.
func ReconnectChanged(a, b Config) bool {
	if len(a.Routers) != len(b.Routers) {
		return true
	}
	for _, r := range a.Routers {
		o, ok := b.RouterByUID(r.UID)
		if !ok && r.UID == a.DefaultRouter().UID {
			o, ok = b.DefaultRouter(), true
		}
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

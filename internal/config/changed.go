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

// routersIndex is the index of Config.Routers.
var routersIndex = func() int {
	f, _ := reflect.TypeFor[Config]().FieldByName("Routers")
	return f.Index[0]
}()

// Fields gives Config's fields in order. The fields of the default router come before Routers, since their JSON keys
// are flat keys of Config. Each one has the Index {Routers, its field in Router}. Read and write a field with FieldOf
// and FieldToSet, not with reflect's FieldByIndex.
func Fields() []reflect.StructField {
	var out []reflect.StructField
	t := reflect.TypeFor[Config]()
	for i := range t.NumField() {
		f := t.Field(i)
		if i == routersIndex {
			rt := reflect.TypeFor[Router]()
			for j := range rt.NumField() {
				rf := rt.Field(j)
				rf.Index = []int{i, j}
				out = append(out, rf)
			}
		}
		out = append(out, f)
	}
	return out
}

// FieldOf gives the field of c at a Fields index, to read.
func (c Config) FieldOf(index []int) reflect.Value {
	if len(index) == 2 && index[0] == routersIndex {
		return reflect.ValueOf(c.DefaultRouter()).Field(index[1])
	}
	return reflect.ValueOf(c).FieldByIndex(index)
}

// FieldToSet gives the field of c at a Fields index, to write. A field of the default router is in a copy of the list
// (EditDefaultRouter).
func (c *Config) FieldToSet(index []int) reflect.Value {
	if len(index) == 2 && index[0] == routersIndex {
		return reflect.ValueOf(c.EditDefaultRouter()).Elem().Field(index[1])
	}
	return reflect.ValueOf(c).Elem().FieldByIndex(index)
}

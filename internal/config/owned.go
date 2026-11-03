package config

import "reflect"

// Some settings are saved apart from the Settings form: by Recent's Filters,
// the Salvos dialog, the grid editor, the Users dialog, the PIN dialogs.
// Those Config fields carry the tag `owned:"true"`, and a Settings save
// keeps the live values of all of them (KeepOwned) whatever the browser sent,
// so a stale copy can't undo them and a new field can't be forgotten:
// TestOwnedOrForm makes every field choose.

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

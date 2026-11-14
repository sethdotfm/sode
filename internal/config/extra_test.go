package config

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEveryConfigStructKeepsExtra(t *testing.T) {
	pkg := reflect.TypeOf(Config{}).PkgPath()
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type, string)
	walk = func(typ reflect.Type, path string) {
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(typ.Elem(), path)
			return
		case reflect.Map:
			walk(typ.Key(), path)
			walk(typ.Elem(), path)
			return
		case reflect.Struct:
		default:
			return
		}
		if typ.PkgPath() != pkg || seen[typ] {
			return
		}
		seen[typ] = true
		f, ok := typ.FieldByName("Extra")
		switch {
		case !ok:
			t.Errorf("%s (%s) has no Extra field", typ.Name(), path)
		case f.Type != reflect.TypeOf(map[string]any(nil)):
			t.Errorf("%s.Extra is %s, want map[string]any", typ.Name(), f.Type)
		case f.Tag.Get("yaml") != ",inline" || f.Tag.Get("json") != "-":
			t.Errorf("%s.Extra tags %q, want yaml:\",inline\" json:\"-\"", typ.Name(), f.Tag)
		}
		for i := range typ.NumField() {
			sf := typ.Field(i)
			if sf.Name != "Extra" {
				walk(sf.Type, path+"."+sf.Name)
			}
		}
	}
	walk(reflect.TypeOf(Config{}), "Config")
	if len(seen) < 10 {
		t.Errorf("walked only %d struct types; the walk is missing some", len(seen))
	}
}

const futureYAML = `
router: 127.0.0.1:8910
futureKey: 1
users:
  - name: a
    futureField: x
locks:
  futureLock: true
`

var futureKeys = []string{"futureKey", "locks.futureLock", "users[0].futureField"}

func TestUnknownKeysRoundTrip(t *testing.T) {
	c, err := Parse([]byte(futureYAML))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.UnknownKeys(); !slices.Equal(got, futureKeys) {
		t.Fatalf("UnknownKeys = %q, want %q", got, futureKeys)
	}
	// Load validates, and a user needs a PIN.
	h, err := HashPin("1234")
	if err != nil {
		t.Fatal(err)
	}
	c.Users[0].Pin, c.Users[0].Len = h, 4
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if k := got.UnknownKeys(); !slices.Equal(k, futureKeys) {
		t.Fatalf("after Save and Load UnknownKeys = %q, want %q", k, futureKeys)
	}
	if got.Extra["futureKey"] != 1 || got.Users[0].Extra["futureField"] != "x" || got.Locks.Extra["futureLock"] != true {
		t.Errorf("values changed: %v %v %v", got.Extra, got.Users[0].Extra, got.Locks.Extra)
	}
}

func TestLegacyKeysAreNotUnknown(t *testing.T) {
	const legacy = `
salvos:
  - name: old
    items: [{src: 1, dsts: [2]}]
grids: [1]
gridConfirm: true
hideDsts: [3]
levelTypedBase: 1
locks:
  grid: true
`
	var raw Config
	if err := yaml.Unmarshal([]byte(legacy), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.LegacySalvos) != 1 || raw.LegacyGrids == nil || raw.LegacyGridConfirm == nil ||
		!slices.Equal(raw.HideDsts, []int{3}) || raw.LevelTypedBase != 1 || raw.Locks.LegacyGrid == nil {
		t.Errorf("a legacy key missed its field: %+v", raw)
	}
	c, err := Parse([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if k := c.UnknownKeys(); len(k) != 0 {
		t.Errorf("UnknownKeys = %q, want none", k)
	}
}

package config

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updatePins = flag.Bool("update-pins", false, "write the router shape pins in testdata/pin as the new want")

// pinFile compares got with testdata/pin/name. -update-pins writes it.
func pinFile(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "pin", name)
	if *updatePins {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update-pins to make it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s changed\n got %s\nwant %s", name, got, want)
	}
}

// These keep their shape byte for byte, with one router and with three: the file after Load then Save, and the JSON of
// the settings. That JSON decoded over the defaults also keeps it.
func TestPinRouterShapes(t *testing.T) {
	for _, name := range []string{"one-router", "three-routers"} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", "pin", name+".yaml"))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, src, 0o600); err != nil {
				t.Fatal(err)
			}
			c, _, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := Save(path, c); err != nil {
				t.Fatal(err)
			}
			saved, _ := os.ReadFile(path)
			pinFile(t, name+".saved.yaml", saved)

			j, err := json.MarshalIndent(c, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			pinFile(t, name+".json", append(j, '\n'))

			back := Defaults()
			if err := json.Unmarshal(j, &back); err != nil {
				t.Fatal(err)
			}
			jb, _ := json.MarshalIndent(back, "", "  ")
			pinFile(t, name+".json-decoded.json", append(jb, '\n'))
		})
	}
}

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultLevelsStoredPerRouterUID(t *testing.T) {
	c := Defaults()
	c.Levels, c.DefaultLevels = 4, "1-2"
	c.Normalize()
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	var file struct {
		Routers []map[string]any `yaml:"routers"`
		Rest    map[string]any   `yaml:",inline"`
	}
	if err := yaml.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Routers) != 1 || file.Routers[0]["uid"] != c.UID || file.Routers[0]["defaultLevels"] != "1-2" {
		t.Fatalf("routers %v", file.Routers)
	}
	if _, top := file.Rest["defaultLevels"]; top {
		t.Fatal("defaultLevels is also an install-wide key")
	}
}

func TestRightsStoreRouterReferences(t *testing.T) {
	c := Defaults()
	c.Address = "127.0.0.1:1"
	c.Normalize()
	c.GuestRights = Rights{Sources: &[]Ref{{N: 4, Name: "CAM 5"}}, Dsts: nums(), Presets: &[]string{"8b8db20da8d1c1f6"}}
	c.APIClients = []APIClient{{Name: "Bot", Scopes: []string{ScopeTake}, Rights: Rights{Dsts: nums(2)}}}
	c.Normalize()
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	file := string(b)
	// A UID such as 4080680479828e54 reads as a number, so YAML quotes it.
	file = strings.ReplaceAll(file, `router: "`+c.UID+`"`, "router: "+c.UID)
	for _, want := range []string{"router: " + c.UID, `"n": 4`, "name: CAM 5", "8b8db20da8d1c1f6"} {
		if !strings.Contains(file, want) {
			t.Errorf("file has no %q:\n%s", want, file)
		}
	}
	got, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	src := (*got.GuestRights.Sources)[0]
	if src.Router != c.UID || src.N != 4 || src.Name != "CAM 5" {
		t.Errorf("source came back %+v", src)
	}
	if got.GuestRights.Dsts == nil || len(*got.GuestRights.Dsts) != 0 {
		t.Error("an empty list (nothing allowed) came back as unset (everything)")
	}
	if !got.GuestRights.AllowsSource(Ref{N: 4}) || got.GuestRights.AllowsSource(Ref{N: 5}) || got.GuestRights.AllowsDst(Ref{N: 0}) {
		t.Error("allow checks wrong after load")
	}
	if d := (*got.APIClients[0].Rights.Dsts)[0]; d.Router != c.UID || d.N != 2 {
		t.Errorf("client destination %+v", d)
	}
}

func TestRightsJSONTakesReferences(t *testing.T) {
	var r Rights
	if err := json.Unmarshal([]byte(`{"sources":[{"n":1},{"router":"u","n":2,"name":"X"}],"dsts":[],"levels":[1],"presets":["id1"]}`), &r); err != nil {
		t.Fatal(err)
	}
	if !r.AllowsSource(Ref{N: 1}) || !r.AllowsSource(Ref{N: 2}) || r.AllowsSource(Ref{N: 3}) || r.AllowsDst(Ref{N: 0}) || !r.AllowsPreset("id1") {
		t.Errorf("decoded %+v", r)
	}
	if (*r.Sources)[1].Name != "X" || (*r.Sources)[1].Router != "u" {
		t.Errorf("reference lost %+v", (*r.Sources)[1])
	}
	b, _ := json.Marshal(r)
	if !strings.Contains(string(b), `"sources":[{"router":"","n":1},{"router":"u","n":2,"name":"X"}]`) || !strings.Contains(string(b), `"dsts":[]`) {
		t.Errorf("encoded %s", b)
	}
	var bad Rights
	for _, in := range []string{`{"sources":["x"]}`, `{"sources":[1]}`, `{"dsts":[2]}`} {
		if err := json.Unmarshal([]byte(in), &bad); err == nil {
			t.Errorf("%s decoded", in)
		}
	}
}

func TestRightsWithinIgnoresNames(t *testing.T) {
	r := Rights{Sources: &[]Ref{{Router: "u", N: 1, Name: "OLD"}}}
	of := Rights{Sources: &[]Ref{{Router: "u", N: 1, Name: "NEW"}, {Router: "u", N: 2}}}
	if !r.Within(of) {
		t.Error("a renamed source is outside the cap")
	}
	blank := Rights{Sources: &[]Ref{{N: 1}}}
	if !blank.Within(of) || !of.Within(Rights{Sources: &[]Ref{{N: 1}, {N: 2}}}) {
		t.Error("a blank router is not the one router")
	}
}

func nums(ns ...int) *[]Ref {
	out := make([]Ref, len(ns))
	for i, n := range ns {
		out[i] = Ref{N: n}
	}
	return &out
}

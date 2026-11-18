package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rights store {router, n, name} per source and destination and preset ids; the file shows it.
func TestRightsStoreRouterReferences(t *testing.T) {
	c := Defaults()
	c.Address = "127.0.0.1:1"
	c.Normalize()
	c.GuestRights = Rights{Sources: &[]Ref{{N: 4, Name: "CAM 5"}}, Dsts: Nums(), Presets: &[]string{"8b8db20da8d1c1f6"}}
	c.APIClients = []APIClient{{Name: "Bot", Scopes: []string{ScopeTake}, Rights: Rights{Dsts: Nums(2)}}}
	c.Normalize()
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	file := string(b)
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
	if !got.GuestRights.AllowsSource(4) || got.GuestRights.AllowsSource(5) || got.GuestRights.AllowsDst(0) {
		t.Error("allow checks wrong after load")
	}
	if d := (*got.APIClients[0].Rights.Dsts)[0]; d.Router != c.UID || d.N != 2 {
		t.Errorf("client destination %+v", d)
	}
}

// The wire takes bare numbers or references; it gives references.
func TestRightsJSONTakesNumbersOrReferences(t *testing.T) {
	var r Rights
	if err := json.Unmarshal([]byte(`{"sources":[1,{"router":"u","n":2,"name":"X"}],"dsts":[],"levels":[1],"presets":["id1"]}`), &r); err != nil {
		t.Fatal(err)
	}
	if !r.AllowsSource(1) || !r.AllowsSource(2) || r.AllowsSource(3) || r.AllowsDst(0) || !r.AllowsPreset("id1") {
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
	if err := json.Unmarshal([]byte(`{"sources":["x"]}`), &bad); err == nil {
		t.Error("a text source decoded")
	}
}

// A stored name is for people and import; it never changes what a list allows.
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

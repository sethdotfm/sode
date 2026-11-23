package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const routersYAML = `
format: 2
routers:
  - uid: 7f3c9a1e2b4d5f60
    prefix: A
    title: Studio Ultrix
    address: 10.0.0.50:8910
    statusLabel: name
    matrix: 2
    levels: 5
    levelOffset: 0
    levelNumbering: va
    numberOffset: 0
    defaultLevels: 1-3
    nameLength: 8
    nameRequests: ext
    nameRereadSeconds: 120
    routeRereadSeconds: 600
    fireMode: native
    salvoGroup: 9
brandText: Desk
`

func TestRoutersListLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(routersYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	c, exists, err := Load(path)
	if err != nil || !exists {
		t.Fatalf("Load: %v %v", exists, err)
	}
	want := Router{
		UID: "7f3c9a1e2b4d5f60", Prefix: "A", Title: "Studio Ultrix", Address: "10.0.0.50:8910",
		RouterShow: "name", Matrix: 2, Levels: 5, LevelWireOffset: 0, LevelNumbering: "va", NumberOffset: 0,
		DefaultLevels: "1-3", NameLength: 8, NameRequests: "ext", NameRefreshSeconds: 120,
		TallyResyncSeconds: 600, FireMode: "native", SalvoGroup: 9,
	}
	got := c.Router
	got.Extra = nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("router\n got %+v\nwant %+v", got, want)
	}
	if c.BrandText != "Desk" || len(c.UnknownKeys()) != 0 {
		t.Errorf("brandText %q, unknown %q", c.BrandText, c.UnknownKeys())
	}
}

func TestSaveWritesOneRouterWithUIDAndPrefixA(t *testing.T) {
	c := Defaults()
	c.Address = "10.0.0.50:8910"
	c.Normalize()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	var raw map[string]any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"router", "matrix", "levels", "routerName", "fireMode"} {
		if _, ok := raw[k]; ok {
			t.Errorf("top-level %q in the file; it belongs to the router", k)
		}
	}
	rs, _ := raw["routers"].([]any)
	if len(rs) != 1 {
		t.Fatalf("routers = %v, want one router\n%s", raw["routers"], b)
	}
	r := rs[0].(map[string]any)
	uid, _ := r["uid"].(string)
	if _, title := r["title"]; len(uid) < 16 || !title || r["prefix"] != "A" || r["address"] != "10.0.0.50:8910" {
		t.Errorf("router in file = %v", r)
	}
	got, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.UID != c.UID {
		t.Errorf("UID after Load %q, want %q", got.UID, c.UID)
	}
}

func TestNormalizeMakesUIDOnce(t *testing.T) {
	a, b := Defaults(), Defaults()
	a.Normalize()
	b.Normalize()
	if a.UID == "" || a.UID == b.UID || a.Prefix != "A" {
		t.Fatalf("UIDs %q %q, prefix %q", a.UID, b.UID, a.Prefix)
	}
	uid := a.UID
	a.Address = "10.0.0.9:8910"
	a.Normalize()
	if a.UID != uid {
		t.Errorf("UID changed with the address: %q to %q", uid, a.UID)
	}
}

func TestLoadKeepsAMadeUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("format: 2\nrouters:\n  - address: 10.0.0.50:8910\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.UID == "" || a.UID != b.UID {
		t.Errorf("UIDs %q then %q", a.UID, b.UID)
	}
}

// A file can hold several routers. Each one is read over the defaults, and a save writes them all back in order.
func TestSeveralRouters(t *testing.T) {
	c, err := Parse([]byte("format: 2\nrouters:\n  - uid: a1\n    address: 10.0.0.8:8910\n  - uid: b2\n    prefix: b\n    levels: 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.UID != "a1" || c.Address != "10.0.0.8:8910" || len(c.MoreRouters) != 1 {
		t.Fatalf("routers %+v", c.Routers())
	}
	b := c.MoreRouters[0]
	if b.UID != "b2" || b.Prefix != "B" || b.Levels != 2 || b.Address != "" || b.NameLength != Defaults().NameLength || b.DefaultLevels != "1-2" {
		t.Errorf("second router %+v", b)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("valid routers: %v", err)
	}
	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if rs := back.Routers(); len(rs) != 2 || rs[0].UID != "a1" || rs[1].UID != "b2" || rs[1].Levels != 2 {
		t.Errorf("after a save: %+v", rs)
	}
}

// Two routers cannot share a UID or a prefix. A problem of a router after the first names its prefix.
func TestValidateRouters(t *testing.T) {
	c := Defaults()
	c.MoreRouters = []Router{{Levels: 0, Prefix: "B"}, {Levels: 1, Prefix: "A"}}
	c.Normalize()
	c.MoreRouters[1].UID = c.UID
	err := c.Validate()
	for _, want := range []string{"router B: levels must be 1-256", "router A: two routers have the UID", "router A: two routers have the prefix A"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q in %v", want, err)
		}
	}
}

// A change to the link fields of a router after the first is a reconnect; so is a router that comes or goes.
func TestReconnectChangedSeesEveryRouter(t *testing.T) {
	a := Defaults()
	a.MoreRouters = []Router{{Address: "10.0.0.9:8910", Levels: 1, Prefix: "B"}}
	a.Normalize()
	b := a.Clone()
	b.MoreRouters[0].Title = "Second"
	if ReconnectChanged(a, b) {
		t.Error("a title change reconnects")
	}
	b.MoreRouters[0].Address = "10.0.0.10:8910"
	if !ReconnectChanged(a, b) || !RouterReconnectChanged(a.MoreRouters[0], b.MoreRouters[0]) {
		t.Error("an address change on the second router does not reconnect")
	}
	if RouterReconnectChanged(a.Router, b.Router) {
		t.Error("the first router changed")
	}
	b = a.Clone()
	b.MoreRouters = nil
	if !ReconnectChanged(a, b) {
		t.Error("a router that went does not reconnect")
	}
}

func TestRoutersListsTheFirstRouterFirst(t *testing.T) {
	c := Defaults()
	c.MoreRouters = []Router{{Address: " 10.0.0.9:8910 ", Levels: 1, Prefix: "b"}}
	c.Normalize()
	rs := c.Routers()
	if len(rs) != 2 || rs[0].UID != c.UID || rs[1].Address != "10.0.0.9:8910" || rs[1].Prefix != "B" {
		t.Fatalf("routers %+v", rs)
	}
	if rs[1].UID == "" || rs[1].UID == c.UID || rs[1].DefaultLevels != "1-1" || rs[1].FireMode != "sequential" {
		t.Errorf("second router not normalized: %+v", rs[1])
	}
	for uid, want := range map[string]string{"": c.UID, c.UID: c.UID, rs[1].UID: rs[1].UID} {
		if r, ok := c.RouterByUID(uid); !ok || r.UID != want {
			t.Errorf("RouterByUID(%q) = %q %v", uid, r.UID, ok)
		}
	}
	if _, ok := c.RouterByUID("b0b0"); ok {
		t.Error("RouterByUID of an unknown UID")
	}
	d := c.Clone()
	d.MoreRouters[0].Title = "X"
	if c.MoreRouters[0].Title == "X" {
		t.Error("Clone shares MoreRouters")
	}
}

// A router prefix is one or two letters A to Z.
func TestPrefixRules(t *testing.T) {
	for _, p := range []string{"a", " B ", "ba", "ZZ"} {
		c := Defaults()
		c.Prefix = p
		c.Normalize()
		if err := c.Validate(); err != nil || c.Prefix != strings.ToUpper(strings.TrimSpace(p)) {
			t.Errorf("prefix %q: %q %v", p, c.Prefix, err)
		}
	}
	for _, p := range []string{"ABC", "1", "A1", "Ä"} {
		c := Defaults()
		c.Prefix = p
		c.Normalize()
		if c.Validate() == nil {
			t.Errorf("prefix %q passed", p)
		}
	}
}

func TestRefShowsNoPrefixWithOneRouter(t *testing.T) {
	c := Defaults()
	c.Normalize()
	src := c.SourceRef(22, "CAM 1")
	if !reflect.DeepEqual(src, Ref{Router: c.UID, N: 22, Shown: "23", Name: "CAM 1"}) {
		t.Errorf("source ref %+v", src)
	}
	c.NumberOffset = 0
	if d := c.DestinationRef(0, ""); d.Shown != "0" || d.N != 0 {
		t.Errorf("destination ref %+v", d)
	}
	b, err := yaml.Marshal(src)
	var back map[string]any
	if err == nil {
		err = yaml.Unmarshal(b, &back)
	}
	if err != nil || back["router"] != c.UID || back["n"] != 22 || back["shown"] != "23" || back["name"] != "CAM 1" {
		t.Errorf("ref yaml %q %v", b, err)
	}
}

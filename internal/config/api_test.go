package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A file from before the API loads with it off and nothing restricted.
func TestOldFileLoadsWithAPIOff(t *testing.T) {
	h, err := HashPin("1234")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "router: 10.0.0.1:8910\nusers:\n  - name: Ana\n    pin: " + h + "\n    len: 4\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	c, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.API.Enabled || c.API.GuestEnabled || c.API.LogReads || c.RequireHTTPS {
		t.Errorf("API or Require HTTPS on: %+v", c.API)
	}
	if c.API.RateLimit <= 0 || c.API.TakeCap <= 0 || c.API.MaxUserKeys <= 0 || c.API.MaxClientKeys <= 0 {
		t.Errorf("limits not defaulted: %+v", c.API)
	}
	if !reflect.DeepEqual(c.GuestAPIScopes, DefaultGuestScopes) {
		t.Errorf("guest scopes %v, want %v", c.GuestAPIScopes, DefaultGuestScopes)
	}
	if len(c.APIClients) != 0 || !c.GuestRights.IsOpen() {
		t.Errorf("clients %v, guest rights %+v", c.APIClients, c.GuestRights)
	}
	if u := c.Users[0]; u.NoAutoLock || !u.Rights.IsOpen() {
		t.Errorf("user %+v", u)
	}
}

func TestRightsRoundTrip(t *testing.T) {
	none := []int{}
	c := Defaults()
	c.GuestRights = Rights{Sources: &[]int{1, 2}, Dsts: &none, Visibility: VisHidden}
	c.APIClients = []APIClient{{Name: "Panel bot", Scopes: []string{ScopeNames, ScopeTake}, Rights: Rights{Presets: &[]string{"Cam"}}}}
	c.Normalize()
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.GuestRights.Dsts == nil || len(*got.GuestRights.Dsts) != 0 {
		t.Error("an empty list (nothing allowed) came back as unset (everything)")
	}
	if got.GuestRights.Sources == nil || got.GuestRights.Levels != nil {
		t.Errorf("lists %+v", got.GuestRights)
	}
	if !reflect.DeepEqual(got.APIClients, c.APIClients) {
		t.Errorf("clients %+v", got.APIClients)
	}
}

func TestRightsAllow(t *testing.T) {
	r := Rights{Sources: &[]int{4}, Presets: &[]string{"Cam"}}
	if !r.AllowsSource(4) || r.AllowsSource(5) || !r.AllowsDst(9) || !r.AllowsLevel(1) {
		t.Error("source list or unset lists wrong")
	}
	if !r.AllowsPreset("Cam") || r.AllowsPreset("Other") {
		t.Error("preset list wrong")
	}
	if r.Hidden() || !(Rights{Visibility: VisHidden}).Hidden() {
		t.Error("visibility wrong")
	}
}

func TestCloneSharesNoRights(t *testing.T) {
	c := Defaults()
	c.GuestRights.Sources = &[]int{1}
	c.APIClients = []APIClient{{Name: "a", Rights: Rights{Dsts: &[]int{2}}, Scopes: []string{ScopeNames}}}
	d := c.Clone()
	(*d.GuestRights.Sources)[0] = 9
	(*d.APIClients[0].Rights.Dsts)[0] = 9
	d.APIClients[0].Scopes[0] = "x"
	if (*c.GuestRights.Sources)[0] != 1 || (*c.APIClients[0].Rights.Dsts)[0] != 2 || c.APIClients[0].Scopes[0] != ScopeNames {
		t.Error("clone shares memory")
	}
}

func TestAPIValidation(t *testing.T) {
	cases := map[string]func(*Config){
		"bad visibility":     func(c *Config) { c.GuestRights.Visibility = "secret" },
		"bad scope":          func(c *Config) { c.GuestAPIScopes = []string{"root"} },
		"client no name":     func(c *Config) { c.APIClients = []APIClient{{}} },
		"duplicate client":   func(c *Config) { c.APIClients = []APIClient{{Name: "A"}, {Name: "a"}} },
		"level out of range": func(c *Config) { c.GuestRights.Levels = &[]int{99} },
		"negative source":    func(c *Config) { c.GuestRights.Sources = &[]int{-1} },
		"rate limit":         func(c *Config) { c.API.RateLimit = -1 },
		"take cap":           func(c *Config) { c.API.TakeCap = 0 },
		"https no proxies":   func(c *Config) { c.RequireHTTPS = true },
	}
	for name, mut := range cases {
		c := Defaults()
		c.Levels = 3
		mut(&c)
		c.Normalize()
		if err := c.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	c := Defaults()
	c.RequireHTTPS, c.TrustedProxies = true, []string{"10.0.0.0/8"}
	c.Normalize()
	if err := c.Validate(); err != nil && strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("valid Require HTTPS refused: %v", err)
	}
}

// Clients and Guest rights stay out of the config JSON the hello carries.
func TestRightsStayOutOfConfigJSON(t *testing.T) {
	c := Defaults()
	c.APIClients = []APIClient{{Name: "Secretbot"}}
	c.GuestRights.Sources = &[]int{7}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "Secretbot") || strings.Contains(string(b), "guestRights") || strings.Contains(string(b), "uestApiScopes") {
		t.Errorf("leaks: %s", b)
	}
}

func TestAPIClientNameClashesWithUser(t *testing.T) {
	h, _ := HashPin("1234")
	c := Defaults()
	c.Users = []User{{Name: "Ana", Pin: h, Len: 4}}
	c.APIClients = []APIClient{{Name: "ana"}}
	c.Normalize()
	if err := c.Validate(); err == nil {
		t.Error("an API client named like a user was accepted")
	}
}

package app

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func TestPresetSaveFieldsMatchApply(t *testing.T) {
	for _, p := range config.AccessPresets {
		want := scrambled()
		p.Apply(&want)
		got := scrambled()
		for k, v := range presetSaveFields(p) {
			if err := setField(&got, k, v, &saveCtx{}); err != nil {
				t.Fatalf("%s: %s: %v", p.Name, k, err)
			}
		}
		// The page stages the Guest's Use tie lines right in the Guest Rights it has (stageGuestTieLines).
		if err := setField(&got, "guestRights", guestWithTieLines(t, got.GuestRights, accessPresetViewOf(t, p.Key).GuestTieLines), &saveCtx{}); err != nil {
			t.Fatalf("%s: guestRights: %v", p.Name, err)
		}
		if g, w := valuesOf(got), valuesOf(want); !reflect.DeepEqual(g, w) {
			t.Errorf("%s: staged %+v\nApply %+v", p.Name, g, w)
		}
		if got.API.RateLimit != scrambled().API.RateLimit {
			t.Errorf("%s: the staged api field wrote more than guestEnabled", p.Name)
		}
	}
}

func valuesOf(c config.Config) config.AccessValues {
	return config.AccessValues{Locks: c.Locks, AutoLockMinutes: c.AutoLockMinutes, GuestRole: c.GuestRole, PinWaits: c.PinWaits,
		PublicPanels: c.PublicPanels, KeylessAPI: c.API.GuestEnabled, KeylessScopes: c.GuestAPIScopes, NewPanelActs: c.NewPanelActs,
		NewUserDefaults: config.NewUserDefaults{Persistent: c.NewUserDefaults.Persistent, NoAutoLock: c.NewUserDefaults.NoAutoLock},
		GuestTieLines:   c.GuestRights.AllowsTieLines()}
}

func accessPresetViewOf(t *testing.T, key string) accessPresetView {
	t.Helper()
	for _, v := range accessPresetViews() {
		if v.Key == key {
			return v
		}
	}
	t.Fatalf("no %s preset", key)
	return accessPresetView{}
}

// guestWithTieLines is the guestRights value the page sends: the Guest Rights it has, with the right set.
func guestWithTieLines(t *testing.T, g config.Rights, on bool) json.RawMessage {
	t.Helper()
	g = g.Clone()
	g.TieLines = &on
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// applyPreset sends what the page stages: the preset's fields from usersList, and both flags for each ticked user.
func (p *wsPeer) applyPreset(id int, key string, ticked ...string) map[string]any {
	p.t.Helper()
	list := p.ask("usersList", 1000+id, nil)
	var view *accessPresetView
	b, _ := json.Marshal(list["accessPresets"])
	var views []accessPresetView
	if err := json.Unmarshal(b, &views); err != nil {
		p.t.Fatal(err)
	}
	for i := range views {
		if views[i].Key == key {
			view = &views[i]
		}
	}
	if view == nil {
		p.t.Fatalf("usersList has no %s preset: %v", key, list["accessPresets"])
	}
	vers := map[string]any{}
	for _, src := range []any{p.hello["versions"], list["versions"]} {
		m, _ := src.(map[string]any)
		for k, v := range m {
			vers[k] = v
		}
	}
	fields := map[string]any{}
	for k, v := range view.Fields {
		fields[k] = map[string]any{"value": v, "version": vers[k]}
	}
	var guest config.Rights
	b, _ = json.Marshal(list["guestRights"])
	if err := json.Unmarshal(b, &guest); err != nil {
		p.t.Fatal(err)
	}
	fields["guestRights"] = map[string]any{"value": guestWithTieLines(p.t, guest, view.GuestTieLines), "version": vers["guestRights"]}
	for _, u := range ticked {
		for attr, v := range map[string]bool{"persistent": view.Users.Persistent, "noAutoLock": view.Users.NoAutoLock} {
			k := "user/" + u + "/" + attr
			fields[k] = map[string]any{"value": v, "version": vers[k]}
		}
	}
	return p.ask("settingsSave", id, map[string]any{"fields": fields, "accessPreset": view.Name})
}

func TestApplySecuredAccessNeedsOneRecheck(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	setUsers(t, a, func(c *config.Config) {
		for i := range c.Users {
			c.Users[i].Persistent, c.Users[i].NoAutoLock = true, true
		}
	})
	ana, bo := idOf(a, "Ana"), idOf(a, "Bo")
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	r := p.applyPreset(1, "secured", ana)
	for _, k := range []string{"locks", "publicPanels", "newPanelActs", "user/" + ana + "/persistent"} {
		wantRefused(t, r, k, "needs PIN recheck")
	}
	passed(t, "recheck", p.recheck("2468"))
	r = p.applyPreset(2, "secured", ana)
	res, _ := r["results"].(map[string]any)
	for k, v := range res {
		if s := v.(map[string]any)["status"]; s != "saved" && s != "unchanged" {
			t.Errorf("%s: %v", k, v)
		}
	}
	secured, _ := config.AccessPresetByName("secured")
	cfg := a.r().cfg
	if got := valuesOf(cfg); !reflect.DeepEqual(got, valuesOf(withPreset(cfg, secured))) {
		t.Fatalf("after the save %+v", got)
	}
	u, _ := cfg.FindUserByID(ana)
	if u.Persistent || u.NoAutoLock {
		t.Fatalf("ticked Ana kept Persistent %v, No auto-lock %v", u.Persistent, u.NoAutoLock)
	}
	u, _ = cfg.FindUserByID(bo)
	if !u.Persistent || !u.NoAutoLock {
		t.Fatal("unticked Bo lost Persistent or No auto-lock")
	}
	rows := waitSaves(t, a, 1)
	if got := rows[len(rows)-1].Save.Preset; got != "Secured access" {
		t.Fatalf("Settings saves row preset %q", got)
	}
}

func withPreset(c config.Config, p config.AccessPreset) config.Config {
	n := c.Clone()
	p.Apply(&n)
	return n
}

func TestApplyPresetKeepsTheSaverSignedIn(t *testing.T) {
	a := userApp(t, false)
	setUsers(t, a, func(c *config.Config) {
		for i := range c.Users {
			c.Users[i].Persistent = true
		}
	})
	ada := idOf(a, "Ada")
	p := dialPeer(t, a)
	p.signInRechecked("Ada", "2468")
	wantSaved(t, p.applyPreset(1, "secured", ada), "user/"+ada+"/persistent")
	if u, _ := a.r().cfg.FindUserByID(ada); u.Persistent {
		t.Fatal("Persistent did not go off")
	}
	if r := p.ask("usersList", 2, nil); r["ok"] != true {
		t.Fatalf("the saver is signed out: %v", r)
	}
}

func TestApplyOpenAfterSecuredDeletesNothing(t *testing.T) {
	a := userApp(t, false)
	ana := idOf(a, "Ana")
	setUsers(t, a, func(c *config.Config) {
		c.APIClients = append(c.APIClients, config.APIClient{Name: "Wall", Scopes: []string{config.ScopeTally}})
		for i := range c.Users {
			if c.Users[i].ID == ana {
				c.Users[i].Rights = config.Rights{Dsts: nums(1, 2)}
			}
		}
	})
	if _, err := a.panels.create(Panel{Name: "Desk", Owner: ana}); err != nil {
		t.Fatal(err)
	}
	_, client, err := a.keys.createCapped(keyOwner{Kind: actorClient, Name: "Wall"}, "Wall key", time.Time{}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	_, user, err := a.keys.createCapped(ownerOf(a, "Ana"), "Ana key", time.Time{}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	before := a.r().cfg
	p := dialPeer(t, a)
	p.signInRechecked("Ada", "2468")
	for i, key := range []string{"secured", "open"} {
		if r := p.applyPreset(i+1, key, ana); r["ok"] != true {
			t.Fatalf("%s: %v", key, r)
		}
	}
	after := a.r().cfg
	if len(after.Users) != len(before.Users) || len(after.APIClients) != len(before.APIClients) {
		t.Fatalf("users %d to %d, API clients %d to %d", len(before.Users), len(after.Users), len(before.APIClients), len(after.APIClients))
	}
	if u, _ := after.FindUserByID(ana); !reflect.DeepEqual(u.Rights, mustUser(before, ana).Rights) {
		t.Fatalf("Ana's Rights changed: %+v", u.Rights)
	}
	if len(a.panels.list()) != 1 {
		t.Fatal("the panel is gone")
	}
	for _, k := range []apiKey{client, user} {
		if !a.keys.alive(k.ID) {
			t.Fatalf("key %s ended", k.Name)
		}
	}
	if !after.API.GuestEnabled || after.GuestRole != config.RoleTrusted {
		t.Fatal("Open access did not save")
	}
}

func mustUser(c config.Config, id string) config.User {
	u, _ := c.FindUserByID(id)
	return u
}

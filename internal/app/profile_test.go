package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// profiles drains c's queue and returns the profile messages in it.
func profiles(c *client) []profileMsg {
	var out []profileMsg
	for {
		select {
		case b := <-c.send:
			var m profileMsg
			if json.Unmarshal(b, &m) == nil && m.T == "profile" {
				out = append(out, m)
			}
		default:
			return out
		}
	}
}

func testPreset(name string) config.Preset {
	return config.Preset{Name: name, Items: []config.PresetItem{{Src: 1, Dsts: []int{2}, Levels: []int{1}}}}
}

func anasProfile(t *testing.T, a *App) config.Profile {
	t.Helper()
	u, _ := a.r().cfg.FindUser("Ana")
	return u.Profile
}

func TestProfileSelfSave(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	anaTwo := watched(t, a, "10.0.0.2", "1234") // her other browser
	bo := watched(t, a, "10.0.0.3", "567890")
	anon := watched(t, a, "10.0.0.4", "")
	profiles(ana)
	profiles(anaTwo)
	profiles(bo)

	if err := admitMsg(a, ana, inMsg{T: "profileSave", Name: "Ana"}); err != nil {
		t.Fatalf("own profile refused for an untrusted user: %v", err)
	}
	n, err := a.ProfileSave(ana, profileReq{
		Name:       "ana", // case does not matter
		Prefs:      map[string]string{"theme": "dark", "flip": "on"},
		FilterSets: []config.FilterSet{{Name: "Mine", Sode: true, Ext: true}},
		Presets:    []config.Preset{testPreset("Open")},
	})
	if err != nil || n != 4 {
		t.Fatalf("save: n=%d err=%v", n, err)
	}
	p := anasProfile(t, a)
	if p.Prefs["theme"] != "dark" || len(p.FilterSets) != 1 || len(p.Presets) != 1 || p.Presets[0].Pinned == nil {
		t.Fatalf("profile %+v", p)
	}
	for _, c := range []*client{ana, anaTwo} {
		got := profiles(c)
		if len(got) != 1 || got[0].Name != "Ana" || got[0].Prefs["theme"] != "dark" || got[0].Presets[0].Name != "Open" {
			t.Fatalf("Ana's connection got %+v", got)
		}
	}
	if got := profiles(bo); len(got) != 0 {
		t.Fatalf("Bo was sent Ana's profile: %+v", got)
	}
	if got := profiles(anon); len(got) != 0 {
		t.Fatalf("an anonymous connection was sent a profile: %+v", got)
	}
	// It is on disk too.
	cfg, _, err := config.Load(a.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if u, _ := cfg.FindUser("Ana"); u.Profile.Prefs["flip"] != "on" || len(u.Profile.Presets) != 1 {
		t.Fatalf("saved file %+v", u.Profile)
	}
}

func TestProfileOmittedPartsKept(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"},
		FilterSets: []config.FilterSet{{Name: "Mine"}}, Presets: []config.Preset{testPreset("Open")}}); err != nil {
		t.Fatal(err)
	}
	// Only prefs: filter sets and presets stay.
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"clock": "12"}}); err != nil {
		t.Fatal(err)
	}
	p := anasProfile(t, a)
	if p.Prefs["clock"] != "12" || p.Prefs["theme"] != "" || len(p.FilterSets) != 1 || len(p.Presets) != 1 {
		t.Fatalf("after a prefs-only save: %+v", p)
	}
	// An empty list clears just that part.
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Presets: []config.Preset{}}); err != nil {
		t.Fatal(err)
	}
	p = anasProfile(t, a)
	if len(p.Presets) != 0 || len(p.FilterSets) != 1 || p.Prefs["clock"] != "12" {
		t.Fatalf("after clearing presets: %+v", p)
	}
}

func TestProfileValidation(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	for name, req := range map[string]profileReq{
		"bad pref value": {Name: "Ana", Prefs: map[string]string{"theme": "purple"}},
		"duplicate sets": {Name: "Ana", FilterSets: []config.FilterSet{{Name: "A"}, {Name: "a"}}},
		"empty preset":   {Name: "Ana", Presets: []config.Preset{{Name: "X"}}},
		"unnamed preset": {Name: "Ana", Presets: []config.Preset{testPreset("")}},
		"unknown user":   {Name: "Nobody", Prefs: map[string]string{}},
		"too many presets": {Name: "Ana", Presets: func() []config.Preset {
			var out []config.Preset
			for i := range config.MaxPresets + 1 {
				out = append(out, testPreset(strings.Repeat("p", 1)+string(rune('a'+i))))
			}
			return out
		}()},
	} {
		if _, err := a.ProfileSave(ana, req); err == nil || errors.Is(err, errLocked) && req.Name == "Ana" {
			t.Errorf("%s: err=%v", name, err)
		}
	}
	// An unknown pref name is dropped, the rest kept.
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"bogus": "x", "theme": "light"}}); err != nil {
		t.Fatal(err)
	}
	if p := anasProfile(t, a); len(p.Prefs) != 1 || p.Prefs["theme"] != "light" {
		t.Fatalf("prefs %+v", p.Prefs)
	}
}

func TestProfileTrustedSavesAnothers(t *testing.T) {
	a := userApp(t, false)
	setFlags(t, a, "Bo", true, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	bo := watched(t, a, "10.0.0.3", "567890")
	profiles(ana)
	profiles(bo)
	if err := admitMsg(a, bo, inMsg{T: "profileSave", Name: "Ana"}); err != nil {
		t.Fatalf("trusted user refused: %v", err)
	}
	if err := admitMsg(a, bo, inMsg{T: "profileGet", Name: "Ana"}); err != nil {
		t.Fatalf("trusted profileGet refused: %v", err)
	}
	if _, err := a.ProfileSave(bo, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	if got := profiles(ana); len(got) != 1 || got[0].Prefs["theme"] != "dark" {
		t.Fatalf("Ana was sent %+v", got)
	}
	if got := profiles(bo); len(got) != 0 {
		t.Fatalf("the trusted editor was sent Ana's profile: %+v", got)
	}
	// Untrusting takes effect at once.
	setFlags(t, a, "Bo", false, false)
	if _, err := a.ProfileSave(bo, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "light"}}); !errors.Is(err, errLocked) {
		t.Fatalf("untrusted again: %v", err)
	}
	// The settings PIN works for anyone's.
	c := &client{ip: "10.0.0.9"}
	if _, err := a.ProfileSave(c, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "light"}}); !errors.Is(err, errLocked) {
		t.Fatalf("anonymous: %v", err)
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ProfileSave(c, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "light"}}); err != nil {
		t.Fatalf("settings PIN: %v", err)
	}
	if anasProfile(t, a).Prefs["theme"] != "light" {
		t.Fatal("PIN save did not land")
	}
}

func TestProfileUntrustedRefused(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	bo := watched(t, a, "10.0.0.3", "567890") // not trusted
	anon := &client{ip: "10.0.0.9"}
	for who, c := range map[string]*client{"Bo": bo, "anonymous": anon} {
		if err := admitMsg(a, c, inMsg{T: "profileSave", Name: "Ana"}); !errors.Is(err, errLocked) {
			t.Errorf("%s: profileSave admitted: %v", who, err)
		}
		if err := admitMsg(a, c, inMsg{T: "profileGet", Name: "Ana"}); !errors.Is(err, errLocked) {
			t.Errorf("%s: profileGet admitted: %v", who, err)
		}
		if _, err := a.ProfileSave(c, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); !errors.Is(err, errLocked) {
			t.Errorf("%s: ProfileSave: %v", who, err)
		}
	}
	// An empty name is nobody's own.
	if err := admitMsg(a, ana, inMsg{T: "profileSave"}); !errors.Is(err, errLocked) {
		t.Errorf("empty name admitted: %v", err)
	}
	// Ana can't write Bo's either.
	if _, err := a.ProfileSave(ana, profileReq{Name: "Bo", Prefs: map[string]string{"theme": "dark"}}); !errors.Is(err, errLocked) {
		t.Errorf("Ana saving Bo's: %v", err)
	}
	if !anasProfile(t, a).Empty() {
		t.Fatal("a refused save changed the profile")
	}
	// A refusal answers as a failed result echoing the id, with locked.
	c := watched(t, a, "10.0.0.7", "")
	rl := policy["profileSave"].forMsg(&inMsg{T: "profileSave", Name: "Ana", ID: 7})
	m := inMsg{T: "profileSave", Name: "Ana", ID: 7}
	a.refuse(c, "profileSave", rl, &m, errLocked)
	var res opResult
	select {
	case b := <-c.send:
		json.Unmarshal(b, &res)
	default:
		t.Fatal("no reply")
	}
	if res.T != "profileSaveResult" || res.OK || res.ID == nil || *res.ID != 7 || res.Locked == nil || !*res.Locked {
		t.Fatalf("refusal %+v", res)
	}
}

func TestProfileGet(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.9", send: make(chan []byte, 4)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	a.serveProfileGet(c, &inMsg{T: "profileGet", Name: "ana", ID: 5})
	var res profileGetResult
	json.Unmarshal(<-c.send, &res)
	if !res.OK || res.ID != 5 || res.Name != "Ana" || res.Profile.Prefs["theme"] != "dark" {
		t.Fatalf("result %+v", res)
	}
	a.serveProfileGet(c, &inMsg{T: "profileGet", Name: "Zed", ID: 6})
	var bad opResult
	json.Unmarshal(<-c.send, &bad)
	if bad.T != "profileGetResult" || bad.OK {
		t.Fatalf("unknown user: %+v", bad)
	}
}

// The profile follows the row's Was through a rename, with or without a new
// PIN, and goes with a removed user.
func TestProfileRenameKeepsAndDeleteDrops(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"},
		Presets: []config.Preset{testPreset("Open")}}); err != nil {
		t.Fatal(err)
	}
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	// Rename, PIN kept.
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Anna", Was: "Ana"}, {Name: "Bo", Was: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	u, ok := a.r().cfg.FindUser("Anna")
	if !ok || u.Profile.Prefs["theme"] != "dark" || len(u.Profile.Presets) != 1 {
		t.Fatalf("after rename: %+v", u)
	}
	// Rename with a new PIN.
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ann", Was: "Anna", Pin: "4321"}, {Name: "Bo", Was: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	u, _ = a.r().cfg.FindUser("Ann")
	if u.Profile.Prefs["theme"] != "dark" {
		t.Fatalf("after rename with a new PIN: %+v", u)
	}
	// A new user of the old name starts empty; a profile is kept only once.
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ann", Was: "Ann"}, {Name: "Ana", Pin: "1357"}, {Name: "Bo", Was: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	if u, _ := a.r().cfg.FindUser("Ana"); !u.Profile.Empty() {
		t.Fatalf("a new user inherited %+v", u.Profile)
	}
	// Removed: gone, and a user added later under the name starts empty.
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Bo", Was: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Bo", Was: "Bo"}, {Name: "Ann", Pin: "4321"}}}); err != nil {
		t.Fatal(err)
	}
	if u, _ := a.r().cfg.FindUser("Ann"); !u.Profile.Empty() {
		t.Fatalf("a removed user's profile came back: %+v", u.Profile)
	}
	raw, _ := json.Marshal(a.r().cfg)
	if strings.Contains(string(raw), "profile") {
		t.Fatalf("config JSON carries a profile: %s", raw)
	}
}

// A rename relocks nothing it shouldn't and the renamed user's connection
// is told the new name's profile; a removed user's connection is cleared.
func TestProfilePushOnAccessChange(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	profiles(ana)
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	// Removing Ana relocks her: her browser is told to forget the profile.
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Bo", Was: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	got := profiles(ana)
	if len(got) != 1 || got[0].Name != "" || len(got[0].Prefs) != 0 {
		t.Fatalf("after removal: %+v", got)
	}
	// Logging out clears too, and logging in sends it (hi/unlock paths).
	bo := watched(t, a, "10.0.0.3", "567890")
	if got := profiles(bo); len(got) != 1 || got[0].Name != "Bo" {
		t.Fatalf("login: %+v", got)
	}
	a.UserLock(bo)
	if got := profiles(bo); len(got) != 1 || got[0].Name != "" {
		t.Fatalf("lock: %+v", got)
	}
	a.UserLock(bo) // nothing held: nothing sent
	if got := profiles(bo); len(got) != 0 {
		t.Fatalf("second lock: %+v", got)
	}
}

// Never in a hello, and a profile's contents never reach a connection that
// isn't that user's (nor the config JSON).
func TestProfileNeverInHello(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"},
		FilterSets: []config.FilterSet{{Name: "SecretSet"}}, Presets: []config.Preset{testPreset("SecretPreset")}}); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	b := a.hello(helloPage{})
	a.mu.Unlock()
	for _, s := range []string{"SecretSet", "SecretPreset", "profile", `"theme":"dark"`} {
		if strings.Contains(string(b), s) {
			t.Fatalf("hello mentions %q: %s", s, b)
		}
	}
	// A config applied by someone else (hello to all) leaks nothing to Bo.
	bo := watched(t, a, "10.0.0.3", "567890")
	anon := watched(t, a, "10.0.0.4", "")
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana", Was: "Ana"}, {Name: "Bo", Was: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	for who, c := range map[string]*client{"Bo": bo, "anonymous": anon} {
		for {
			select {
			case m := <-c.send:
				var pm profileMsg
				json.Unmarshal(m, &pm)
				if strings.Contains(string(m), "SecretSet") || pm.T == "profile" && pm.Name != "Bo" {
					t.Fatalf("%s was sent: %s", who, m)
				}
				continue
			default:
			}
			break
		}
	}
}

// A profileSave with every part at its largest still fits the socket, and
// the cap refuses what would not.
func TestProfileSaveFitsReadLimit(t *testing.T) {
	cfg := maxOwned(t)
	size := func(v any) int { b, _ := json.Marshal(v); return len(b) }
	p := config.Profile{Prefs: map[string]string{"theme": "dark"}, FilterSets: cfg.FilterSets}
	c := cfg.Clone()
	c.FilterSets, c.Presets = nil, nil
	c.Users = []config.User{mkUser(t, "Ana", "1234")}
	c.Users[0].Profile = p
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("max filter sets as a profile: %v", err)
	}
	n := size(map[string]any{"t": "profileSave", "id": 1<<31 - 1, "name": strings.Repeat("N", 24),
		"prefs": p.Prefs, "filterSets": p.FilterSets})
	t.Logf("profileSave with max filter sets: %d bytes", n)
	if n >= wsReadLimit {
		t.Fatalf("profileSave is %d bytes, over the %d read limit", n, wsReadLimit)
	}
	// Both lists at their global maximum together are over the profile cap.
	c.Users[0].Profile.Presets = cfg.Presets
	c.Normalize()
	if err := c.Validate(); err == nil {
		t.Fatal("a profile over the size cap was accepted")
	}
}

func TestProfileSaveLogsCommand(t *testing.T) {
	a := userApp(t, false)
	stop := startLog(t, a)
	defer stop()
	ana := watched(t, a, "10.0.0.1", "1234")
	a.serveProfileSave(ana, &inMsg{T: "profileSave", Name: "Ana", ID: 3, Prefs: map[string]string{"theme": "dark", "clock": "12"}})
	var row Sent
	for _, e := range waitLogged(t, a, 1) {
		if e.Kind == kindCommand && e.Why == "profileSave" {
			row = e
		}
	}
	if row.Result != "ok" || row.Note != "Ana: 2 items" || row.By != "Ana" {
		t.Fatalf("logged %+v", row)
	}
	if strings.Contains(row.Note, "dark") {
		t.Fatal("a value was logged")
	}
}

// Removing a user and adding a new one of the same name in one save gives the
// new one an empty profile: only a row with a Was (or one keeping its PIN)
// carries a profile over.
func TestProfileNotInheritedBySameNameNewRow(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	// Ana removed and a new Ana (no Was, new PIN) added in the same save.
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana", Pin: "9753"}, {Name: "Bo", Was: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	u, _ := a.r().cfg.FindUser("Ana")
	if len(u.Profile.Prefs) != 0 {
		t.Fatalf("new Ana inherited the old profile: %+v", u.Profile)
	}
}

// The pinned presets' order is part of the profile: saved by the user, kept
// when other parts are saved, and cleaned (no repeats; keys kept as sent).
func TestProfileSavePinOrder(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", PinOrder: []string{"local:b", "global:a", "local:b"}}); err != nil {
		t.Fatal(err)
	}
	u, _ := a.r().cfg.FindUser("Ana")
	if got := u.Profile.Normalized().PinOrder; len(got) != 2 || got[0] != "local:b" || got[1] != "global:a" {
		t.Fatalf("pin order %v", got)
	}
	// A save of another part leaves it alone.
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	u, _ = a.r().cfg.FindUser("Ana")
	if len(u.Profile.Normalized().PinOrder) != 2 {
		t.Fatalf("pin order lost: %v", u.Profile.PinOrder)
	}
}

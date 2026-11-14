package app

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

const futurePrefs = `users:
  - name: a
    profile:
      prefs:
        theme: dark
        futurePref: x
  - name: b
`

func TestUnknownPrefSurvivesLoadAndSave(t *testing.T) {
	a := futureApp(t, futurePrefs)
	want := []string{"users[0].profile.prefs.futurePref"}
	if k := a.r().cfg.UnknownKeys(); !slices.Equal(k, want) {
		t.Fatalf("UnknownKeys after load = %q, want %q", k, want)
	}
	ana := watched(t, a, "10.0.0.1", "1234")
	profiles(ana)
	// A browser can't add an unknown pref, only keep the one the file has.
	req := profileReq{Name: "Ana", Prefs: map[string]string{"theme": "light", "browserPref": "y", "futurePref": "z"}}
	if _, err := a.ProfileSave(ana, req); err != nil {
		t.Fatal(err)
	}
	c, _, err := config.Load(a.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if k := c.UnknownKeys(); !slices.Equal(k, want) {
		t.Fatalf("UnknownKeys after save = %q, want %q", k, want)
	}
	if p := c.Users[0].Profile.Prefs; p["theme"] != "light" || p["futurePref"] != "x" {
		t.Fatalf("saved prefs = %v", p)
	}
	msgs := profiles(ana)
	if len(msgs) == 0 {
		t.Fatal("no profile message after the save")
	}
	for _, m := range msgs {
		if _, ok := m.Profile.Prefs["futurePref"]; ok {
			t.Fatalf("profile message carries an unknown pref: %v", m.Profile.Prefs)
		}
	}
}

func TestUnknownPrefNamedOnStartRow(t *testing.T) {
	a := futureApp(t, futurePrefs)
	var errs []string
	for _, e := range serverRows(t, a) {
		if e.Err != "" {
			errs = append(errs, e.Err)
		}
	}
	if len(errs) != 1 || !strings.HasSuffix(errs[0], "kept as is: users[0].profile.prefs.futurePref") {
		t.Fatalf("server row errors = %q", errs)
	}
}

// One unknown key on each kind of list item, and a second item beside it with none.
const futureLists = `filterSets:
  - {name: A, sode: true, ext: true, futureF: 1}
  - {name: B, sode: true, ext: true}
presets:
  - name: P
    futureP: 1
    items:
      - {src: 1, dsts: [2], futureI: 1}
  - name: Q
    items:
      - {src: 3, dsts: [4]}
guestRights:
  futureG: 1
apiClients:
  - {name: cam, scopes: [names], rights: {futureCR: 1}, futureC: 1}
  - {name: other, scopes: [names]}
users:
  - name: a
    rights: {futureR: 1}
    profile:
      filterSets:
        - {name: A, sode: true, ext: true, futureF: 1}
        - {name: B, sode: true, ext: true}
      presets:
        - name: P
          futureP: 1
          items:
            - {src: 1, dsts: [2], futureI: 1}
        - name: Q
          items:
            - {src: 3, dsts: [4]}
  - name: b
`

var listIndex = regexp.MustCompile(`\[\d+\]`)

// unindexed drops list positions, so a reorder doesn't count as a change.
func unindexed(keys []string) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = listIndex.ReplaceAllString(k, "[]")
	}
	slices.Sort(out)
	return out
}

// browserFilterSets and browserPresets are what a page sends after an edit to the second item,
// a reorder and a pin toggle. Extra stands for an unknown key the browser tried to add.
func browserFilterSets() []config.FilterSet {
	planted := map[string]any{"browserKey": 1}
	return []config.FilterSet{
		{Name: "B", Src: "cam", Sode: true, Ext: true, Extra: planted},
		{Name: "a", Sode: true, Ext: true}, // the validator matches names in any case
		{Name: "New", Sode: true, Ext: true, Extra: planted},
	}
}

func browserPresets() []config.Preset {
	off := false
	planted := map[string]any{"browserKey": 1}
	return []config.Preset{
		{Name: "Q", Items: []config.PresetItem{{Src: 3, Dsts: []int{4, 5}, Extra: planted}}},
		{Name: "P", Pinned: &off, Items: []config.PresetItem{{Src: 1, Dsts: []int{2}}}, Extra: planted},
	}
}

func TestWholeListSavesKeepUnknownKeys(t *testing.T) {
	for name, save := range map[string]func(t *testing.T, a *App) error{
		"filter sets": func(t *testing.T, a *App) error {
			return a.SaveFilterSets(adminClient(t, a), browserFilterSets())
		},
		"presets": func(t *testing.T, a *App) error {
			return a.SavePresets(adminClient(t, a), browserPresets())
		},
		"profile lists": func(t *testing.T, a *App) error {
			ana := watched(t, a, "10.0.0.1", "1234")
			_, err := a.ProfileSave(ana, profileReq{Name: "Ana", FilterSets: browserFilterSets(), Presets: browserPresets(),
				PinOrder: []string{"user:q", "user:p"}})
			return err
		},
		"users editor": func(t *testing.T, a *App) error {
			planted := map[string]any{"browserKey": 1}
			src := []int{1, 2}
			rights := config.Rights{Sources: &src, Extra: planted}
			clients := []config.APIClient{
				{Name: "other", Scopes: []string{"names", "tally"}, Extra: planted},
				{Name: "CAM", Scopes: []string{"names"}, Rights: rights},
			}
			req := usersReq{
				Users:       []userEdit{{Name: "Bo"}, {Name: "Ana", Rights: &rights}},
				APIClients:  &clients,
				GuestRights: &rights,
			}
			return a.SaveUsers(adminClient(t, a), req)
		},
	} {
		t.Run(name, func(t *testing.T) {
			a := futureApp(t, futureLists)
			want := unindexed(a.r().cfg.UnknownKeys())
			if len(want) != 10 {
				t.Fatalf("fixture loads %d unknown keys, want 10: %q", len(want), want)
			}
			if err := save(t, a); err != nil {
				t.Fatal(err)
			}
			if got := unindexed(savedUnknownKeys(t, a)); !slices.Equal(got, want) {
				t.Fatalf("UnknownKeys after save = %q\nwant %q", got, want)
			}
		})
	}
}

package config

import (
	"reflect"
	"strings"
	"testing"
)

// A route across routers stores its tie lines. A tie line that Sode does not have makes the preset Broken with the
// tie kind. The mark goes when the tie line comes back.
func TestPresetTiesAndTieMarks(t *testing.T) {
	c, _, b := twoRouterConfig()
	across := PresetItem{Src: Ref{Router: c.UID, N: 1}, Dsts: []Ref{{Router: b, N: 2}}, Ties: []string{" t1 ", ""}}
	c.TieLines = []TieLine{{ID: "t1", Name: "TIE 1", From: Ref{Router: c.UID, N: 5}, To: Ref{Router: b, N: 6}}}
	c.Presets = []Preset{{Name: "Show", Items: []PresetItem{across}}}
	c.Users = []User{{Name: "Ann", Profile: Profile{Presets: []Preset{{Name: "Mine", Items: []PresetItem{across}}}}}}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := c.Presets[0].Items[0].Ties; !reflect.DeepEqual(got, []string{"t1"}) || c.Presets[0].Broken != nil {
		t.Fatalf("ties %q broken %+v", got, c.Presets[0].Broken)
	}
	saved := c.TieLines
	c.TieLines = nil
	c.Normalize()
	for _, p := range []Preset{c.Presets[0], c.Users[0].Profile.Presets[0]} {
		if b := p.Broken; len(b) != 1 || b[0].Kind != MissingTie || b[0].Tie != "t1" {
			t.Fatalf("%s: no tie mark: %+v", p.Name, b)
		}
	}
	c.TieLines = saved
	c.Normalize()
	if b := c.Presets[0].Broken; len(b) != 0 {
		t.Fatalf("mark stayed: %+v", b)
	}
	for _, tc := range []struct {
		it   PresetItem
		want string
	}{
		{PresetItem{Src: across.Src, Dsts: across.Dsts}, "needs its tie lines"},
		{PresetItem{Src: across.Src, Dsts: []Ref{{Router: c.UID, N: 2}}, Ties: []string{"t1"}}, "only for a route across routers"},
		{PresetItem{Src: across.Src, Dsts: []Ref{{Router: b, N: 2}, {Router: c.UID, N: 3}}, Ties: []string{"t1"}}, "one router"},
	} {
		c.Presets[0].Items = []PresetItem{tc.it}
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: %v, want %q", tc.it, err, tc.want)
		}
	}
}

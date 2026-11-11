package config

import "testing"

func ints(v ...int) *[]int { return &v }

func strs(v ...string) *[]string { return &v }

func TestRightsWithin(t *testing.T) {
	cases := []struct {
		name  string
		r, of Rights
		want  bool
	}{
		{"both open", Rights{}, Rights{}, true},
		{"open is not within a list", Rights{}, Rights{Sources: ints(1, 2)}, false},
		{"list is within open", Rights{Sources: ints(1, 2)}, Rights{}, true},
		{"empty list is within anything", Rights{Dsts: ints()}, Rights{Dsts: ints(3)}, true},
		{"subset", Rights{Sources: ints(2)}, Rights{Sources: ints(1, 2, 3)}, true},
		{"equal", Rights{Dsts: ints(1, 2)}, Rights{Dsts: ints(2, 1)}, true},
		{"superset", Rights{Sources: ints(1, 2, 3)}, Rights{Sources: ints(1, 2)}, false},
		{"overlap only", Rights{Sources: ints(1, 9)}, Rights{Sources: ints(1, 2)}, false},
		{"levels subset", Rights{Levels: ints(1)}, Rights{Levels: ints(1, 2)}, true},
		{"levels wider", Rights{Levels: ints(1, 2)}, Rights{Levels: ints(1)}, false},
		{"levels open vs list", Rights{}, Rights{Levels: ints(1)}, false},
		{"presets subset", Rights{Presets: strs("a")}, Rights{Presets: strs("a", "b")}, true},
		{"presets wider", Rights{Presets: strs("a", "c")}, Rights{Presets: strs("a", "b")}, false},
		{"one list too wide fails the rest", Rights{Sources: ints(1), Dsts: ints(1, 2)}, Rights{Sources: ints(1), Dsts: ints(1)}, false},
		{"read-only within read-only", Rights{}, Rights{Visibility: VisReadOnly}, true},
		{"hidden within read-only", Rights{Visibility: VisHidden}, Rights{}, true},
		{"hidden within hidden", Rights{Visibility: VisHidden}, Rights{Visibility: VisHidden}, true},
		{"read-only is wider than hidden", Rights{}, Rights{Visibility: VisHidden}, false},
		{"explicit read-only is wider than hidden", Rights{Visibility: VisReadOnly}, Rights{Visibility: VisHidden}, false},
	}
	for _, c := range cases {
		if got := c.r.Within(c.of); got != c.want {
			t.Errorf("%s: Within = %v, want %v", c.name, got, c.want)
		}
	}
}

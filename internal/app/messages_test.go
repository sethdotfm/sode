package app

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"
)

func keys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// The browser reads these keys by name, and in places it treats a missing key and a zero key differently.
func TestMessageKeys(t *testing.T) {
	zero, one := 0, 1
	e := &Entry{}
	cases := []struct {
		name string
		v    any
		want []string
	}{
		{"tally", deltaMsg{T: "tally"}, []string{"t", "u"}},
		{"protected", deltaMsg{T: "protected"}, []string{"t", "u"}},
		{"namesChecked", namesCheckedMsg{T: "namesChecked", At: time.Now()}, []string{"at", "t"}},
		{"names", namesMsg{T: "names"}, []string{"at", "dsts", "srcs", "t"}},
		{"filterSets", filterSetsMsg{T: "filterSets"}, []string{"sets", "t"}},
		{"historyCleared", simpleMsg{T: "historyCleared"}, []string{"t"}},
		{"reload", simpleMsg{T: "reload"}, []string{"t"}},
		{"takeResult bare", routeResult{T: "takeResult"}, []string{"id", "ok", "t"}},
		{"takeResult planned fail", routeResult{T: "takeResult", Err: "x"}, []string{"err", "id", "ok", "t"}},
		{"takeResult done", routeResult{T: "takeResult", OK: true, Entry: &zero}, []string{"entry", "id", "ok", "t"}},
		{"takeResult full", routeResult{T: "takeResult", Err: "x", Already: []int{1}, Entry: &one}, []string{"already", "entry", "err", "id", "ok", "t"}},
		{"entry", recentMsg{T: "entry", E: e}, []string{"e", "t"}},
		{"entry update", recentMsg{T: "entry", E: e, Update: true}, []string{"e", "t", "update"}},
		{"history", historyMsg{T: "history"}, []string{"before", "entries", "more", "t"}},
		{"filterSetsResult ok", opResult{T: "filterSetsResult", OK: true, ID: &zero}, []string{"id", "ok", "t"}},
		{"filterSetsResult fail", opResult{T: "filterSetsResult", Err: "x", ID: &zero}, []string{"err", "id", "ok", "t"}},
		{"configResult ok", opResult{T: "configResult", OK: true}, []string{"ok", "t"}},
		{"configResult fail", opResult{T: "configResult", Err: "x"}, []string{"err", "ok", "t"}},
		{"logAdd", sentAddMsg{T: "logAdd"}, []string{"entries", "lost", "t", "total"}},
		{"logPage", sentPageMsg{T: "logPage"}, []string{"before", "entries", "lost", "more", "t", "total"}},
	}
	for _, c := range cases {
		if got := keys(t, c.v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: keys %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMessageValues(t *testing.T) {
	one := 1
	b, _ := json.Marshal(opResult{T: "filterSetsResult", Err: "x", ID: &one})
	if string(b) != `{"t":"filterSetsResult","ok":false,"id":1,"err":"x"}` {
		t.Errorf("failed op result: %s", b)
	}
	b, _ = json.Marshal(routeResult{T: "takeResult", ID: 4, OK: true, Already: []int{2}, Entry: &one})
	if string(b) != `{"t":"takeResult","id":4,"ok":true,"already":[2],"entry":1}` {
		t.Errorf("take result: %s", b)
	}
}

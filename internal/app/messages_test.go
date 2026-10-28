package app

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"
)

// keys marshals v and returns its top-level JSON keys, sorted.
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

// TestMessageKeys pins the key set of every server -> browser message to
// what the map[string]any versions sent before they were typed (the browser
// reads these by name, and treats a missing key and a zero one differently
// in places).
func TestMessageKeys(t *testing.T) {
	zero, one := 0, 1
	yes, no := true, false
	e := &Entry{}
	cases := []struct {
		name string
		v    any
		want []string
	}{
		{"tally", deltaMsg{T: "tally"}, []string{"t", "u"}},
		{"protect", deltaMsg{T: "protect"}, []string{"t", "u"}},
		{"namesChecked", namesCheckedMsg{T: "namesChecked", At: time.Now()}, []string{"at", "t"}},
		{"names", namesMsg{T: "names"}, []string{"at", "dsts", "srcs", "t"}},
		{"filterSets", filterSetsMsg{T: "filterSets"}, []string{"sets", "t"}},
		{"locked", simpleMsg{T: "locked"}, []string{"t"}},
		{"historyCleared", simpleMsg{T: "historyCleared"}, []string{"t"}},
		// takeResult: id and ok always; err, already, entry when set.
		{"takeResult bare", takeResult{T: "takeResult"}, []string{"id", "ok", "t"}},
		{"takeResult planned fail", takeResult{T: "takeResult", Err: "x"}, []string{"err", "id", "ok", "t"}},
		{"takeResult done", takeResult{T: "takeResult", OK: true, Entry: &zero}, []string{"entry", "id", "ok", "t"}},
		{"takeResult full", takeResult{T: "takeResult", Err: "x", Already: []int{1}, Entry: &one}, []string{"already", "entry", "err", "id", "ok", "t"}},
		{"recent", recentMsg{T: "recent", E: e}, []string{"e", "t"}},
		{"recent update", recentMsg{T: "recent", E: e, Update: true}, []string{"e", "t", "update"}},
		{"history", historyMsg{T: "history"}, []string{"before", "entries", "more", "t"}},
		// unlockResult: id always; err on failure.
		{"unlockResult ok", opResult{T: "unlockResult", OK: true, ID: &zero}, []string{"id", "ok", "t"}},
		{"unlockResult fail", opResult{T: "unlockResult", Err: "x", ID: &one}, []string{"err", "id", "ok", "t"}},
		// filterSetsResult: id always; err and locked (true or false) on failure.
		{"filterSetsResult ok", opResult{T: "filterSetsResult", OK: true, ID: &zero}, []string{"id", "ok", "t"}},
		{"filterSetsResult fail", opResult{T: "filterSetsResult", Err: "x", ID: &zero, Locked: &no}, []string{"err", "id", "locked", "ok", "t"}},
		// configResult: no id; err and locked on failure.
		{"configResult ok", opResult{T: "configResult", OK: true}, []string{"ok", "t"}},
		{"configResult fail", opResult{T: "configResult", Err: "x", Locked: &yes}, []string{"err", "locked", "ok", "t"}},
		{"sentAdd", sentAddMsg{T: "sentAdd"}, []string{"entries", "lost", "t", "total"}},
		{"sentPage", sentPageMsg{T: "sentPage"}, []string{"before", "entries", "lost", "more", "t", "total"}},
	}
	for _, c := range cases {
		if got := keys(t, c.v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: keys %v, want %v", c.name, got, c.want)
		}
	}
}

// TestMessageValues checks the parts of the shapes that are not just keys.
func TestMessageValues(t *testing.T) {
	one := 1
	no := false
	b, _ := json.Marshal(opResult{T: "filterSetsResult", Err: "x", ID: &one, Locked: &no})
	if string(b) != `{"t":"filterSetsResult","ok":false,"id":1,"err":"x","locked":false}` {
		t.Errorf("failed op result: %s", b)
	}
	// A failed save that wasn't the PIN still says locked:false.
	if l := locked(nil); *l {
		t.Error("locked(nil) = true")
	}
	b, _ = json.Marshal(takeResult{T: "takeResult", ID: 4, OK: true, Already: []int{2}, Entry: &one})
	if string(b) != `{"t":"takeResult","id":4,"ok":true,"already":[2],"entry":1}` {
		t.Errorf("take result: %s", b)
	}
}

package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func grpAdd(a *App, group string, n int, e Entry) {
	for range n {
		e := e
		e.Group = group
		if group != "" {
			e.GroupN = n
		}
		e.OK = true
		a.record(&e)
	}
}

func grpIDs(t *testing.T, body map[string]any) []float64 {
	t.Helper()
	var ids []float64
	for _, e := range body["entries"].([]any) {
		ids = append(ids, e.(map[string]any)["id"].(float64))
	}
	return ids
}

func TestGroupLastNotFoundWithoutGroup(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := botKey(t, a)
	wantErr(t, apiDo(a, "GET", "/api/v1/history/groups/last", key, nil), 404, "not_found")
	grpAdd(a, "", 1, Entry{Src: 1, Dst: 2, Levels: []int{1}}) // a single-route take is not a Group
	grpAdd(a, "x1", 2, Entry{Src: 1, Dst: 2, Levels: []int{1}, Ext: true})
	wantErr(t, apiDo(a, "GET", "/api/v1/history/groups/last", key, nil), 404, "not_found")
}

func TestGroupLastReturnsNewestTakeGroupOldestFirst(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := botKey(t, a)
	grpAdd(a, "old", 2, Entry{Src: 1, Dst: 2, Levels: []int{1}})
	grpAdd(a, "new", 3, Entry{Src: 1, Dst: 2, Levels: []int{1}, Client: "PanelsBot (Panel 43)"})
	grpAdd(a, "", 1, Entry{Src: 1, Dst: 2, Levels: []int{1}})
	grpAdd(a, "x9", 2, Entry{Src: 1, Dst: 2, Levels: []int{1}, Ext: true})
	body := apiBody(t, apiDo(a, "GET", "/api/v1/history/groups/last", key, nil))
	ids := grpIDs(t, body)
	if len(ids) != 3 || ids[0] != 2 || ids[2] != 4 || body["group"] != "new" {
		t.Fatalf("got %v %v", body["group"], ids)
	}
	if e := body["entries"].([]any)[0].(map[string]any); e["client"] != "PanelsBot (Panel 43)" || e["groupN"] != float64(3) {
		t.Errorf("entry %v", e)
	}
}

func TestGroupLastHiddenShowsVisibleRoutesOnly(t *testing.T) {
	a, key := viewApp(t, config.VisHidden)
	grpAdd(a, "older", 2, Entry{Src: 1, Dst: 2, Levels: []int{1}})
	a.record(&Entry{Group: "newer", GroupN: 2, Src: 1, Dst: 2, Levels: []int{1}, OK: true})
	a.record(&Entry{Group: "newer", GroupN: 2, Src: 5, Dst: 7, Levels: []int{1}, OK: true})
	body := apiBody(t, apiDo(a, "GET", "/api/v1/history/groups/last", key, nil))
	if ids := grpIDs(t, body); len(ids) != 1 || ids[0] != 2 || body["group"] != "newer" {
		t.Fatalf("got %v %v", body["group"], ids)
	}
	if _, ok := body["entries"].([]any)[0].(map[string]any)["groupN"]; ok {
		t.Error("groupN leaks the hidden route count")
	}
	// A Group with nothing visible is skipped for an older one.
	grpAdd(a, "dark", 2, Entry{Src: 5, Dst: 7, Levels: []int{1}})
	if body := apiBody(t, apiDo(a, "GET", "/api/v1/history/groups/last", key, nil)); body["group"] != "newer" {
		t.Errorf("got %v", body["group"])
	}
}

func TestGroupLastNeedsHistoryScope(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "NamesBot"}, "n", time.Time{})
	if rec := apiDo(a, "GET", "/api/v1/history/groups/last", key, nil); rec.Code != 403 {
		t.Errorf("status %d", rec.Code)
	}
}

package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBookmarkCases(t *testing.T) {
	raw, err := os.ReadFile("../../web/test/bookmark_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			B       Bookmark `json:"b"`
			Problem string   `json:"problem"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, c := range doc.Cases {
		if got := BookmarkProblem(c.B); got != c.Problem {
			t.Errorf("%+v: got %q, want %q", c.B, got, c.Problem)
		}
	}
}

func TestBookmarksCapAndValidate(t *testing.T) {
	list := make([]Bookmark, MaxBookmarks+1)
	for i := range list {
		list[i] = Bookmark{Key: "page:log", Label: "Log"}
	}
	c := Defaults()
	c.Bookmarks = list[:MaxBookmarks]
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("24 bookmarks refused: %v", err)
	}
	c.Bookmarks = list
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "at most 24 bookmarks") {
		t.Fatalf("25th bookmark: %v", err)
	}
	c.Bookmarks = []Bookmark{{URL: "javascript:alert(1)", Label: "X"}}
	if err := c.Validate(); err == nil {
		t.Fatal("javascript: URL kept")
	}
	p := Profile{Bookmarks: []Bookmark{{URL: "https://x.test/#key=1", Label: "X"}}}
	if errs := validateProfile("Ana", p); len(errs) != 1 {
		t.Fatalf("profile bookmark with a panel key: %v", errs)
	}
}

func TestRenameBookmarks(t *testing.T) {
	list := []Bookmark{{Key: "filterSet:global:Cams", Label: "Cams"}, {Key: "filterSet:user:Cams", Label: "Cams"}}
	got := RenameBookmarks(list, "filterSet:global:Cams", "filterSet:global:Cameras", "Cameras")
	if got[0].Key != "filterSet:global:Cameras" || got[0].Label != "Cameras" || got[1].Key != "filterSet:user:Cams" {
		t.Fatalf("renamed %+v", got)
	}
	if list[0].Key != "filterSet:global:Cams" {
		t.Fatal("RenameBookmarks changed its input")
	}
	was := []Preset{{ID: "p1", Name: "Old"}, {ID: "p2", Name: "Same"}}
	now := []Preset{{ID: "p1", Name: "New"}, {ID: "p2", Name: "Same"}}
	bm := []Bookmark{{Key: "preset:global:p1", Label: "Old"}, {Key: "preset:user:p1", Label: "Old"}}
	got = RenamedPresetBookmarks(bm, "global", was, now)
	if got[0].Label != "New" || got[0].Key != "preset:global:p1" || got[1].Label != "Old" {
		t.Fatalf("preset rename %+v", got)
	}
}

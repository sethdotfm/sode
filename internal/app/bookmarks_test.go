package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestSaveBookmarksEveryone(t *testing.T) {
	a := userApp(t, false)
	ada := adminClient(t, a)
	list := []config.Bookmark{{Key: "page:log", Label: "Log", InPalette: true}, {URL: "https://example.com/", Label: "Rundown", NewTab: true}}
	if err := a.SaveBookmarks(ada, list); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg.Bookmarks; len(got) != 2 || got[1].URL != "https://example.com/" || !got[0].InPalette {
		t.Fatalf("Everyone's bookmarks %+v", got)
	}
	ana := watched(t, a, "10.0.0.1", "1234") // Standard
	if err := admitMsg(a, ana, inMsg{T: "bookmarks"}); !isRoleErr(err) {
		t.Fatalf("a Standard user admitted to Everyone's bookmarks: %v", err)
	}
	if err := a.SaveBookmarks(ana, nil); !isRoleErr(err) {
		t.Fatalf("a Standard user saved Everyone's bookmarks: %v", err)
	}
}

// The page refuses these too (bookmark_cases.json), but a stale or hand-made message must not get past the server.
func TestSaveBookmarksRefusesUnsafe(t *testing.T) {
	a := userApp(t, false)
	ada := adminClient(t, a)
	for _, b := range []config.Bookmark{
		{URL: "javascript:alert(1)", Label: "X"},
		{URL: "https://sode.local/panels/wall#key=abc", Label: "Wall"},
		{URL: "https://sode.local/panels/wall#setup=abc", Label: "Wall"},
		{URL: "https://sode.local/#share=abc", Label: "Share"},
	} {
		if err := a.SaveBookmarks(ada, []config.Bookmark{b}); err == nil {
			t.Errorf("%s kept", b.URL)
		}
		ana := watched(t, a, "10.0.0.1", "1234")
		if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Bookmarks: []config.Bookmark{b}}); err == nil {
			t.Errorf("%s kept in Me", b.URL)
		}
	}
	many := make([]config.Bookmark, config.MaxBookmarks+1)
	for i := range many {
		many[i] = config.Bookmark{Key: "page:log", Label: "Log"}
	}
	if err := a.SaveBookmarks(ada, many); err == nil || !strings.Contains(err.Error(), "at most 30") {
		t.Fatalf("31 bookmarks: %v", err)
	}
	if len(a.r().cfg.Bookmarks) != 0 {
		t.Fatalf("a refused save wrote %+v", a.r().cfg.Bookmarks)
	}
}

// A rename moves the bookmarks in the same scope in the same save. Bookmarks in another scope keep the old key, so they
// show Missing on the page.
func TestRenameMovesBookmarksInScope(t *testing.T) {
	a := userApp(t, false)
	ada := adminClient(t, a)
	ana := watched(t, a, "10.0.0.1", "1234")
	if err := a.SaveFilterSets(ada, []config.FilterSet{{Name: "Cams", Src: "cam"}}); err != nil {
		t.Fatal(err)
	}
	p := testPreset("Morning")
	p.ID = "00112233aabbccdd"
	if err := a.SavePresets(ada, []config.Preset{p}); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveBookmarks(ada, []config.Bookmark{
		{Key: "filterSet:global:Cams", Label: "Cams"},
		{Key: "preset:global:" + p.ID, Label: "Morning"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Bookmarks: []config.Bookmark{{Key: "filterSet:global:Cams", Label: "Cams"}}}); err != nil {
		t.Fatal(err)
	}

	if err := a.SaveFilterSetsRenamed(ada, []config.FilterSet{{Name: "Cameras", Src: "cam"}}, &setRename{From: "Cams", To: "Cameras"}); err != nil {
		t.Fatal(err)
	}
	p.Name = "Evening"
	if err := a.SavePresets(ada, []config.Preset{p}); err != nil {
		t.Fatal(err)
	}
	got := a.r().cfg.Bookmarks
	if got[0].Key != "filterSet:global:Cameras" || got[0].Label != "Cameras" {
		t.Fatalf("Everyone's filter set bookmark %+v", got[0])
	}
	if got[1].Key != "preset:global:"+p.ID || got[1].Label != "Evening" {
		t.Fatalf("Everyone's preset bookmark %+v", got[1])
	}
	if mine := anasProfile(t, a).Bookmarks; mine[0].Key != "filterSet:global:Cams" {
		t.Fatalf("Ana's Me bookmark followed a rename in another scope: %+v", mine)
	}
}

func TestProfileSaveBookmarks(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	p := testPreset("Desk")
	p.ID = "aabbccdd00112233"
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana",
		FilterSets: []config.FilterSet{{Name: "Mine"}},
		Presets:    []config.Preset{p},
		Bookmarks: []config.Bookmark{
			{Key: "filterSet:user:Mine", Label: "Mine"},
			{Key: "preset:user:" + p.ID, Label: "Desk", InPalette: true},
		}}); err != nil {
		t.Fatal(err)
	}
	// A save of other parts keeps the bookmarks.
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	if got := anasProfile(t, a).Bookmarks; len(got) != 2 || !got[1].InPalette {
		t.Fatalf("bookmarks lost: %+v", got)
	}
	p.Name = "Desk 2"
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana",
		FilterSets: []config.FilterSet{{Name: "Ours"}}, RenameSet: &setRename{From: "Mine", To: "Ours"},
		Presets: []config.Preset{p}}); err != nil {
		t.Fatal(err)
	}
	got := anasProfile(t, a).Bookmarks
	if got[0].Key != "filterSet:user:Ours" || got[0].Label != "Ours" || got[1].Label != "Desk 2" {
		t.Fatalf("renamed %+v", got)
	}
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Bookmarks: []config.Bookmark{}}); err != nil {
		t.Fatal(err)
	}
	if n := len(anasProfile(t, a).Bookmarks); n != 0 {
		t.Fatalf("an empty list kept %d", n)
	}
}

package app

import (
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Everyone bookmarks in the export file (docs/agents/profiles.md): one item, written whole, as the own export's
// Profile item. A bookmark's place in the list is its Alt+1-9 key, so a merge by item would move them.
const (
	everyoneBookmarksName  = "Everyone bookmarks"
	noteReplacesBookmarks  = "Replaces Everyone bookmarks"
	everyonePresetKeyStart = "preset:global:"
)

// planBookmarks: an empty file list adds nothing; Replace import then empties the live list (emptyRest). The step
// validates the list with the bookmark rules, so an unsafe web address refuses the category.
func (p *importPlanner) planBookmarks(cand *config.Config, cat *importCat) error {
	if len(p.b.Bookmarks) == 0 || !p.picked(catBookmarks, catBookmarks) {
		return nil
	}
	list := p.bookmarksHere(p.b.Bookmarks)
	it := importItem{Key: catBookmarks, Name: everyoneBookmarksName}
	if len(cand.Bookmarks) == 0 {
		cand.Bookmarks = list
	} else {
		it.Note = noteReplacesBookmarks
		if p.settle(&it, catBookmarks, everyoneBookmarksName, sameBookmarks(list, cand.Bookmarks), nil) == overwrite {
			cand.Bookmarks = list
		}
	}
	cat.Items = append(cat.Items, it)
	return nil
}

// bookmarksHere gives a bookmark on an Everyone preset the id that preset has here, when this import moved it. Other
// keys are opaque and stay as the file has them; a target that is not here shows Missing.
func (p *importPlanner) bookmarksHere(in []config.Bookmark) []config.Bookmark {
	out := config.CloneBookmarks(in)
	for i, b := range out {
		if id, ok := strings.CutPrefix(b.Key, everyonePresetKeyStart); ok {
			if to := p.presetTo(id); to != "" {
				out[i].Key = everyonePresetKeyStart + to
			}
		}
	}
	return out
}

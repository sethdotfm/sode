package app

import (
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Bookmarks (GLOSSARY.md): Everyone's list is cfg.Bookmarks, saved here by a Trusted user or an Administrator.
// A user's Me list rides profileSave. config.Validate refuses an unsafe one (BookmarkProblem).

type bookmarksMsg struct {
	T         string            `json:"t"` // "bookmarks"
	Bookmarks []config.Bookmark `json:"bookmarks"`
}

// setRename: a filter set save that renames one set. Filter sets have no id, so their bookmark keys hold the name.
// The browser names the rename, and the bookmarks in the same scope follow it in the same save.
type setRename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// apply: scope is the key's scope word, "global" (Everyone) or "user" (Me). A nil rename changes nothing.
func (r *setRename) apply(list []config.Bookmark, scope string) []config.Bookmark {
	if r == nil {
		return list
	}
	from, to := strings.TrimSpace(r.From), strings.TrimSpace(r.To)
	if from == "" || to == "" || from == to {
		return list
	}
	return config.RenameBookmarks(list, "filterSet:"+scope+":"+from, "filterSet:"+scope+":"+to, to)
}

func (a *App) SaveBookmarks(c *client, list []config.Bookmark) error {
	if list == nil {
		list = []config.Bookmark{}
	}
	return a.saveList(c, func(cur *config.Config) error {
		cur.Bookmarks = config.CloneBookmarks(list)
		return nil
	}, func(cfg config.Config) any { return bookmarksMsg{T: "bookmarks", Bookmarks: cfg.Bookmarks} })
}

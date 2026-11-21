package app

import (
	"github.com/sethdotfm/sode/internal/config"
)

// Bookmarks (GLOSSARY.md): Everyone's list is cfg.Bookmarks, saved here by a Trusted user or an Administrator.
// A user's Me list rides profileSave. config.Validate refuses an unsafe one (BookmarkProblem).

type bookmarksMsg struct {
	T         string            `json:"t"` // "bookmarks"
	Bookmarks []config.Bookmark `json:"bookmarks"`
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

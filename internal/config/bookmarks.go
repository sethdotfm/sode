package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"
)

// A Bookmark (GLOSSARY.md) opens a Palette row or an outside web address. Everyone's list is Config.Bookmarks, a
// user's Me list is Profile.Bookmarks, and This browser's is in localStorage. Go twin of web/bookmarkrules.js
// (TestBookmarkCases runs the shared cases in web/test/bookmark_cases.json).
type Bookmark struct {
	// Key names a Palette row (palettecatalogue.js), such as "preset:global:<id>". Sode does not read the rest of it.
	Key string `yaml:"key,omitempty" json:"key,omitempty"`
	// URL is an http: or https: address. A bookmark has a Key or a URL, never both.
	URL string `yaml:"url,omitempty" json:"url,omitempty"`
	// Label is the name the strip shows. A row's live label wins; this one shows while the target is Missing.
	Label  string `yaml:"label" json:"label"`
	NewTab bool   `yaml:"newTab,omitempty" json:"newTab"`
	// InPalette: Show in Palette. The Palette's empty query lists these first.
	InPalette bool           `yaml:"inPalette" json:"inPalette"`
	Extra     map[string]any `yaml:",inline" json:"-"`
}

const (
	MaxBookmarks       = 30
	MaxBookmarkLabel   = 40
	MaxBookmarkURLSize = 2000
)

// The kinds of Palette row that a Key can name (palettecatalogue.js).
var bookmarkKinds = []string{"action", "preset", "page", "panel", "filterSet", "setting"}

func CloneBookmarks(in []Bookmark) []Bookmark { return slices.Clone(in) }

func normalizeBookmarks(in []Bookmark) []Bookmark {
	out := make([]Bookmark, len(in))
	for i, b := range in {
		b.Key, b.URL, b.Label = strings.TrimSpace(b.Key), strings.TrimSpace(b.URL), strings.TrimSpace(b.Label)
		out[i] = b
	}
	return out
}

// BookmarkProblem says why Sode refuses b, or "". The page shows the same words (bookmarkrules.js bookmarkProblem).
func BookmarkProblem(b Bookmark) string {
	if b.Label == "" {
		return "a bookmark needs a name"
	}
	if len([]rune(b.Label)) > MaxBookmarkLabel {
		return fmt.Sprintf("a bookmark name has at most %d characters", MaxBookmarkLabel)
	}
	if strings.IndexFunc(b.Label, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return "a bookmark name has only characters you can see"
	}
	switch {
	case b.Key != "" && b.URL != "":
		return "a bookmark opens a Palette row or a web address, not both"
	case b.Key != "":
		kind, _, ok := strings.Cut(b.Key, ":")
		if !ok || !slices.Contains(bookmarkKinds, kind) || len([]rune(b.Key)) > MaxPaletteKey {
			return "a bookmark names a Palette row that Sode does not know"
		}
		return ""
	case b.URL != "":
		return BookmarkURLProblem(b.URL)
	}
	return "a bookmark needs a Palette row or a web address"
}

// BookmarkURLProblem: only http: and https:, and never a secret. Each reader of a bookmark gets what it holds. So it
// never holds a panel key (#key=), a setup link (#setup=), a #share= link, or a user name and password.
func BookmarkURLProblem(raw string) string {
	if len(raw) > MaxBookmarkURLSize {
		return fmt.Sprintf("a web address has at most %d characters", MaxBookmarkURLSize)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "use a web address that starts with http:// or https://"
	}
	if u.User != nil {
		return "a web address in a bookmark cannot hold a user name or password"
	}
	for _, part := range strings.Split(u.Fragment, "&") {
		name, _, _ := strings.Cut(part, "=")
		switch strings.ToLower(name) {
		case "key":
			return "a bookmark cannot hold a panel key"
		case "setup":
			return "a bookmark cannot hold a setup link"
		case "share":
			return "a bookmark cannot hold a #share= link"
		}
	}
	for name := range u.Query() {
		switch strings.ToLower(name) {
		case "key", "token":
			return "a bookmark cannot hold a panel key"
		case "setup":
			return "a bookmark cannot hold a setup link"
		}
	}
	return ""
}

func validateBookmarks(list []Bookmark) []error {
	var errs []error
	if len(list) > MaxBookmarks {
		errs = append(errs, fmt.Errorf("at most %d bookmarks in a list", MaxBookmarks))
	}
	for i, b := range list {
		if why := BookmarkProblem(b); why != "" {
			errs = append(errs, fmt.Errorf("bookmark %d (%q): %s", i+1, b.Label, why))
		}
	}
	return errs
}

// RenameBookmarks points the bookmarks on from at to, with the new label, in the same save as the rename. A list in
// another scope keys its own row, so it is not changed here: that bookmark shows Missing.
func RenameBookmarks(list []Bookmark, from, to, label string) []Bookmark {
	if from == "" || to == "" {
		return list
	}
	out := slices.Clone(list)
	for i := range out {
		if out[i].Key == from {
			out[i].Key = to
			if label != "" {
				out[i].Label = label
			}
		}
	}
	return out
}

// RenamedPresetBookmarks: a preset keeps its id through a rename, so its key stays and only the label follows.
// scope is the key's scope word: "global" for Everyone, "user" for Me.
func RenamedPresetBookmarks(list []Bookmark, scope string, was, now []Preset) []Bookmark {
	old := map[string]string{}
	for _, p := range was {
		old[p.ID] = p.Name
	}
	for _, p := range now {
		if n, ok := old[p.ID]; ok && n != p.Name {
			k := "preset:" + scope + ":" + p.ID
			list = RenameBookmarks(list, k, k, p.Name)
		}
	}
	return list
}

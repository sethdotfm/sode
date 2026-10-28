package app

import (
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Server -> browser messages (hello aside, which stays a map). The `t` field names the kind; web/core.js switches on it. Keys
// and omitempty rules are what the browser reads: change them with web/.

// deltaMsg is a batch of tally or protect changes: [dst, ui level, src or
// device] triples. T is "tally" or "protect".
type deltaMsg struct {
	T string   `json:"t"`
	U [][3]int `json:"u"`
}

type namesCheckedMsg struct {
	T  string    `json:"t"` // "namesChecked"
	At time.Time `json:"at"`
}

type namesMsg struct {
	T    string    `json:"t"` // "names"
	Srcs [][2]any  `json:"srcs"`
	Dsts [][2]any  `json:"dsts"`
	At   time.Time `json:"at"`
}

type filterSetsMsg struct {
	T    string             `json:"t"` // "filterSets"
	Sets []config.FilterSet `json:"sets"`
}

// simpleMsg has no fields but its kind: "locked", "historyCleared".
type simpleMsg struct {
	T string `json:"t"`
}

// takeResult answers one take. Entry is the history entry the browser's
// banner follows (a pointer: entry 0 is real); a take that never got as far
// as being recorded has none.
type takeResult struct {
	T       string `json:"t"` // "takeResult"
	ID      int    `json:"id"`
	OK      bool   `json:"ok"`
	Err     string `json:"err,omitempty"`
	Already []int  `json:"already,omitempty"`
	Entry   *int   `json:"entry,omitempty"`
}

// recentMsg is a new history entry, or with Update an entry that changed.
type recentMsg struct {
	T      string `json:"t"` // "recent"
	E      *Entry `json:"e"`
	Update bool   `json:"update,omitempty"`
}

type historyMsg struct {
	T       string  `json:"t"` // "history"
	Before  int     `json:"before"`
	Entries []Entry `json:"entries"`
	More    bool    `json:"more"`
}

// opResult answers unlock ("unlockResult"), filterSets ("filterSetsResult")
// and config ("configResult"). ID is echoed by the first two only, Locked
// is sent by the last two, on failure only.
type opResult struct {
	T      string `json:"t"`
	OK     bool   `json:"ok"`
	ID     *int   `json:"id,omitempty"`
	Err    string `json:"err,omitempty"`
	Locked *bool  `json:"locked,omitempty"`
}

type sentAddMsg struct {
	T       string `json:"t"` // "sentAdd"
	Entries []Sent `json:"entries"`
	Total   int64  `json:"total"`
	Lost    int64  `json:"lost"`
}

type sentPageMsg struct {
	T       string `json:"t"` // "sentPage"
	Before  int64  `json:"before"`
	Entries []Sent `json:"entries"`
	More    bool   `json:"more"`
	Total   int64  `json:"total"`
	Lost    int64  `json:"lost"`
}

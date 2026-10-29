package app

import (
	"encoding/json"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Server -> browser messages. The `t` field names the kind; web/core.js switches on it. Keys
// and omitempty rules are what the browser reads: change them with web/.

// routeChange is one tally or protect entry: destination, UI level, and the
// source routed there (or the protecting device, -1 = none). On the wire and
// in the browser it is the array [dst, level, src].
type routeChange struct {
	Dst, Level, Src int
}

func (c routeChange) MarshalJSON() ([]byte, error) {
	return json.Marshal([3]int{c.Dst, c.Level, c.Src})
}

func (c *routeChange) UnmarshalJSON(b []byte) error {
	var a [3]int
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*c = routeChange{a[0], a[1], a[2]}
	return nil
}

// deltaMsg is a batch of tally or protect changes. T is "tally" or "protect".
type deltaMsg struct {
	T string        `json:"t"`
	U []routeChange `json:"u"`
}

// helloMsg is the full snapshot a browser gets on connect or config change.
// Every key is always sent (none is omitempty).
type helloMsg struct {
	T             string              `json:"t"` // "hello"
	Srcs          [][2]any            `json:"srcs"`
	Dsts          [][2]any            `json:"dsts"`
	At            time.Time           `json:"at"`
	Tally         []routeChange       `json:"tally"`
	Protect       []routeChange       `json:"protect"`
	Status        statusMsg           `json:"status"`
	Recent        []Entry             `json:"recent"`
	More          bool                `json:"more"`
	Levels        []config.Level      `json:"levels"`
	LevelDefaults []string            `json:"levelDefaults"`
	Cfg           config.Config       `json:"cfg"`
	PinSet        bool                `json:"pinSet"`
	ConfigPath    string              `json:"configPath"`
	Version       string              `json:"version"`
	BrandColors   []config.BrandColor `json:"brandColors"`
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

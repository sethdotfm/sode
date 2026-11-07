package app

import (
	"encoding/json"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Server -> browser messages. t names the kind (web/core.js switches on it); keys and
// omitempty are what the browser reads: change with web/.

// routeChange is [dst, level, src] on the wire; src is the protecting device for protect
// (-1 = none).
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

type deltaMsg struct {
	T string        `json:"t"`
	U []routeChange `json:"u"`
}

// helloMsg: every key always sent (none omitempty).
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

type presetsMsg struct {
	T       string          `json:"t"` // "presets"
	Presets []config.Preset `json:"presets"`
}

type simpleMsg struct {
	T string `json:"t"`
}

// takeResult: Entry is a pointer (entry 0 is real); none for a take never recorded.
type takeResult struct {
	T       string `json:"t"` // "takeResult"
	ID      int    `json:"id"`
	OK      bool   `json:"ok"`
	Err     string `json:"err,omitempty"`
	Already []int  `json:"already,omitempty"`
	Entry   *int   `json:"entry,omitempty"`
}

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

// opResult: ID echoed by unlockResult and filterSetsResult only; Locked sent by
// filterSetsResult and configResult, on failure only.
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

type uptimeMsg struct {
	T      string        `json:"t"` // "uptimeResult"
	ID     int           `json:"id"`
	From   int64         `json:"from"`
	To     int64         `json:"to"`
	Slot   int64         `json:"slot"`
	Now    int64         `json:"now"`
	Start  string        `json:"start"` // the state at from; "" = nothing recorded
	Trans  []UptimeTrans `json:"trans"`
	Counts []UptimeSlot  `json:"counts"`
}

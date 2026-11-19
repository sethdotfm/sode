package app

import (
	"encoding/json"
	"strconv"
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

// A hello carries one per destination and level (12750 on a 2500 by 750 router), so no reflection.
func (c routeChange) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, 20)
	b = append(b, '[')
	b = strconv.AppendInt(b, int64(c.Dst), 10)
	b = append(b, ',')
	b = strconv.AppendInt(b, int64(c.Level), 10)
	b = append(b, ',')
	b = strconv.AppendInt(b, int64(c.Src), 10)
	return append(b, ']'), nil
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
	T             string          `json:"t"` // "hello"
	Srcs          [][2]any        `json:"srcs"`
	Dsts          [][2]any        `json:"dsts"`
	At            time.Time       `json:"at"`
	SavedAt       time.Time       `json:"savedAt,omitzero"` // the names are the saved names table from then; zero after the first read
	Tally         json.RawMessage `json:"tally"`            // routeTriples
	Protect       json.RawMessage `json:"protected"`        // routeTriples
	Status        statusMsg       `json:"status"`
	Recent        []Entry         `json:"history"`
	More          bool            `json:"more"`
	Levels        []config.Level  `json:"levels"`
	LevelDefaults []string        `json:"levelDefaults"`
	Cfg           config.Config   `json:"cfg"`
	// Versions: each one Save field's version (onesave.go); a save sends back the one it loaded.
	Versions map[string]string `json:"versions"`
	// Users: the sign-in list. It reaches locked pages, so it holds each name and PIN length (0 for
	// none) and never more; cfg.users stays empty.
	Users       []helloUser         `json:"users"`
	ConfigPath  string              `json:"configPath"`
	Version     string              `json:"version"`
	BrandColors []config.BrandColor `json:"brandColors"`
	// RouterUID: browsers write it into This browser presets and filter sets. Never shown.
	RouterUID string `json:"routerUid"`
	// InstallUID: pages compare it with an Item link's install. Never shown.
	InstallUID string `json:"installUid"`
	// Prefix is the router prefix, so typing accepts A.23. Not in cfg, which hides it.
	Prefix string `json:"prefix"`
	// NoAdministrator: users and no Administrator (a hand-edited file). The page shows the
	// recovery banner. Only a restart can change it.
	NoAdministrator bool `json:"noAdministrator,omitempty"`
}

type helloUser struct {
	Name string `json:"name"`
	Len  int    `json:"len"`
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

// routeResult: Entry is a pointer (entry 0 is real); none for a take never recorded.
type routeResult struct {
	T       string `json:"t"` // "takeResult"
	ID      int    `json:"id"`
	OK      bool   `json:"ok"`
	Err     string `json:"err,omitempty"`
	Already []int  `json:"already,omitempty"`
	Entry   *int   `json:"entry,omitempty"`
}

type recentMsg struct {
	T      string `json:"t"` // "entry"
	E      *Entry `json:"e"`
	Update bool   `json:"update,omitempty"`
}

type historyMsg struct {
	T       string  `json:"t"` // "history"
	Before  int     `json:"before"`
	Entries []Entry `json:"entries"`
	More    bool    `json:"more"`
}

// opResult is shared by many result messages. ID and Kept are omitted unless the sender
// sets them.
type opResult struct {
	T   string `json:"t"`
	OK  bool   `json:"ok"`
	ID  *int   `json:"id,omitempty"`
	Err string `json:"err,omitempty"`
	// Recheck: refused for want of a PIN recheck; the page asks for it and sends again.
	Recheck bool `json:"recheck,omitempty"`
}

type sentAddMsg struct {
	T       string `json:"t"` // "logAdd"
	Entries []Sent `json:"entries"`
	Total   int64  `json:"total"`
	Lost    int64  `json:"lost"`
}

type sentPageMsg struct {
	T       string `json:"t"` // "logPage"
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

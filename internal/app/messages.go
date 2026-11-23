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
	// RouterUID: the router of the change, so the page keeps the tally of each router apart.
	RouterUID string `json:"routerUid,omitempty"`
	// from: the router unit of a tally change. The API stream sends it only to the streams of that router. nil is
	// every stream.
	from *routerUnit
}

// helloMsg: every key always sent (none omitempty).
type helloMsg struct {
	T       string          `json:"t"` // "hello"
	Srcs    [][2]any        `json:"srcs"`
	Dsts    [][2]any        `json:"dsts"`
	At      time.Time       `json:"at"`
	SavedAt time.Time       `json:"savedAt,omitzero"` // the names are the saved names table from then; zero after the first read
	Tally   json.RawMessage `json:"tally"`            // routeTriples
	Protect json.RawMessage `json:"protected"`        // protectRows: [dst, level, device, kind]
	// ProtectBy: the Protects in Sode's own record, with who set each (protect.go).
	ProtectBy     []protectBy    `json:"protectBy"`
	Status        statusMsg      `json:"status"`
	Recent        []Entry        `json:"history"`
	More          bool           `json:"more"`
	Levels        []config.Level `json:"levels"`
	LevelDefaults []string       `json:"levelDefaults"`
	Cfg           config.Config  `json:"cfg"`
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
	// NotSaved: env-only mode (no SODE_CONFIG). The page shows the Not saved banner.
	NotSaved bool `json:"notSaved,omitempty"`
	// Setup opens the setup wizard (setup.go): only while setup is pending.
	Setup *setupHello `json:"setup,omitempty"`
	// Routers: every router, the first one first. The flat fields above repeat the first router until the page reads
	// only this list.
	Routers []helloRouter `json:"routers"`
}

// helloRouter is one router of the hello. UID is the router unit's UID, which the tally, names and status messages
// and a take's route name.
type helloRouter struct {
	UID    string `json:"uid"`
	Prefix string `json:"prefix"`
	Title  string `json:"title"`
	// Address and Matrix fill the router's row in Settings > Routers.
	Address      string         `json:"address"`
	Matrix       int            `json:"matrix"`
	NumberOffset int            `json:"numberOffset"`
	Levels       []config.Level `json:"levels"`
	// LevelNumbering and DefaultLevels are this router's: typed levels of a route are levels of its router.
	LevelNumbering string          `json:"levelNumbering"`
	DefaultLevels  string          `json:"defaultLevels"`
	Srcs           [][2]any        `json:"srcs"`
	Dsts           [][2]any        `json:"dsts"`
	At             time.Time       `json:"at"`
	Tally          json.RawMessage `json:"tally"` // routeTriples
	Status         statusMsg       `json:"status"`
}

type helloUser struct {
	Name string `json:"name"`
	Len  int    `json:"len"`
}

type namesCheckedMsg struct {
	T         string    `json:"t"` // "namesChecked"
	At        time.Time `json:"at"`
	RouterUID string    `json:"routerUid,omitempty"` // as on deltaMsg
}

type namesMsg struct {
	T         string      `json:"t"` // "names"
	Srcs      [][2]any    `json:"srcs"`
	Dsts      [][2]any    `json:"dsts"`
	At        time.Time   `json:"at"`
	RouterUID string      `json:"routerUid,omitempty"` // as on deltaMsg
	from      *routerUnit // as on deltaMsg
}

type filterSetsMsg struct {
	T    string             `json:"t"` // "filterSets"
	Sets []config.FilterSet `json:"sets"`
	// Bookmarks: Everyone's list after the save, since a rename moves the bookmarks on the set.
	Bookmarks []config.Bookmark `json:"bookmarks"`
}

type presetsMsg struct {
	T       string          `json:"t"` // "presets"
	Presets []config.Preset `json:"presets"`
	// Bookmarks: Everyone's list after the save, since a rename changes the bookmarks' names.
	Bookmarks []config.Bookmark `json:"bookmarks"`
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

type opResult struct {
	T   string `json:"t"`
	OK  bool   `json:"ok"`
	ID  *int   `json:"id,omitempty"`
	Err string `json:"err,omitempty"`
	// Recheck: refused for want of a PIN recheck; the page asks for it and sends again.
	Recheck bool `json:"recheck,omitempty"`
	// Approval: the Approval itself was refused; the page shows why in its Approval popup.
	Approval bool `json:"approval,omitempty"`
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

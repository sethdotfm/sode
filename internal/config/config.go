// Package config loads, validates and saves Sode's YAML settings.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is everything editable from the settings dialog. Process-level
// settings (listen address, file paths, debug) stay in env vars.
type Config struct {
	Router string `yaml:"router" json:"router" guard:"router"`
	// BrandText replaces "Sode" in the header and page titles ("" = Sode);
	// BrandBorder is a colour name from BrandColors for a border round the
	// window ("" = none). Display only: never touches the router link.
	BrandText   string `yaml:"brandText,omitempty" json:"brandText" guard:"open"`
	BrandBorder string `yaml:"brandBorder,omitempty" json:"brandBorder" guard:"open"`
	// RouterName is a friendly name for the status card; RouterShow picks
	// what the card shows: "address" (default), "name" or "none".
	RouterName      string `yaml:"routerName" json:"routerName" guard:"open"`
	RouterShow      string `yaml:"routerShow" json:"routerShow" guard:"open"`
	Matrix          int    `yaml:"matrix" json:"matrix" guard:"router"`
	Levels          int    `yaml:"levels" json:"levels" guard:"router"`
	LevelWireOffset int    `yaml:"levelWireOffset" json:"levelWireOffset" guard:"router"`
	DefaultLevels   string `yaml:"defaultLevels" json:"defaultLevels" guard:"open"`
	NameLength      int    `yaml:"nameLength" json:"nameLength" guard:"router"`
	NameRequests    string `yaml:"nameRequests" json:"nameRequests" guard:"router"` // std | ext
	// FireMode is how a confirm of several takes is sent: "sequential" (one
	// extended connect after another) or "native" (a SW-P-08 group salvo,
	// set with one Go, so they land much closer together; app/fire.go). SalvoGroup is the group
	// (0-127) native salvos stage in: the router's list is shared by every
	// client, so it must be one only Sode uses.
	FireMode           string   `yaml:"fireMode" json:"fireMode" guard:"pin"`
	SalvoGroup         int      `yaml:"salvoGroup" json:"salvoGroup" guard:"pin"`
	NumberOffset       int      `yaml:"numberOffset" json:"numberOffset" guard:"open"`
	ConfirmLockoutMs   int      `yaml:"confirmLockoutMs" json:"confirmLockoutMs" guard:"open"`
	ConfirmEnter       bool     `yaml:"confirmEnter" json:"confirmEnter" guard:"open"`
	NameRefreshSeconds int      `yaml:"nameRefreshSeconds" json:"nameRefreshSeconds" guard:"open"`
	TallyResyncSeconds int      `yaml:"tallyResyncSeconds" json:"tallyResyncSeconds" guard:"open"`
	TrustedProxies     []string `yaml:"trustedProxies" json:"trustedProxies" guard:"pin"`
	// HiddenDsts is a Recent filter expression (web/filter.js) for
	// destinations whose changes from other panels every browser hides
	// unless its Filters show them; still recorded. E.g. a shader's monitor.
	HiddenDsts string `yaml:"hiddenDsts,omitempty" json:"hiddenDsts" guard:"open"`
	// FilterSets are named Recent filters offered to every browser as
	// quick buttons; browsers can add their own.
	FilterSets []FilterSet `yaml:"filterSets,omitempty" json:"filterSets" owned:"true"`
	// Presets are named sets of takes offered to every browser as buttons
	// above Recent (presets.go).
	Presets []Preset `yaml:"presets,omitempty" json:"presets" owned:"true"`
	// LegacyPresets reads the key presets had before Beta 1.5 was released
	// ("salvos"); Normalize moves them to Presets, so the next save writes
	// the new key.
	LegacySalvos []Preset `yaml:"salvos,omitempty" json:"-" guard:"open"`
	// Grid was parked (#150); ignored. Kept so a file that has them loads,
	// and dropped on the next save.
	LegacyGrids       any `yaml:"grids,omitempty" json:"-" guard:"open"`
	LegacyGridConfirm any `yaml:"gridConfirm,omitempty" json:"-" guard:"open"`
	// HideDsts is no longer used; kept so files that have it load.
	HideDsts []int `yaml:"hideDsts,omitempty" json:"-" guard:"open"`
	// LevelColors[i] colors UI level i+1 as #rrggbb; "" or missing keeps
	// the default (DefaultLevelColors, then the patchbay swatch). Entries
	// past Levels are kept but unused.
	LevelColors []string `yaml:"levelColors" json:"levelColors" guard:"open"`
	// LevelNumbering is how levels are shown and typed: "1" (1-17), "0"
	// (0-16) or "va" (V, 1-16: "0" with level 0 shown as V).
	LevelNumbering string `yaml:"levelNumbering" json:"levelNumbering" guard:"open"`
	// LevelTypedBase is no longer used; kept so files that have it load.
	LevelTypedBase int `yaml:"levelTypedBase,omitempty" json:"-" guard:"open"`
	// SettingsPin is a HashPin hash guarding the settings view; "" = none.
	// Never sent to browsers.
	SettingsPin string `yaml:"settingsPin,omitempty" json:"-" owned:"true"`
	// PinMaxTries wrong PINs in a row lock settings until the PIN is reset
	// (sode -reset-pin) or Sode restarts; 0 = never.
	PinMaxTries int `yaml:"pinMaxTries" json:"pinMaxTries" guard:"pin"`
	// DeviceDefaults are what the "This device" settings are for browsers
	// that haven't chosen their own. Browser-side only: changing them never
	// touches the router link.
	DeviceDefaults DeviceDefaults `yaml:"deviceDefaults" json:"deviceDefaults" owned:"true"`
	// Users, Locks and AutoLockMinutes: optional access control (users.go).
	Users           []User `yaml:"users,omitempty" json:"users" owned:"true"`
	Locks           Locks  `yaml:"locks" json:"locks" owned:"true"`
	AutoLockMinutes int    `yaml:"autoLockMinutes" json:"autoLockMinutes" owned:"true"`
	// MaxPinLen is the longest user PIN (4-12); the lock pad stops there.
	MaxPinLen int `yaml:"maxPinLen" json:"maxPinLen" owned:"true"`
}

// MaxFilterSetsBytes caps all filter sets as JSON, so a "filterSets"
// message fits the 64 KiB read limit with room to spare.
const MaxFilterSetsBytes = 48 << 10

// BrandColor is one accepted BrandBorder name. Hex is the dark-mode value of
// the signal token web/base.css maps the name to (`[data-brand="name"]`); the
// server draws the app icon with it. Label is how browsers list it.
type BrandColor struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Hex   string `json:"-"`
}

// BrandColors is the one table of colours. Never red: that is the lost-link
// outline.
var BrandColors = []BrandColor{
	{"blue", "Blue", "#5fb2e8"},
	{"teal", "Teal", "#5ccfc9"},
	{"green", "Green", "#5fd09a"},
	{"amber", "Amber", "#e8a35e"},
	{"violet", "Violet", "#a67ef0"},
	{"pink", "Pink", "#e87bb3"},
}

// BrandColorByName looks a name up in BrandColors.
func BrandColorByName(name string) (BrandColor, bool) {
	i := slices.IndexFunc(BrandColors, func(c BrandColor) bool { return c.Name == name })
	if i < 0 {
		return BrandColor{}, false
	}
	return BrandColors[i], true
}

// BrandNames lists the names, for messages.
func BrandNames() []string {
	out := make([]string, len(BrandColors))
	for i, c := range BrandColors {
		out[i] = c.Name
	}
	return out
}

// DeviceDefaults mirrors the per-browser settings (web/prefs.js): Theme is
// system | light | dark, Layout auto | phone | desktop, Clock 24 | 12 |
// locale; Flip is Destination before Source, ScrollTop scrolls to the top
// when a box is focused, SearchFilters makes quick search respect filters,
// SetStack is how stacked filter sets combine: and (all must match) | or
// (any may), NextMode is what the Next chips and Ctrl+Shift+S/D mean by next:
// number (one higher) | fuzzy (the next match of the name's text).
type DeviceDefaults struct {
	Theme         string `yaml:"theme" json:"theme"`
	Layout        string `yaml:"layout" json:"layout"`
	Clock         string `yaml:"clock" json:"clock"`
	Flip          bool   `yaml:"flip" json:"flip"`
	ScrollTop     bool   `yaml:"scrollTop" json:"scrollTop"`
	SearchFilters bool   `yaml:"searchFilters" json:"searchFilters"`
	SetStack      string `yaml:"setStack" json:"setStack"`
	NextMode      string `yaml:"nextMode" json:"nextMode"`
}

// FilterSet is one named Recent filter (see web/filter.js): which kinds of
// entry show, and expressions for sources and destinations.
type FilterSet struct {
	Name   string `yaml:"name" json:"name"`
	Src    string `yaml:"src,omitempty" json:"src"`
	Dst    string `yaml:"dst,omitempty" json:"dst"`
	Sode   bool   `yaml:"sode" json:"sode"`
	Ext    bool   `yaml:"ext" json:"ext"`
	Hidden bool   `yaml:"hidden,omitempty" json:"hidden"`
}

func Defaults() Config {
	return Config{
		RouterShow:         "address",
		Matrix:             0,
		Levels:             17,
		LevelWireOffset:    -1,
		NameLength:         12,
		NameRequests:       "std",
		FireMode:           "sequential",
		SalvoGroup:         127,
		NumberOffset:       1,
		ConfirmLockoutMs:   500,
		NameRefreshSeconds: 60,
		TallyResyncSeconds: 300,
		TrustedProxies:     []string{},
		LevelColors:        []string{},
		LevelNumbering:     "1",
		PinMaxTries:        10,
		MaxPinLen:          DefaultMaxPinLen,
		DeviceDefaults:     DeviceDefaults{Theme: "system", Layout: "auto", Clock: "24", ScrollTop: true, SetStack: "and", NextMode: "number"},
	}
}

// FromEnv overlays the SODE_* seed variables onto c. They are only used to
// create the config file on first start.
func FromEnv(c Config) (Config, error) {
	var errs []error
	str := func(k string, dst *string) {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			*dst = v
		}
	}
	num := func(k string, dst *int) {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", k, err))
				return
			}
			*dst = n
		}
	}
	str("SODE_ROUTER", &c.Router)
	num("SODE_MATRIX", &c.Matrix)
	num("SODE_LEVEL_COUNT", &c.Levels)
	num("SODE_LEVEL_WIRE_OFFSET", &c.LevelWireOffset)
	str("SODE_DEFAULT_LEVELS", &c.DefaultLevels)
	num("SODE_NAME_LEN", &c.NameLength)
	str("SODE_NAMES", &c.NameRequests)
	str("SODE_FIRE_MODE", &c.FireMode)
	num("SODE_SALVO_GROUP", &c.SalvoGroup)
	num("SODE_NUM_OFFSET", &c.NumberOffset)
	num("SODE_CONFIRM_LOCKOUT_MS", &c.ConfirmLockoutMs)
	num("SODE_NAME_REFRESH_S", &c.NameRefreshSeconds)
	num("SODE_TALLY_RESYNC_S", &c.TallyResyncSeconds)
	str("SODE_LEVEL_NUMBERING", &c.LevelNumbering)
	num("SODE_PIN_MAX_TRIES", &c.PinMaxTries)
	num("SODE_MAX_PIN_LEN", &c.MaxPinLen)
	if v := os.Getenv("SODE_CONFIRM_ENTER"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("SODE_CONFIRM_ENTER: %w", err))
		}
		c.ConfirmEnter = b
	}
	if v := os.Getenv("SODE_TRUSTED_PROXIES"); v != "" {
		c.TrustedProxies = nil
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				c.TrustedProxies = append(c.TrustedProxies, p)
			}
		}
	}
	return c, errors.Join(errs...)
}

// Clone returns a copy that shares no slices with c, so Normalize and other
// in-place edits on it can't touch a config others are reading.
func (c Config) Clone() Config {
	c.TrustedProxies = slices.Clone(c.TrustedProxies)
	c.LevelColors = slices.Clone(c.LevelColors)
	c.FilterSets = slices.Clone(c.FilterSets)
	c.Presets = ClonePresets(c.Presets)
	c.HideDsts = slices.Clone(c.HideDsts)
	c.Users = slices.Clone(c.Users)
	return c
}

// Normalize fills derived defaults; call before Validate.
func (c *Config) Normalize() {
	c.Router = strings.TrimSpace(c.Router)
	c.RouterName = strings.TrimSpace(c.RouterName)
	c.BrandText = strings.TrimSpace(c.BrandText)
	c.normalizeUsers()
	c.BrandBorder = strings.ToLower(strings.TrimSpace(c.BrandBorder))
	c.RouterShow = strings.ToLower(strings.TrimSpace(c.RouterShow))
	if c.RouterShow == "" {
		c.RouterShow = "address"
	}
	c.DefaultLevels = strings.TrimSpace(c.DefaultLevels)
	c.FireMode = strings.ToLower(strings.TrimSpace(c.FireMode))
	if c.FireMode == "" {
		c.FireMode = "sequential"
	}
	c.LevelNumbering = strings.ToLower(strings.TrimSpace(c.LevelNumbering))
	if c.LevelNumbering == "" {
		c.LevelNumbering = "1"
	}
	if c.DefaultLevels == "" && c.Levels > 0 {
		c.DefaultLevels = fmt.Sprintf("1-%d", c.Levels)
	}
	if c.TrustedProxies == nil {
		c.TrustedProxies = []string{}
	}
	if c.LevelColors == nil {
		c.LevelColors = []string{}
	}
	d := &c.DeviceDefaults
	d.Theme = strings.ToLower(strings.TrimSpace(d.Theme))
	d.Layout = strings.ToLower(strings.TrimSpace(d.Layout))
	d.Clock = strings.ToLower(strings.TrimSpace(d.Clock))
	d.SetStack = strings.ToLower(strings.TrimSpace(d.SetStack))
	d.NextMode = strings.ToLower(strings.TrimSpace(d.NextMode))
	if d.Theme == "" {
		d.Theme = "system"
	}
	if d.Layout == "" {
		d.Layout = "auto"
	}
	if d.Clock == "" {
		d.Clock = "24"
	}
	if d.SetStack == "" {
		d.SetStack = "and"
	}
	if d.NextMode == "" {
		d.NextMode = "number"
	}
	c.HiddenDsts = strings.TrimSpace(c.HiddenDsts)
	c.LegacyGrids, c.LegacyGridConfirm, c.Locks.LegacyGrid = nil, nil, nil
	if len(c.Presets) == 0 && len(c.LegacySalvos) > 0 {
		c.Presets = c.LegacySalvos // saved under the old key; the next save writes presets:
	}
	c.LegacySalvos = nil
	c.HideDsts = nil // superseded by HiddenDsts; dropped on the next save
	if c.FilterSets == nil {
		c.FilterSets = []FilterSet{}
	}
	for i := range c.FilterSets {
		f := &c.FilterSets[i]
		f.Name, f.Src, f.Dst = strings.TrimSpace(f.Name), strings.TrimSpace(f.Src), strings.TrimSpace(f.Dst)
	}
	c.normalizePresets()
	for i, v := range c.LevelColors {
		c.LevelColors[i] = normColor(v)
	}
	// Drop trailing blanks so an all-default list saves as [].
	for len(c.LevelColors) > 0 && c.LevelColors[len(c.LevelColors)-1] == "" {
		c.LevelColors = c.LevelColors[:len(c.LevelColors)-1]
	}
}

// normColor turns "#ABC", "abc", "AABBCC" into "#aabbcc". Anything else is
// returned trimmed for Validate to reject.
func normColor(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	h := strings.TrimPrefix(v, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 || strings.Trim(h, "0123456789abcdef") != "" {
		return v
	}
	return "#" + h
}

func (c Config) Validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if c.Router != "" {
		host, port, err := net.SplitHostPort(c.Router)
		if p, perr := strconv.Atoi(port); err != nil || host == "" || perr != nil || p < 1 || p > 65535 {
			bad("router must be host:port, e.g. 10.0.0.50:8910")
		}
	}
	switch c.NameRequests {
	case "std":
		// Standard messages carry the matrix in a 4-bit field.
		if c.Matrix < 0 || c.Matrix > 15 {
			bad("matrix must be 0-15 with standard name requests")
		}
	case "ext":
		if c.Matrix < 0 || c.Matrix > 255 {
			bad("matrix must be 0-255")
		}
	default:
		bad("name requests must be std or ext")
	}
	switch c.FireMode {
	case "sequential":
	case "native":
		// Group salvos carry the matrix in a 4-bit field.
		if c.Matrix < 0 || c.Matrix > 15 {
			bad("native salvos need matrix 0-15")
		}
	default:
		bad("fire mode must be sequential or native")
	}
	if c.SalvoGroup < 0 || c.SalvoGroup > 127 {
		bad("salvo group must be 0-127")
	}
	if c.Levels < 1 || c.Levels > 256 {
		bad("levels must be 1-256")
	} else if lo, hi := 1+c.LevelWireOffset, c.Levels+c.LevelWireOffset; lo < 0 || hi > 255 {
		bad("level wire offset puts levels at wire %d-%d; they must fall within 0-255", lo, hi)
	} else if _, err := ParseLevels(c.DefaultLevels, c.Levels); err != nil {
		bad("default levels: %v", err)
	}
	switch c.NameLength {
	case 4, 8, 12, 16:
	default:
		bad("name length must be 4, 8, 12 or 16")
	}
	if c.NumberOffset < 0 || c.NumberOffset > 1 {
		bad("number offset must be 0 or 1")
	}
	if c.ConfirmLockoutMs < 0 || c.ConfirmLockoutMs > 10000 {
		bad("confirm lockout must be 0-10000 ms")
	}
	if s := c.NameRefreshSeconds; s != 0 && (s < 10 || s > 86400) {
		bad("name refresh must be 0 (off) or 10-86400 seconds")
	}
	if s := c.TallyResyncSeconds; s != 0 && (s < 30 || s > 86400) {
		bad("tally resync must be 0 (off) or 30-86400 seconds")
	}
	switch c.LevelNumbering {
	case "0", "1", "va":
	default:
		bad("level numbering must be 0, 1 or va")
	}
	switch c.RouterShow {
	case "address", "name", "none":
	default:
		bad("router status must show address, name or none")
	}
	if n := len([]rune(c.RouterName)); n > 32 {
		bad("router name must be at most 32 characters")
	}
	if n := len([]rune(c.BrandText)); n > 24 {
		bad("logo text must be at most 24 characters")
	}
	if _, ok := BrandColorByName(c.BrandBorder); c.BrandBorder != "" && !ok {
		bad("border colour must be one of %s", strings.Join(BrandNames(), ", "))
	}
	if c.PinMaxTries < 0 || c.PinMaxTries > 1000 {
		bad("PIN tries before lockout must be 0 (never) or 1-1000")
	}
	switch c.DeviceDefaults.Theme {
	case "system", "light", "dark":
	default:
		bad("default theme must be system, light or dark")
	}
	switch c.DeviceDefaults.Layout {
	case "auto", "phone", "desktop":
	default:
		bad("default layout must be auto, phone or desktop")
	}
	switch c.DeviceDefaults.Clock {
	case "24", "12", "locale":
	default:
		bad("default clock must be 24, 12 or locale")
	}
	switch c.DeviceDefaults.SetStack {
	case "and", "or":
	default:
		bad("default set stacking must be and or or")
	}
	switch c.DeviceDefaults.NextMode {
	case "number", "fuzzy":
	default:
		bad("default next must be number or fuzzy")
	}
	for i, v := range c.LevelColors {
		if h, ok := strings.CutPrefix(v, "#"); v != "" && (!ok || len(h) != 6 || strings.Trim(h, "0123456789abcdef") != "") {
			bad("level %d color %q must be a hex color like #3a7bd5", i+1, v)
		}
	}
	if n := len([]rune(c.HiddenDsts)); n > 500 {
		bad("globally hidden destinations must be at most 500 characters")
	}
	fnames := make([]string, len(c.FilterSets))
	for i, f := range c.FilterSets {
		fnames[i] = f.Name
		if len([]rune(f.Src)) > 500 || len([]rune(f.Dst)) > 500 {
			bad("filter set %q: expressions must be at most 500 characters", f.Name)
		}
	}
	errs = append(errs, validateNames("filter set", fnames, 30)...)
	// Like presets, all of them in one message under the read limit.
	if b, err := json.Marshal(c.FilterSets); err == nil && len(b) > MaxFilterSetsBytes {
		bad("filter sets are too large to save (%d KiB, at most %d KiB)", len(b)>>10, MaxFilterSetsBytes>>10)
	}
	errs = append(errs, c.validatePresets()...)
	if _, err := c.Proxies(); err != nil {
		bad("trusted proxies: %v", err)
	}
	errs = append(errs, c.validateUsers()...)
	return errors.Join(errs...)
}

// Proxies parses TrustedProxies as IPs or CIDR prefixes.
func (c Config) Proxies() ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, s := range c.TrustedProxies {
		if p, err := netip.ParsePrefix(s); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("%q is not an IP or CIDR", s)
		}
		out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
	}
	return out, nil
}

// DefaultLevelColors are the user's Ultrix level colors, level 1 first.
// Levels past these fall back to the patchbay swatches.
var DefaultLevelColors = []string{
	"#7ae85a", "#3b5bfd", "#e5483b", "#f5e663", "#3f8a8c", "#d946ef",
	"#3f7f2e", "#67e8f9", "#7e2a86", "#f0963e", "#b5efcb", "#e287e8",
	"#5b86f0", "#8a8a35", "#a5e3f7", "#4fa03a", "#f29bb0",
}

// Level maps a UI level number to its SW-P-08 wire level.
type Level struct {
	UI     int    `json:"ui"`
	Wire   int    `json:"wire"`
	Label  string `json:"label"`           // as shown, per LevelNumbering
	Swatch string `json:"swatch"`          // patchbay pb-level-- suffix
	Color  string `json:"color,omitempty"` // #rrggbb, configured or default; overrides Swatch
}

// LevelMap: wire = UI + offset; UI 1 gets the video swatch, the rest the
// 16 audio swatches, unless LevelColors or DefaultLevelColors has a color.
func (c Config) LevelMap() []Level {
	out := make([]Level, c.Levels)
	for i := range out {
		ui := i + 1
		l := Level{UI: ui, Wire: ui + c.LevelWireOffset, Label: c.LevelLabel(ui), Swatch: "video"}
		if ui > 1 {
			l.Swatch = fmt.Sprintf("%02d", (ui-2)%16+1)
		}
		if i < len(c.LevelColors) && c.LevelColors[i] != "" {
			l.Color = c.LevelColors[i]
		} else if i < len(DefaultLevelColors) {
			l.Color = DefaultLevelColors[i]
		}
		out[i] = l
	}
	return out
}

// LevelLabel is how UI level ui is shown: "0" → ui-1, "va" → V, 1, 2…,
// otherwise ui.
func (c Config) LevelLabel(ui int) string {
	switch c.LevelNumbering {
	case "0":
		return strconv.Itoa(ui - 1)
	case "va":
		if ui == 1 {
			return "V"
		}
		return strconv.Itoa(ui - 1)
	}
	return strconv.Itoa(ui)
}

// ParseLevels parses "1", "1-3", "1,3,5-8" within 1..max.
func ParseLevels(s string, max int) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		a, b, isRange := strings.Cut(part, "-")
		lo, err1 := strconv.Atoi(a)
		hi := lo
		var err2 error
		if isRange {
			hi, err2 = strconv.Atoi(b)
		}
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("%q is not a level or range", part)
		}
		if lo > hi {
			lo, hi = hi, lo
		}
		if lo < 1 || hi > max {
			return nil, fmt.Errorf("%q is outside 1-%d", part, max)
		}
		for i := lo; i <= hi; i++ {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no levels")
	}
	return out, nil
}

// Load reads path over the defaults. exists is false if there is no file.
func Load(path string) (c Config, exists bool, err error) {
	c = Defaults()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && !errors.Is(err, io.EOF) {
		return c, true, fmt.Errorf("%s: %w", path, err)
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		return c, true, fmt.Errorf("%s: %w", path, err)
	}
	return c, true, nil
}

const header = "# Sode settings. Edited by the settings dialog; hand edits apply on restart.\n"

// Save writes c atomically: temp file, fsync, rename.
func Save(path string, c Config) error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return atomicWrite(path, []byte(header), b)
}

// atomicWrite writes the parts to path through a temp file in the same
// directory (fsync, then rename), so a crash leaves the old file or the new.
func atomicWrite(path string, parts ...[]byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sode-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	for _, p := range parts {
		if _, err = tmp.Write(p); err != nil {
			break
		}
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

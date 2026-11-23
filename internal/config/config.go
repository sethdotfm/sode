// Package config loads, validates and saves Sode's YAML settings.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Config is everything editable from the settings dialog. Process-level
// settings (listen address, file paths, debug) stay in env vars.
type Config struct {
	Format int `yaml:"format" json:"-"`
	// Never changes: Item links and import identify this install by it.
	InstallUID string `yaml:"installUid" json:"-"`
	// The first router: the file holds it as the first entry of routers (router.go). Its fields are the old
	// single-router fields.
	Router `yaml:"-"`
	// MoreRouters: the routers after the first, in file order. Routers gives the whole list.
	MoreRouters []Router `yaml:"-" json:"-"`
	// Brand fields are display only.
	BrandText            string   `yaml:"brandText,omitempty" json:"brandText"`
	BrandBorder          string   `yaml:"brandBorder,omitempty" json:"brandBorder"`
	ConfirmLockoutMs     int      `yaml:"confirmLockoutMs" json:"confirmLockoutMs"`
	ResultDismissSeconds int      `yaml:"resultDismissSeconds" json:"resultDismissSeconds"`
	ConfirmEnter         bool     `yaml:"confirmEnter" json:"confirmEnter"`
	TrustedProxies       []string `yaml:"trustedProxies" json:"trustedProxies"`
	// HiddenDsts is a History filter expression (web/filter.js) that hides Externals on these destinations; Sode
	// records them anyway.
	HiddenDsts string `yaml:"hiddenDsts,omitempty" json:"hiddenDsts"`
	// HiddenDstsRouter: the router UID whose numbers HiddenDsts's number and range terms mean; blank is the one router.
	HiddenDstsRouter string      `yaml:"hiddenDstsRouter,omitempty" json:"hiddenDstsRouter,omitempty"`
	FilterSets       []FilterSet `yaml:"filterSets,omitempty" json:"filterSets"`
	Presets          []Preset    `yaml:"presets,omitempty" json:"presets"`
	// Bookmarks is Everyone's list (GLOSSARY.md, Bookmark).
	Bookmarks []Bookmark `yaml:"bookmarks,omitempty" json:"bookmarks"`
	// TieLines: Settings > Routers > Tie lines (tielines.go). An Administrator saves the list apart from the form.
	TieLines []TieLine `yaml:"tieLines,omitempty" json:"tieLines"`
	// TieHopWeight is the cost of each hop that a route through tie lines sends upstream (internal/tieplan).
	TieHopWeight int `yaml:"tieHopWeight" json:"tieHopWeight"`
	// LevelColors[i] is #rrggbb for UI level i+1; "" keeps the default; entries past Levels are unused.
	LevelColors []string `yaml:"levelColors" json:"levelColors"`
	// PinWaits is the Wait after wrong PINs switch. Administrator names wait whatever it says.
	PinWaits       bool           `yaml:"pinWaits" json:"pinWaits"`
	DeviceDefaults DeviceDefaults `yaml:"browserDefaults" json:"browserDefaults"`
	Users          []User         `yaml:"users,omitempty" json:"users"`
	// NewUserDefaults has no value in Defaults: a file from before it loads both off, as new users started then.
	NewUserDefaults NewUserDefaults `yaml:"newUserDefaults" json:"newUserDefaults"`
	Locks           Locks           `yaml:"locks" json:"locks"`
	AutoLockMinutes int             `yaml:"autoLockMinutes" json:"autoLockMinutes"`
	// MaxPinLen is the longest user PIN (4-12).
	MaxPinLen int `yaml:"maxPinLen" json:"maxPinLen"`
	// Clients, Guest rights and scopes stay out of JSON: never sent before the PIN.
	API         APISettings `yaml:"api" json:"api"`
	APIClients  []APIClient `yaml:"apiClients,omitempty" json:"-"`
	GuestRights Rights      `yaml:"guestRights,omitempty" json:"-"`
	// GuestRole: RoleStandard or RoleTrusted, never RoleAdministrator.
	GuestRole      string   `yaml:"guestRole" json:"guestRole"`
	GuestAPIScopes []string `yaml:"guestApiScopes" json:"-"`
	// Off: no panel hands out a Public key (panelpublic.go).
	PublicPanels bool `yaml:"publicPanels" json:"publicPanels"`
	// Prefills a blank new panel's acts only.
	NewPanelActs string `yaml:"newPanelActs" json:"newPanelActs"`
	// RequireHTTPS refuses requests a trusted proxy did not mark secure (ADR 0005).
	RequireHTTPS bool `yaml:"requireHttps" json:"requireHttps"`
	// Extra holds keys a newer release wrote, so a rollback keeps them; never mutated in place, so Clone shares it.
	Extra map[string]any `yaml:",inline" json:"-"`
	// idsFilled: Normalize gave a user an id; Load writes it back, or the next start makes new ones.
	idsFilled bool
}

// PanelActs are a panel's Authentication modes as stored (acts); keep in step with panelActs* in
// internal/app/panelstore.go.
var PanelActs = []string{"session", "api", "both", "public"}

// MaxFilterSetsBytes keeps a filterSets message under the 64 KiB read limit.
const MaxFilterSetsBytes = 48 << 10

// Hex is the dark-mode value web/base.css maps the name to. The server draws the app icon with it, and the page makes
// the accent from it (web/accent.js). A Retired color is not offered, but a config that has it still loads and draws.
type BrandColor struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Hex     string `json:"hex"`
	Retired bool   `json:"retired,omitempty"`
}

// Never red: that is the lost-link outline. The offered hues stay clear of the state colors (docs/color-roles.md).
// Green, amber and pink are retired: their hues are routed, caution and protected.
var BrandColors = []BrandColor{
	{"blue", "Blue", "#5fb2e8", false},
	{"teal", "Teal", "#5ccfc9", false},
	{"sky", "Sky", "#4dc3dd", false},
	{"slate", "Slate", "#a3b4d4", false},
	{"violet", "Violet", "#a67ef0", false},
	{"green", "Green", "#5fd09a", true},
	{"amber", "Amber", "#e8a35e", true},
	{"pink", "Pink", "#e87bb3", true},
}

func BrandColorByName(name string) (BrandColor, bool) {
	i := slices.IndexFunc(BrandColors, func(c BrandColor) bool { return c.Name == name })
	if i < 0 {
		return BrandColor{}, false
	}
	return BrandColors[i], true
}

var customHex = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// BrandHex is the tile and border colour of a brandBorder value: a BrandColors name, or a custom "#rrggbb"
// (lowercase, as Normalize leaves it).
func BrandHex(v string) (string, bool) {
	if c, ok := BrandColorByName(v); ok {
		return c.Hex, true
	}
	if customHex.MatchString(v) {
		return v, true
	}
	return "", false
}

// BrandNames are the offered names, without the retired ones.
func BrandNames() []string {
	var out []string
	for _, c := range BrandColors {
		if !c.Retired {
			out = append(out, c.Name)
		}
	}
	return out
}

// DeviceDefaults mirrors the per-browser prefs in web/prefs.js.
type DeviceDefaults struct {
	Theme         string `yaml:"theme" json:"theme"`
	Layout        string `yaml:"layout" json:"layout"`
	Clock         string `yaml:"clock" json:"clock"`
	Flip          bool   `yaml:"flip" json:"flip"`
	ScrollTop     bool   `yaml:"scrollTop" json:"scrollTop"`
	SearchFilters bool   `yaml:"searchFilters" json:"searchFilters"`
	SetStack      string `yaml:"setStack" json:"setStack"`
	NextMode      string `yaml:"nextMode" json:"nextMode"`
	StackGap      string `yaml:"stackGap" json:"stackGap"`
	Compact       bool   `yaml:"compact" json:"compact"`
	// PaletteChar is one of PaletteChars, or "off" for no Palette character.
	PaletteChar string         `yaml:"paletteChar" json:"paletteChar"`
	Extra       map[string]any `yaml:",inline" json:"-"`
}

// deviceEnums: the string device defaults. Allowed values are PrefValues; empty means the Defaults value.
var deviceEnums = []struct {
	pref, msg string
	get       func(*DeviceDefaults) *string
}{
	{"theme", "default theme must be system, light or dark", func(d *DeviceDefaults) *string { return &d.Theme }},
	{"layout", "default layout must be auto, phone or desktop", func(d *DeviceDefaults) *string { return &d.Layout }},
	{"clock", "default clock must be 24, 12 or locale", func(d *DeviceDefaults) *string { return &d.Clock }},
	{"setStack", "default set stacking must be and or or", func(d *DeviceDefaults) *string { return &d.SetStack }},
	{"nextMode", "default next must be number or fuzzy", func(d *DeviceDefaults) *string { return &d.NextMode }},
	{"stackGap", "default stacked rows gap must be 0, 10, 30, 60, 120 or 300 seconds", func(d *DeviceDefaults) *string { return &d.StackGap }},
	{"paletteChar", "default Palette character must be one of " + PaletteChars + " or off", func(d *DeviceDefaults) *string { return &d.PaletteChar }},
}

// Router: blank is the one router. ID is hidden and stays through a rename, as a preset's does. Bookmarks and the
// active set name a set by it.
type FilterSet struct {
	ID     string         `yaml:"id" json:"id"`
	Name   string         `yaml:"name" json:"name"`
	Router string         `yaml:"router,omitempty" json:"router,omitempty"`
	Src    string         `yaml:"src,omitempty" json:"src"`
	Dst    string         `yaml:"dst,omitempty" json:"dst"`
	Sode   bool           `yaml:"sode" json:"sode"`
	Ext    bool           `yaml:"ext" json:"ext"`
	Hidden bool           `yaml:"hidden,omitempty" json:"hidden"`
	Broken []Missing      `yaml:"broken,omitempty" json:"broken,omitempty"`
	Extra  map[string]any `yaml:",inline" json:"-"`
}

func Defaults() Config {
	return Config{
		Format: Format,
		Router: Router{
			Prefix:             "A",
			RouterShow:         "address",
			Matrix:             0,
			Levels:             17,
			LevelWireOffset:    -1,
			NameLength:         12,
			NameRequests:       "std",
			FireMode:           "sequential",
			SalvoGroup:         127,
			NumberOffset:       1,
			NameRefreshSeconds: 60,
			TallyResyncSeconds: 300,
			LevelNumbering:     "1",
		},
		ConfirmLockoutMs:     500,
		ResultDismissSeconds: 8,
		TieHopWeight:         DefaultTieHopWeight,
		TrustedProxies:       []string{},
		LevelColors:          []string{},
		PinWaits:             true,
		PublicPanels:         true,
		NewPanelActs:         "public",
		MaxPinLen:            DefaultMaxPinLen,
		API:                  DefaultAPISettings(),
		GuestAPIScopes:       slices.Clone(DefaultGuestScopes),
		DeviceDefaults:       DeviceDefaults{Theme: "system", Layout: "auto", Clock: "24", ScrollTop: true, SetStack: "and", NextMode: "number", StackGap: "30", PaletteChar: PaletteDefault},
	}
}

// Seed only: used to create the config file on first start.
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
	str("SODE_ROUTER", &c.Address)
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
	num("SODE_RESULT_DISMISS_S", &c.ResultDismissSeconds)
	num("SODE_TIE_HOP_WEIGHT", &c.TieHopWeight)
	num("SODE_NAME_REREAD_S", &c.NameRefreshSeconds)
	num("SODE_TALLY_RESYNC_S", &c.TallyResyncSeconds)
	str("SODE_LEVEL_NUMBERING", &c.LevelNumbering)
	num("SODE_MAX_PIN_LEN", &c.MaxPinLen)
	boolean := func(k string, dst *bool) {
		if v := os.Getenv(k); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", k, err))
			}
			*dst = b
		}
	}
	boolean("SODE_CONFIRM_ENTER", &c.ConfirmEnter)
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

// For the "Filled from" hints. Keep in step with FromEnv, AccessPresetFromEnv and web/setupwizardrules.js keys.
func WizardEnv() map[string]string {
	out := map[string]string{}
	for _, f := range [][2]string{
		{"router", "SODE_ROUTER"}, {"levels", "SODE_LEVEL_COUNT"}, {"levelNumbering", "SODE_LEVEL_NUMBERING"},
		{"numberOffset", "SODE_NUM_OFFSET"}, {"fireMode", "SODE_FIRE_MODE"}, {"group", "SODE_SALVO_GROUP"},
		{"preset", "SODE_ACCESS_PRESET"},
	} {
		if strings.TrimSpace(os.Getenv(f[1])) != "" {
			out[f[0]] = f[1]
		}
	}
	return out
}

// Clone shares no slices, so Normalize cannot touch a config others read.
func (c Config) Clone() Config {
	c.MoreRouters = slices.Clone(c.MoreRouters)
	c.TrustedProxies = slices.Clone(c.TrustedProxies)
	c.LevelColors = slices.Clone(c.LevelColors)
	c.FilterSets = slices.Clone(c.FilterSets)
	c.Presets = ClonePresets(c.Presets)
	c.Bookmarks = CloneBookmarks(c.Bookmarks)
	c.TieLines = CloneTieLines(c.TieLines)
	c.Users = slices.Clone(c.Users)
	for i := range c.Users {
		c.Users[i].Profile = c.Users[i].Profile.Clone()
		c.Users[i].Rights = c.Users[i].Rights.Clone()
	}
	c.APIClients = slices.Clone(c.APIClients)
	for i := range c.APIClients {
		c.APIClients[i].Scopes = slices.Clone(c.APIClients[i].Scopes)
		c.APIClients[i].Rights = c.APIClients[i].Rights.Clone()
	}
	c.GuestRights = c.GuestRights.Clone()
	c.GuestAPIScopes = slices.Clone(c.GuestAPIScopes)
	return c
}

func (c *Config) Normalize() {
	c.Router.normalize()
	for i := range c.MoreRouters {
		c.MoreRouters[i].normalize()
	}
	c.InstallUID = strings.TrimSpace(c.InstallUID)
	if c.InstallUID == "" {
		c.InstallUID = newRouterUID()
	}
	c.BrandText = strings.TrimSpace(c.BrandText)
	if c.NewPanelActs = strings.ToLower(strings.TrimSpace(c.NewPanelActs)); c.NewPanelActs == "" {
		c.NewPanelActs = "public"
	}
	c.normalizeUsers()
	c.normalizeAPI()
	c.dropRetiredKeys()
	c.BrandBorder = strings.ToLower(strings.TrimSpace(c.BrandBorder))
	if c.TrustedProxies == nil {
		c.TrustedProxies = []string{}
	}
	if c.LevelColors == nil {
		c.LevelColors = []string{}
	}
	def := Defaults().DeviceDefaults
	for _, e := range deviceEnums {
		v := e.get(&c.DeviceDefaults)
		if *v = strings.ToLower(strings.TrimSpace(*v)); *v == "" {
			*v = *e.get(&def)
		}
	}
	c.HiddenDsts = strings.TrimSpace(c.HiddenDsts)
	c.FilterSets = normalizeFilterSets(c.FilterSets)
	c.normalizePresets()
	c.Bookmarks = normalizeBookmarks(c.Bookmarks)
	c.TieLines = normalizeTieLines(c.TieLines)
	c.normalizeProfiles()
	for i, v := range c.LevelColors {
		c.LevelColors[i] = normColor(v)
	}
	// Drop trailing blanks so an all-default list saves as [].
	for len(c.LevelColors) > 0 && c.LevelColors[len(c.LevelColors)-1] == "" {
		c.LevelColors = c.LevelColors[:len(c.LevelColors)-1]
	}
}

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

var DefaultLevelColors = []string{
	"#7ae85a", "#4c69fd", "#e5483b", "#f5e663", "#3f8a8c", "#d946ef",
	"#4a8639", "#67e8f9", "#a061a5", "#f0963e", "#b5efcb", "#e287e8",
	"#5b86f0", "#8a8a35", "#a5e3f7", "#4fa03a", "#f29bb0",
}

type Level struct {
	UI     int    `json:"ui"`
	Wire   int    `json:"wire"`
	Label  string `json:"label"`
	Swatch string `json:"swatch"`
	Color  string `json:"color,omitempty"` // overrides Swatch
}

// UI 1 gets the video swatch, the rest the 16 audio swatches.
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

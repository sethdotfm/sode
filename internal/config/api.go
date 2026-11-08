package config

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// API scopes say which API areas a caller may use; Rights say which routes it may command.
const (
	ScopeNames     = "names"
	ScopeTally     = "tally"
	ScopeProtected = "protected"
	ScopeStatus    = "status"
	ScopePresets   = "presets"
	ScopeHistory   = "history"
	ScopeLayouts   = "layouts"
	ScopeLog       = "log"
	ScopeTake      = "take"
)

var AllScopes = []string{ScopeNames, ScopeTally, ScopeProtected, ScopeStatus, ScopePresets, ScopeHistory, ScopeLayouts, ScopeLog, ScopeTake}

// DefaultGuestScopes is what a call with no key may read until an administrator widens it.
var DefaultGuestScopes = []string{ScopeNames, ScopeTally, ScopeProtected, ScopeStatus}

// Visibility values; "" means read-only.
const (
	VisReadOnly = "readonly"
	VisHidden   = "hidden"
)

const (
	MaxAPIClients        = 12
	DefaultRateLimit     = 600 // requests per minute per key
	DefaultTakeCap       = 4   // API takes in flight; the router answers one message per ACK
	DefaultMaxUserKeys   = 8
	DefaultMaxClientKeys = 20
)

// Rights is an allowlist per kind; a nil list means all and an empty one means none, so the
// lists are pointers (omitempty would turn "none" back into "all" on reload). Never sent
// before the PIN: the hello blanks Users and the owning Config fields carry json:"-".
type Rights struct {
	Sources    *[]int    `yaml:"sources,omitempty" json:"sources,omitempty"`
	Dsts       *[]int    `yaml:"dsts,omitempty" json:"dsts,omitempty"`
	Levels     *[]int    `yaml:"levels,omitempty" json:"levels,omitempty"` // UI levels
	Presets    *[]string `yaml:"presets,omitempty" json:"presets,omitempty"`
	Visibility string    `yaml:"visibility,omitempty" json:"visibility,omitempty"`
}

func in[T comparable](l *[]T, v T) bool { return l == nil || slices.Contains(*l, v) }

func (r Rights) AllowsSource(i int) bool    { return in(r.Sources, i) }
func (r Rights) AllowsDst(i int) bool       { return in(r.Dsts, i) }
func (r Rights) AllowsLevel(ui int) bool    { return in(r.Levels, ui) }
func (r Rights) AllowsPreset(n string) bool { return in(r.Presets, n) }
func (r Rights) Hidden() bool               { return r.Visibility == VisHidden }

// IsOpen: every list unset and read-only, the state of an old file.
func (r Rights) IsOpen() bool {
	return r.Sources == nil && r.Dsts == nil && r.Levels == nil && r.Presets == nil && !r.Hidden()
}

func cloneList[T any](l *[]T) *[]T {
	if l == nil {
		return nil
	}
	c := slices.Clone(*l)
	if c == nil {
		c = []T{}
	}
	return &c
}

func (r Rights) Clone() Rights {
	r.Sources, r.Dsts, r.Levels, r.Presets = cloneList(r.Sources), cloneList(r.Dsts), cloneList(r.Levels), cloneList(r.Presets)
	return r
}

// APIClient is a non-person actor. Its keys live in the key store, not here.
type APIClient struct {
	Name   string   `yaml:"name" json:"name"`
	Scopes []string `yaml:"scopes" json:"scopes"`
	Rights Rights   `yaml:"rights,omitempty" json:"rights"`
}

// APISettings: advanced settings, saved apart from the Settings form (owned).
type APISettings struct {
	Enabled       bool `yaml:"enabled" json:"enabled"`
	RateLimit     int  `yaml:"rateLimit" json:"rateLimit"` // per key per minute, 0 = none
	TakeCap       int  `yaml:"takeCap" json:"takeCap"`
	GuestEnabled  bool `yaml:"guestEnabled" json:"guestEnabled"`
	LogReads      bool `yaml:"logReads" json:"logReads"`
	MaxUserKeys   int  `yaml:"maxUserKeys" json:"maxUserKeys"`
	MaxClientKeys int  `yaml:"maxClientKeys" json:"maxClientKeys"`
}

func DefaultAPISettings() APISettings {
	return APISettings{RateLimit: DefaultRateLimit, TakeCap: DefaultTakeCap, MaxUserKeys: DefaultMaxUserKeys, MaxClientKeys: DefaultMaxClientKeys}
}

func (c Config) FindAPIClient(name string) (APIClient, bool) {
	for _, a := range c.APIClients {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return APIClient{}, false
}

func (c *Config) normalizeAPI() {
	c.GuestRights.Visibility = strings.ToLower(strings.TrimSpace(c.GuestRights.Visibility))
	for i := range c.Users {
		c.Users[i].Rights.Visibility = strings.ToLower(strings.TrimSpace(c.Users[i].Rights.Visibility))
	}
	if c.GuestAPIScopes == nil {
		c.GuestAPIScopes = slices.Clone(DefaultGuestScopes)
	}
	for i := range c.APIClients {
		a := &c.APIClients[i]
		a.Name = strings.TrimSpace(a.Name)
		a.Rights.Visibility = strings.ToLower(strings.TrimSpace(a.Rights.Visibility))
		if a.Scopes == nil {
			a.Scopes = []string{}
		}
	}
}

func (r Rights) validate(who string, levels int, bad func(string, ...any)) {
	if r.Visibility != "" && r.Visibility != VisReadOnly && r.Visibility != VisHidden {
		bad("%s: visibility must be %q or %q", who, VisReadOnly, VisHidden)
	}
	for _, l := range []*[]int{r.Sources, r.Dsts} {
		if l != nil && slices.ContainsFunc(*l, func(n int) bool { return n < 0 }) {
			bad("%s: sources and destinations can't be negative", who)
		}
	}
	if r.Levels != nil && slices.ContainsFunc(*r.Levels, func(n int) bool { return n < 1 || n > levels }) {
		bad("%s: levels must be 1-%d", who, levels)
	}
}

func checkScopes(who string, scopes []string, bad func(string, ...any)) {
	for _, s := range scopes {
		if !slices.Contains(AllScopes, s) {
			bad("%s: unknown scope %q", who, s)
		}
	}
}

func (c Config) validateAPI() []error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	a := c.API
	if a.RateLimit < 0 || a.RateLimit > 100000 {
		bad("API rate limit must be 0 (none) or 1-100000 per minute")
	}
	if a.TakeCap < 1 || a.TakeCap > 64 {
		bad("API take cap must be 1-64")
	}
	if a.MaxUserKeys < 1 || a.MaxUserKeys > 100 || a.MaxClientKeys < 1 || a.MaxClientKeys > 100 {
		bad("key caps must be 1-100")
	}
	if c.RequireHTTPS && len(c.TrustedProxies) == 0 {
		bad("Require HTTPS needs at least one trusted proxy: only a trusted proxy can say a request was secure")
	}
	c.GuestRights.validate("Guest rights", c.Levels, bad)
	checkScopes("Guest API scopes", c.GuestAPIScopes, bad)
	for _, u := range c.Users {
		u.Rights.validate("rights of "+u.Name, c.Levels, bad)
	}
	if len(c.APIClients) > MaxAPIClients {
		bad("at most %d API clients", MaxAPIClients)
	}
	seen := map[string]bool{}
	for _, cl := range c.APIClients {
		key := strings.ToLower(cl.Name)
		switch n := utf8.RuneCountInString(cl.Name); {
		case n == 0:
			bad("every API client needs a name")
		case n > MaxUserName:
			bad("API client name %q must be at most %d characters", cl.Name, MaxUserName)
		case strings.IndexFunc(cl.Name, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0:
			bad("API client name %q must use printable characters only", cl.Name)
		case seen[key]:
			bad("two API clients are named %q", cl.Name)
		}
		if _, ok := c.FindUser(cl.Name); ok {
			bad("API client %q has the name of a user; History would not tell them apart", cl.Name)
		}
		seen[key] = true
		cl.Rights.validate("rights of API client "+cl.Name, c.Levels, bad)
		checkScopes("API client "+cl.Name, cl.Scopes, bad)
	}
	return errs
}

package config

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// API scopes say which API areas a caller can use; Rights say which routes it can command.
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
	// ScopeProtect sets and frees Sode's Protects; the caller's Rights need the Protect right too.
	ScopeProtect = "protect"
	// Panels: reading is for any panel the caller can use. Managing is create, update, delete,
	// Save a copy and artwork uploads, and needs a user's key to own what it makes.
	ScopePanels       = "panels"
	ScopePanelsManage = "panelsmanage"
)

var AllScopes = []string{ScopeNames, ScopeTally, ScopeProtected, ScopeStatus, ScopePresets, ScopeHistory, ScopeLayouts, ScopeLog, ScopeTake, ScopeProtect, ScopePanels, ScopePanelsManage}

// MaxProtectDevice: the protect dump packs the device number in 12 bits.
const MaxProtectDevice = 4095

// DefaultGuestScopes is what a call with no key can read until an administrator widens it.
var DefaultGuestScopes = []string{ScopeNames, ScopeTally, ScopeProtected, ScopeStatus}

// Visibility values; "" means read-only.
const (
	VisReadOnly = "readonly"
	VisHidden   = "hidden"
)

const (
	MaxAPIClients    = 12
	DefaultRateLimit = 600 // requests per minute per key
	DefaultTakeCap   = 4   // API takes in flight; the router answers one message per ACK
)

// Rights is an allowlist per kind. A nil list means all and an empty one means none, so the
// lists are pointers (omitempty would turn "none" back into "all" on reload). Never sent
// before the PIN: the hello blanks Users and the owning Config fields carry json:"-".
// Sources and destinations are router references; Presets are preset ids, so a rename keeps them.
//
// Protect is the Protect right: set and free Sode's Protects on the destinations the lists allow. Unset means the
// Role decides (ProtectFor): on for a Trusted user and an Administrator, off for everyone else.
//
// TieLines is the "Use tie lines" right: send a route across routers (docs/tielines-design.md, Rights). Unset is on,
// as an unset list is everything, so a file from before the right changes nothing. An Access preset writes it for
// the Guest.
type Rights struct {
	Sources    *[]Ref         `yaml:"sources,omitempty" json:"sources,omitempty"`
	Dsts       *[]Ref         `yaml:"dsts,omitempty" json:"dsts,omitempty"`
	Levels     *[]int         `yaml:"levels,omitempty" json:"levels,omitempty"` // UI levels
	Presets    *[]string      `yaml:"presets,omitempty" json:"presets,omitempty"`
	Protect    *bool          `yaml:"protect,omitempty" json:"protect,omitempty"`
	TieLines   *bool          `yaml:"tieLines,omitempty" json:"tieLines,omitempty"`
	Visibility string         `yaml:"visibility,omitempty" json:"visibility,omitempty"`
	Extra      map[string]any `yaml:",inline" json:"-"`
}

func in[T comparable](l *[]T, v T) bool { return l == nil || slices.Contains(*l, v) }

// sameRef: the router and the number decide, never the name. A blank router matches any router. Only a request
// leaves it blank: Normalize fills a stored one, and every check in internal/app fills the one it asks about.
func sameRef(a, b Ref) bool {
	return a.N == b.N && (a.Router == "" || b.Router == "" || a.Router == b.Router)
}

func inRefs(l *[]Ref, x Ref) bool {
	return l == nil || slices.ContainsFunc(*l, func(r Ref) bool { return sameRef(r, x) })
}

// x names its router: a right for source 5 on one router does not allow source 5 on another.
func (r Rights) AllowsSource(x Ref) bool    { return inRefs(r.Sources, x) }
func (r Rights) AllowsDst(x Ref) bool       { return inRefs(r.Dsts, x) }
func (r Rights) AllowsLevel(ui int) bool    { return in(r.Levels, ui) }
func (r Rights) AllowsPreset(n string) bool { return in(r.Presets, n) }
func (r Rights) Hidden() bool               { return r.Visibility == VisHidden }

// AllowsProtect: an unset right is off, so an API client needs it ticked. ActsAs and GuestActs set it from the Role.
func (r Rights) AllowsProtect() bool { return r.Protect != nil && *r.Protect }

// AllowsTieLines: the "Use tie lines" right. Unset is on.
func (r Rights) AllowsTieLines() bool { return r.TieLines == nil || *r.TieLines }

// ProtectFor fills an unset Protect right from the Role: on for a Trusted user and an Administrator.
func (r Rights) ProtectFor(role string) Rights {
	if r.Protect == nil {
		on := RoleAtLeast(role, RoleTrusted)
		r.Protect = &on
	}
	return r
}

// protectWithin: only a right that is on needs it on in o. Unset is off for an API client, and ActsAs and GuestActs
// set it from the Role.
func protectWithin(r, o *bool) bool { return r == nil || !*r || o == nil || *o }

// tieLinesBoth: off when either is off; unset only when both are unset.
func tieLinesBoth(r, o Rights) *bool {
	if r.TieLines == nil && o.TieLines == nil {
		return nil
	}
	on := r.AllowsTieLines() && o.AllowsTieLines()
	return &on
}

// protectBoth: off when either is off; unset only when both are.
func protectBoth(r, o *bool) *bool {
	switch {
	case r == nil:
		return cloneBool(o)
	case o == nil:
		return cloneBool(r)
	}
	on := *r && *o
	return &on
}

func cloneBool(b *bool) *bool {
	if b == nil {
		return nil
	}
	v := *b
	return &v
}

func subset[T comparable](l, of *[]T) bool {
	return subsetBy(l, of, func(a, b T) bool { return a == b })
}

func subsetBy[T any](l, of *[]T, same func(a, b T) bool) bool {
	if of == nil {
		return true
	}
	if l == nil {
		return false
	}
	for _, v := range *l {
		if !slices.ContainsFunc(*of, func(o T) bool { return same(v, o) }) {
			return false
		}
	}
	return true
}

// Within: r grants nothing o does not (ADR 0006's cap). An unset list is everything. Hidden
// shows less than read-only, so it is within both, and read-only is not within hidden.
func (r Rights) Within(o Rights) bool {
	return subsetBy(r.Sources, o.Sources, sameRef) && subsetBy(r.Dsts, o.Dsts, sameRef) && subset(r.Levels, o.Levels) &&
		subset(r.Presets, o.Presets) && (r.Hidden() || !o.Hidden()) && protectWithin(r.Protect, o.Protect) &&
		(!r.AllowsTieLines() || o.AllowsTieLines())
}

func intersectBy[T any](l, o *[]T, same func(a, b T) bool) *[]T {
	switch {
	case l == nil:
		return cloneList(o)
	case o == nil:
		return cloneList(l)
	}
	out := []T{}
	for _, v := range *l {
		if slices.ContainsFunc(*o, func(w T) bool { return same(v, w) }) {
			out = append(out, v)
		}
	}
	return &out
}

func eq[T comparable](a, b T) bool { return a == b }

// Hidden on either side is hidden; r's Extra is kept.
func (r Rights) Intersect(o Rights) Rights {
	out := Rights{Sources: intersectBy(r.Sources, o.Sources, sameRef), Dsts: intersectBy(r.Dsts, o.Dsts, sameRef),
		Levels: intersectBy(r.Levels, o.Levels, eq[int]), Presets: intersectBy(r.Presets, o.Presets, eq[string]),
		Protect: protectBoth(r.Protect, o.Protect), TieLines: tieLinesBoth(r, o), Visibility: r.Visibility, Extra: r.Extra}
	if o.Hidden() {
		out.Visibility = VisHidden
	}
	return out
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
	r.Protect, r.TieLines = cloneBool(r.Protect), cloneBool(r.TieLines)
	return r
}

// PanelClientScopes: a panel's own API client starts with read, tally and take scopes,
// never
var PanelClientScopes = []string{ScopeNames, ScopeTally, ScopeProtected, ScopeStatus, ScopePresets, ScopeLayouts, ScopeTake, ScopePanels}

// APIClient is a non-person actor. Its keys live in the key store, not here. Panel is the id of
// the panel that owns a panel's own client (ADR 0006). Such a client is made and removed with
// its panel. It does not count toward MaxAPIClients, which bounds the administrator's own.
// Owner is the id of the user who made the client from My user. That user's Rights cap the
// client's live, and it goes with them. Each user has at most MaxAPIClients.
type APIClient struct {
	Name   string   `yaml:"name" json:"name"`
	Scopes []string `yaml:"scopes" json:"scopes"`
	Rights Rights   `yaml:"rights,omitempty" json:"rights"`
	Panel  string   `yaml:"panel,omitempty" json:"panel,omitempty"`
	Owner  string   `yaml:"owner,omitempty" json:"-"`
	// Never stored.
	OwnerName string `yaml:"-" json:"ownerName,omitempty"`
	// Networks limits where the client's keys work: CIDRs, empty means anywhere. Set through a
	// panel's Share tab (panelnet.go).
	Networks []string       `yaml:"networks,omitempty" json:"networks,omitempty"`
	Extra    map[string]any `yaml:",inline" json:"-"`
}

type APISettings struct {
	RateLimit    int            `yaml:"rateLimit" json:"rateLimit"` // per key per minute, 0 = none
	TakeCap      int            `yaml:"takeCap" json:"takeCap"`
	GuestEnabled bool           `yaml:"guestEnabled" json:"guestEnabled"`
	LogReads     bool           `yaml:"logReads" json:"logReads"`
	Extra        map[string]any `yaml:",inline" json:"-"`
}

func DefaultAPISettings() APISettings {
	return APISettings{RateLimit: DefaultRateLimit, TakeCap: DefaultTakeCap}
}

func (c Config) FindAPIClient(name string) (APIClient, bool) {
	for _, a := range c.APIClients {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return APIClient{}, false
}

func (r *Rights) fillRouter(uid string) {
	RefWalk{Fn: func(_ string, x Ref) []Ref {
		if x.Router != "" {
			return nil
		}
		x.Router = uid
		return []Ref{x}
	}}.Rights(r)
}

func (c *Config) normalizeAPI() {
	c.GuestRights.Visibility = strings.ToLower(strings.TrimSpace(c.GuestRights.Visibility))
	c.GuestRights.fillRouter(c.UID)
	for i := range c.Users {
		c.Users[i].Rights.Visibility = strings.ToLower(strings.TrimSpace(c.Users[i].Rights.Visibility))
		c.Users[i].Rights.fillRouter(c.UID)
	}
	if c.GuestAPIScopes == nil {
		c.GuestAPIScopes = slices.Clone(DefaultGuestScopes)
	}
	for i := range c.APIClients {
		a := &c.APIClients[i]
		a.Name = strings.TrimSpace(a.Name)
		a.Rights.Visibility = strings.ToLower(strings.TrimSpace(a.Rights.Visibility))
		a.Rights.fillRouter(c.UID)
		if a.Scopes == nil {
			a.Scopes = []string{}
		}
	}
}

// uids: the UIDs of every router of the settings.
func (r Rights) validate(who string, levels int, uids []string, bad func(string, ...any)) {
	if r.Visibility != "" && r.Visibility != VisReadOnly && r.Visibility != VisHidden {
		bad("%s: visibility must be %q or %q", who, VisReadOnly, VisHidden)
	}
	for _, l := range []*[]Ref{r.Sources, r.Dsts} {
		if l != nil && slices.ContainsFunc(*l, func(f Ref) bool { return f.N < 0 }) {
			bad("%s: sources and destinations cannot be negative", who)
		}
		if l != nil && slices.ContainsFunc(*l, func(f Ref) bool { return f.Router != "" && !slices.Contains(uids, f.Router) }) {
			bad("%s: a source or destination names a router Sode does not have", who)
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
	if c.RequireHTTPS && len(c.TrustedProxies) == 0 {
		bad("Require HTTPS needs at least one trusted proxy: only a trusted proxy can say a request was secure")
	}
	uids := c.RouterUIDs()
	c.GuestRights.validate("Guest rights", c.Levels, uids, bad)
	checkScopes("keyless scopes", c.GuestAPIScopes, bad)
	for _, u := range c.Users {
		u.Rights.validate("rights of "+u.Name, c.Levels, uids, bad)
	}
	own := 0
	ofUser := map[string]int{}
	for _, cl := range c.APIClients {
		switch {
		case cl.Panel != "" && cl.Owner != "":
			bad("API client %q belongs to a panel and to a user", cl.Name)
		case cl.Owner != "":
			ofUser[cl.Owner]++
		case cl.Panel == "":
			own++
		}
	}
	if own > MaxAPIClients {
		bad("at most %d API clients", MaxAPIClients)
	}
	for id, n := range ofUser {
		if n > MaxAPIClients {
			bad("%s has more than %d API clients", cmp.Or(c.UserName(id), id), MaxAPIClients)
		}
	}
	seen := map[string]bool{}
	for _, cl := range c.APIClients {
		key := foldKey(cl.Name)
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
		if _, ok := c.FindUserByName(cl.Name); ok {
			bad("API client %q has the name of a user; History would not tell them apart", cl.Name)
		}
		seen[key] = true
		cl.Rights.validate("rights of API client "+cl.Name, c.Levels, uids, bad)
		checkScopes("API client "+cl.Name, cl.Scopes, bad)
	}
	return errs
}

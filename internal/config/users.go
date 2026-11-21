package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Len is the PIN length, so the lock screen can tell a partly typed PIN from a wrong full-length one.
type User struct {
	// ID is hidden and never changes, so whatever stores a user survives a rename. Normalize fills a missing one.
	ID   string `yaml:"id" json:"id"`
	Name string `yaml:"name" json:"name"`
	// Pin is "" (and Len 0) for a user without a PIN, who acts no wider than a Guest (ActsAs).
	Pin string `yaml:"pin,omitempty" json:"-"`
	Len int    `yaml:"len,omitempty" json:"len"`
	// PinNeeded: the Role is kept for when the user sets a PIN; the only way above Standard user with no PIN.
	PinNeeded bool `yaml:"pinNeeded,omitempty" json:"pinNeeded,omitempty"`
	// Role: RoleAdministrator, RoleTrusted or RoleStandard (policy: app/policy.go).
	Role       string `yaml:"role" json:"role"`
	Persistent bool   `yaml:"persistent,omitempty" json:"persistent,omitempty"`
	// NoAutoLock: the idle timer skips this user; the Lock button still works.
	NoAutoLock bool `yaml:"noAutoLock,omitempty" json:"noAutoLock,omitempty"`
	// Rights and its Visibility bind this user's browser and keys.
	Rights Rights `yaml:"rights,omitempty" json:"rights"`
	// Profile is never in JSON: sent only to connections signed in as this user.
	Profile Profile        `yaml:"profile,omitempty" json:"-"`
	Extra   map[string]any `yaml:",inline" json:"-"`
}

type Locks struct {
	Main bool `yaml:"main" json:"main"`
	Log  bool `yaml:"log" json:"log"`
	// Panel covers the three panel pages: /panels, /panels/editor and /panels/<id>.
	Panel bool           `yaml:"panel" json:"panel"`
	Extra map[string]any `yaml:",inline" json:"-"`
}

func (l Locks) Any() bool { return l.Main || l.Log || l.Panel }

// Roles. Wire and file values; a missing role loads as RoleStandard.
const (
	RoleStandard      = "standard"
	RoleTrusted       = "trusted"
	RoleAdministrator = "administrator"
)

func roleRank(r string) int {
	switch r {
	case RoleStandard:
		return 1
	case RoleTrusted:
		return 2
	case RoleAdministrator:
		return 3
	}
	return 0
}

// RoleAtLeast: an unknown or empty have is below every role.
func RoleAtLeast(have, need string) bool {
	return roleRank(have) > 0 && roleRank(have) >= roleRank(need)
}

func RoleLabel(r string) string {
	switch r {
	case RoleAdministrator:
		return "Administrator"
	case RoleTrusted:
		return "Trusted user"
	}
	return "Standard user"
}

func (u User) HasPin() bool { return u.Pin != "" }

// A user without a PIN acts as Standard user or Guest role, whichever is higher, with no more than Guest Rights.
// The Protect right comes back set, from the Role when the user's Rights leave it unset.
func (c Config) ActsAs(u User) (string, Rights) {
	if u.HasPin() {
		return u.Role, u.Rights.ProtectFor(u.Role)
	}
	role := RoleStandard
	if roleRank(c.GuestRole) > roleRank(role) && c.GuestRole != RoleAdministrator {
		role = c.GuestRole
	}
	_, guest := c.GuestActs()
	return role, u.Rights.ProtectFor(role).Intersect(guest)
}

// ActsWithGuest: what u acts with on a page open to guests. Signing in never gives less than a guest has there, so the
// Role is the higher of the two and the Rights are the union. Approvals and PIN rechecks use ActsAs alone.
func (c Config) ActsWithGuest(u User) (string, Rights) {
	role, rt := c.ActsAs(u)
	grole, grt := c.GuestActs()
	if roleRank(grole) > roleRank(role) {
		role = grole
	}
	return role, rt.Union(grt)
}

// GuestActs: the Role and Rights of a guest. An unset Protect right follows the Guest role, as a user's follows
// theirs. With no users every browser is an Administrator.
func (c Config) GuestActs() (string, Rights) {
	role := c.GuestRole
	if len(c.Users) == 0 {
		role = RoleAdministrator
	}
	return role, c.GuestRights.ProtectFor(role)
}

// NoPinRefusal names a user above Standard user left without a PIN; PIN needed users are allowed.
func NoPinRefusal(u User) error {
	if u.HasPin() || u.PinNeeded || !RoleAtLeast(u.Role, RoleTrusted) {
		return nil
	}
	return fmt.Errorf("%s is %s and needs a PIN", u.Name, withArticle(RoleLabel(u.Role)))
}

func withArticle(label string) string {
	if strings.HasPrefix(label, "A") {
		return "an " + label
	}
	return "a " + label
}

// HasAdministrator: at least one Administrator with a PIN, which every save keeps (app's update).
func (c Config) HasAdministrator() bool {
	for _, u := range c.Users {
		if u.Role == RoleAdministrator && ValidPinHash(u.Pin) {
			return true
		}
	}
	return false
}

const (
	MaxUsers         = 12
	MaxUserName      = 24
	UserPinMin       = 4
	UserPinMax       = 12
	DefaultMaxPinLen = 8
	MaxAutoLockMin   = 24 * 60
)

// A max of 0 means UserPinMax.
func CheckUserPinRules(pin string, max int) error {
	if max <= 0 {
		max = UserPinMax
	}
	if n := len(pin); n < UserPinMin || n > max || strings.Trim(pin, "0123456789") != "" {
		return fmt.Errorf("a user PIN must be %d-%d digits", UserPinMin, max)
	}
	return nil
}

// FindUserByName is for a name a person typed; anything stored refers to a user by id.
func (c Config) FindUserByName(name string) (User, bool) {
	if name == "" {
		return User{}, false
	}
	for _, u := range c.Users {
		if strings.EqualFold(u.Name, name) {
			return u, true
		}
	}
	return User{}, false
}

// FindUserByID: "" matches nobody.
func (c Config) FindUserByID(id string) (User, bool) {
	if id == "" {
		return User{}, false
	}
	for _, u := range c.Users {
		if u.ID == id {
			return u, true
		}
	}
	return User{}, false
}

func (c Config) UserName(id string) string {
	u, _ := c.FindUserByID(id)
	return u.Name
}

func NewUserID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "u-" + hex.EncodeToString(b)
}

func (c *Config) normalizeUsers() {
	// An older file has no maxPinLen: allow what its users already have.
	if c.MaxPinLen == 0 {
		c.MaxPinLen = DefaultMaxPinLen
		for _, u := range c.Users {
			c.MaxPinLen = max(c.MaxPinLen, u.Len)
		}
	}
	if c.GuestRole == "" {
		c.GuestRole = RoleStandard
	}
	for i := range c.Users {
		c.Users[i].Name = strings.TrimSpace(c.Users[i].Name)
		if c.Users[i].Role == "" {
			c.Users[i].Role = RoleStandard
		}
		if c.Users[i].HasPin() {
			c.Users[i].PinNeeded = false
		}
		if c.Users[i].ID == "" {
			c.Users[i].ID = NewUserID()
			c.idsFilled = true
		}
	}
}

// Two names share a key exactly when strings.EqualFold matches them, as every lookup does.
func foldKey(s string) string {
	return strings.Map(func(r rune) rune {
		m := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			m = min(m, f)
		}
		return m
	}, s)
}

func (c Config) validateUsers() []error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if len(c.Users) > MaxUsers {
		bad("at most %d users", MaxUsers)
	}
	seen := map[string]bool{}
	seenID := map[string]bool{}
	for _, u := range c.Users {
		if u.ID == "" || seenID[u.ID] {
			bad("user %q needs an id of its own", u.Name)
		}
		seenID[u.ID] = true
		key := foldKey(u.Name)
		switch n := utf8.RuneCountInString(u.Name); {
		case n == 0:
			bad("every user needs a name")
		case n > MaxUserName:
			bad("user name %q must be at most %d characters", u.Name, MaxUserName)
		case seen[key]:
			bad("two users are named %q", u.Name)
		}
		seen[key] = true
		if strings.IndexFunc(u.Name, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
			bad("user name %q must use printable characters only", u.Name)
		}
		if roleRank(u.Role) == 0 {
			bad("user %q has an unknown role %q", u.Name, u.Role)
		}
		switch {
		case u.Pin == "" && u.Len == 0:
			if err := NoPinRefusal(u); err != nil {
				bad("%v", err)
			}
		case !ValidPinHash(u.Pin) || u.Len < UserPinMin || u.Len > c.MaxPinLen:
			bad("user %q has no valid PIN", u.Name)
		}
	}
	if c.Locks.Any() && len(c.Users) == 0 {
		bad("add a user before turning a lock on")
	}
	if c.GuestRole != RoleStandard && c.GuestRole != RoleTrusted {
		bad("Guest role must be %q or %q", RoleStandard, RoleTrusted)
	}
	if c.MaxPinLen < UserPinMin || c.MaxPinLen > UserPinMax {
		bad("longest user PIN must be %d-%d digits", UserPinMin, UserPinMax)
	}
	if c.AutoLockMinutes < 0 || c.AutoLockMinutes > MaxAutoLockMin {
		bad("auto-lock must be 0 (never) or 1-%d minutes", MaxAutoLockMin)
	}
	return errs
}

func (c Config) PinLens() []int {
	var out []int
	for _, u := range c.Users {
		if u.Len > 0 && !slices.Contains(out, u.Len) {
			out = append(out, u.Len)
		}
	}
	slices.Sort(out)
	return out
}

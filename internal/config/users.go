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
	Pin  string `yaml:"pin" json:"-"`
	Len  int    `yaml:"len" json:"len"`
	// Trusted: low-risk saves need no Advanced Settings PIN (policy: app/policy.go).
	Trusted    bool `yaml:"trusted,omitempty" json:"trusted,omitempty"`
	Persistent bool `yaml:"persistent,omitempty" json:"persistent,omitempty"`
	// NoAutoLock: the idle timer skips this user; the Lock button still works.
	NoAutoLock bool `yaml:"noAutoLock,omitempty" json:"noAutoLock,omitempty"`
	// Rights and its Visibility bind this user's browser and keys.
	Rights Rights `yaml:"rights,omitempty" json:"rights"`
	// Profile is never in JSON: sent only to connections unlocked as this user.
	Profile Profile        `yaml:"profile,omitempty" json:"-"`
	Extra   map[string]any `yaml:",inline" json:"-"`
}

type Locks struct {
	Main bool `yaml:"main" json:"main"`
	Log  bool `yaml:"log" json:"log"`
	// Panel covers the three panel pages: /panels, /panels/editor and /panels/<id>.
	Panel bool `yaml:"panel" json:"panel"`
	// Grid: ignored; kept so old files load.
	LegacyGrid any            `yaml:"grid,omitempty" json:"-"`
	Extra      map[string]any `yaml:",inline" json:"-"`
}

func (l Locks) Any() bool { return l.Main || l.Log || l.Panel }

// Sign-in modes. Wire and file values.
const (
	SignInPin      = "pin"      // the PIN alone says who; every PIN is unique
	SignInUsername = "username" // the user's Name and PIN together; PINs can repeat
)

const (
	MaxUsers         = 12 // a PIN is checked against each user of its length
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

// UserName is the live name of user id, or "" when there is no such user.
func (c Config) UserName(id string) string {
	u, _ := c.FindUserByID(id)
	return u.Name
}

func NewUserID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "u-" + hex.EncodeToString(b)
}

// UserIDsFilled: Normalize gave a user an id the file lacked. The caller must save before
// anything refers to it, or the next start makes new ones.
func (c Config) UserIDsFilled() bool { return c.idsFilled }

// KeepPins is c with PIN hashes it lacks copied from live (a user's hash and length together).
// Hashes must look like hashes so a file cannot stop the next start; a user with neither is an error.
func (c Config) KeepPins(live Config) (Config, error) {
	if c.SettingsPin == "" {
		c.SettingsPin = live.SettingsPin
	} else if !ValidPinHash(c.SettingsPin) {
		return c, fmt.Errorf("settingsPin in the file is not a PIN hash. A PIN cannot be typed into a file. Set it in Settings")
	}
	users := make([]User, len(c.Users))
	for i, u := range c.Users {
		if u.Pin != "" {
			if !ValidPinHash(u.Pin) {
				return c, fmt.Errorf("the PIN of user %q in the file is not a PIN hash", u.Name)
			}
		} else {
			keep, ok := live.FindUserByID(u.ID)
			if !ok {
				keep, ok = live.FindUserByName(u.Name)
			}
			if !ok || keep.Pin == "" {
				return c, fmt.Errorf("user %q has no PIN in the file and no PIN here to keep. Export with PINs included, or add the user in Settings first", u.Name)
			}
			u.Pin, u.Len = keep.Pin, keep.Len
		}
		users[i] = u
	}
	c.Users = users
	return c, nil
}

func (c *Config) normalizeUsers() {
	// An older file has no maxPinLen: allow what its users already have.
	if c.MaxPinLen == 0 {
		c.MaxPinLen = DefaultMaxPinLen
		for _, u := range c.Users {
			c.MaxPinLen = max(c.MaxPinLen, u.Len)
		}
	}
	if c.SignInMode == "" {
		c.SignInMode = SignInPin
	}
	for i := range c.Users {
		c.Users[i].Name = strings.TrimSpace(c.Users[i].Name)
		if c.Users[i].ID == "" {
			c.Users[i].ID = NewUserID()
			c.idsFilled = true
		}
	}
}

func (c Config) validateUsers() []error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if len(c.Users) > MaxUsers {
		bad("at most %d users", MaxUsers)
	}
	if c.SignInMode != SignInPin && c.SignInMode != SignInUsername {
		bad("sign-in mode must be %q or %q", SignInPin, SignInUsername)
	}
	seen := map[string]bool{}
	seenID := map[string]bool{}
	for _, u := range c.Users {
		if u.ID == "" || seenID[u.ID] {
			bad("user %q needs an id of its own", u.Name)
		}
		seenID[u.ID] = true
		key := strings.ToLower(u.Name)
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
		if !ValidPinHash(u.Pin) || u.Len < UserPinMin || u.Len > c.MaxPinLen {
			bad("user %q has no valid PIN", u.Name)
		}
	}
	if c.Locks.Any() && len(c.Users) == 0 {
		bad("add a user before turning a lock on")
	}
	if c.Locks.Any() && c.SettingsPin == "" {
		bad("set an Advanced Settings PIN before turning a lock on: users and locks are administered with it")
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

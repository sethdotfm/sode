package config

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Len is the PIN length, so the lock screen can tell a partly typed PIN from a wrong full-length one.
type User struct {
	Name string `yaml:"name" json:"name"`
	// Username signs in under SignInUsername; separate from Name (what History shows). Never
	// sent before the PIN: the hello blanks Users and lockState has no field for it.
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Pin      string `yaml:"pin" json:"-"`
	Len      int    `yaml:"len" json:"len"`
	// Trusted: low-risk saves need no settings PIN (policy: app/policy.go).
	Trusted    bool `yaml:"trusted,omitempty" json:"trusted,omitempty"`
	Persistent bool `yaml:"persistent,omitempty" json:"persistent,omitempty"`
	// Profile is never in JSON: sent only to connections unlocked as this user.
	Profile Profile `yaml:"profile,omitempty" json:"-"`
}

type Locks struct {
	Main bool `yaml:"main" json:"main"`
	Log  bool `yaml:"log" json:"log"`
	// Grid: ignored; kept so old files load.
	LegacyGrid any `yaml:"grid,omitempty" json:"-"`
}

func (l Locks) Any() bool { return l.Main || l.Log }

// Sign-in modes. Wire and file values.
const (
	SignInPin      = "pin"      // the PIN alone says who; every PIN is unique
	SignInUsername = "username" // username and PIN together; PINs may repeat
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

func (c Config) FindUser(name string) (User, bool) {
	for _, u := range c.Users {
		if strings.EqualFold(u.Name, name) {
			return u, true
		}
	}
	return User{}, false
}

// FindUsername is the user whose Username equals username ignoring case; "" matches nobody.
func (c Config) FindUsername(username string) (User, bool) {
	if username == "" {
		return User{}, false
	}
	for _, u := range c.Users {
		if strings.EqualFold(u.Username, username) {
			return u, true
		}
	}
	return User{}, false
}

func (u *User) TakePin(from User) { u.Pin, u.Len = from.Pin, from.Len }

// KeepPins is c with PIN hashes it lacks taken from live (a user's hash and length together).
// Hashes must look like hashes so a file can't stop the next start; a user with neither is an error.
func (c Config) KeepPins(live Config) (Config, error) {
	if c.SettingsPin == "" {
		c.SettingsPin = live.SettingsPin
	} else if !ValidPinHash(c.SettingsPin) {
		return c, fmt.Errorf("settingsPin in the file isn't a PIN hash (a PIN can't be typed into a file; set it in Settings)")
	}
	users := make([]User, len(c.Users))
	for i, u := range c.Users {
		if u.Pin != "" {
			if !ValidPinHash(u.Pin) {
				return c, fmt.Errorf("the PIN of user %q in the file isn't a PIN hash", u.Name)
			}
		} else {
			keep, ok := live.FindUser(u.Name)
			if !ok || keep.Pin == "" {
				return c, fmt.Errorf("user %q has no PIN in the file and no PIN here to keep: export with PINs included, or add the user in Settings first", u.Name)
			}
			u.TakePin(keep)
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
		c.Users[i].Username = strings.TrimSpace(c.Users[i].Username)
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
	seenUsername := map[string]bool{}
	for _, u := range c.Users {
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
		switch n := utf8.RuneCountInString(u.Username); {
		case n == 0:
			if c.SignInMode == SignInUsername {
				bad("user %q needs a username", u.Name)
			}
		case n > MaxUserName:
			bad("username %q must be at most %d characters", u.Username, MaxUserName)
		case strings.IndexFunc(u.Username, func(r rune) bool { return !unicode.IsPrint(r) || unicode.IsSpace(r) }) >= 0:
			bad("username %q must use printable characters and no spaces", u.Username)
		case seenUsername[strings.ToLower(u.Username)]:
			bad("two users have the username %q", u.Username)
		}
		seenUsername[strings.ToLower(u.Username)] = true
		if !ValidPinHash(u.Pin) || u.Len < UserPinMin || u.Len > c.MaxPinLen {
			bad("user %q has no valid PIN", u.Name)
		}
	}
	if c.Locks.Any() && len(c.Users) == 0 {
		bad("add a user before turning a lock on")
	}
	if c.Locks.Any() && c.SettingsPin == "" {
		bad("set a settings PIN before turning a lock on: users and locks are administered with it")
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

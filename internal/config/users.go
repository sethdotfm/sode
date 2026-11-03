package config

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Users and page locks (#151). All off by default: no users, no locks.

// User is one person who can unlock a locked page. Pin is a HashPin hash,
// never sent to browsers. Len is the PIN's length, so the lock screen can
// tell a PIN still being typed (not counted as wrong) from a full-length
// wrong one; it is stored with the hash and leaks nothing else.
type User struct {
	Name string `yaml:"name" json:"name"`
	Pin  string `yaml:"pin" json:"-"`
	Len  int    `yaml:"len" json:"len"`
}

// Locks says which page types need an unlocked user: the main screen and /log.
type Locks struct {
	Main bool `yaml:"main" json:"main"`
	Log  bool `yaml:"log" json:"log"`
}

// Any reports whether any page type is locked.
func (l Locks) Any() bool { return l.Main || l.Log }

const (
	MaxUsers       = 12 // a PIN is checked against each user of its length
	MaxUserName    = 24
	UserPinMin     = 4
	UserPinMax     = 12
	MaxAutoLockMin = 24 * 60
)

// CheckUserPinRules says whether pin is acceptable as a user PIN: 4-12 digits.
func CheckUserPinRules(pin string) error {
	if n := len(pin); n < UserPinMin || n > UserPinMax || strings.Trim(pin, "0123456789") != "" {
		return fmt.Errorf("a user PIN must be %d-%d digits", UserPinMin, UserPinMax)
	}
	return nil
}

// FindUser returns the user called name, ignoring case.
func (c Config) FindUser(name string) (User, bool) {
	for _, u := range c.Users {
		if strings.EqualFold(u.Name, name) {
			return u, true
		}
	}
	return User{}, false
}

func (c *Config) normalizeUsers() {
	for i := range c.Users {
		c.Users[i].Name = strings.TrimSpace(c.Users[i].Name)
	}
}

// validateUsers reports problems with the user directory, locks and
// auto-lock time.
func (c Config) validateUsers() []error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if len(c.Users) > MaxUsers {
		bad("at most %d users", MaxUsers)
	}
	seen := map[string]bool{}
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
		if !ValidPinHash(u.Pin) || u.Len < UserPinMin || u.Len > UserPinMax {
			bad("user %q has no valid PIN", u.Name)
		}
	}
	if c.Locks.Any() && len(c.Users) == 0 {
		bad("add a user before turning a lock on")
	}
	if c.Locks.Any() && c.SettingsPin == "" {
		bad("set a settings PIN before turning a lock on: users and locks are administered with it")
	}
	if c.AutoLockMinutes < 0 || c.AutoLockMinutes > MaxAutoLockMin {
		bad("auto-lock must be 0 (never) or 1-%d minutes", MaxAutoLockMin)
	}
	return errs
}

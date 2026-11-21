package config

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// -make-administrator. A running server keeps its own copy until it restarts.
func MakeAdministrator(path, name, pin string) (created bool, err error) {
	c, exists, err := Load(path)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, fmt.Errorf("%s does not exist; start Sode once to make it", path)
	}
	name = strings.TrimSpace(name)
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return false, errors.New("give the user's name")
	case n > MaxUserName:
		return false, fmt.Errorf("the name is too long: at most %d characters", MaxUserName)
	case strings.IndexFunc(name, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0:
		return false, errors.New("the name has a character that cannot be shown")
	}
	if err := CheckUserPinRules(pin, c.MaxPinLen); err != nil {
		return false, err
	}
	u, found := c.FindUserByName(name)
	if !found && len(c.Users) >= MaxUsers {
		return false, fmt.Errorf("there are %d users, the limit: give the name of a user that exists", MaxUsers)
	}
	h, err := HashPin(pin)
	if err != nil {
		return false, err
	}
	if !found {
		u = User{ID: NewUserID(), Name: name}
	}
	u.Role, u.Pin, u.Len = RoleAdministrator, h, len(pin)
	if found {
		for i := range c.Users {
			if c.Users[i].ID == u.ID {
				c.Users[i] = u
			}
		}
	} else {
		c.Users = append(c.Users, u)
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		return false, err
	}
	return !found, Save(path, c)
}

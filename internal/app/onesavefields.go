package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Some keys of the one Save are not plain Config JSON keys. They are the json:"-" Guest fields,
// and one key per attribute of each user and API client (docs/agents/settings-saves.md). Keep
// the prefixes in step with ITEM_SECTIONS in web/savereviewrules.js.

// fieldSpec reads and writes one key. get's JSON is the value compared, versioned and sent back
// on a conflict.
type fieldSpec struct {
	section string
	get     func(config.Config) any
	set     func(*config.Config, json.RawMessage, *saveCtx) error
	hidden  bool   // the version rides usersList, not the hello
	secret  bool   // a conflict sends no value
	adds    bool   // adds an item: never unchanged, no version check
	action  bool   // an action such as Swap: never unchanged, no version check, its own Settings saves field
	item    string // fields of one item also validate together ("" for none)
}

// saveCtx keeps what a save makes once, so the check and the commit agree: PIN hashes (PBKDF2 is
// slow) and the ids of added users.
type saveCtx struct {
	made map[string]string
	// swap: the Swap of this save, made once in planSave with the router's names. swapped: what the last set
	// changed, per owner.
	swap    *swapper
	swapped swapCounts
}

func (x *saveCtx) once(key string, make func() (string, error)) (string, error) {
	if v, ok := x.made[key]; ok {
		return v, nil
	}
	v, err := make()
	if err != nil {
		return "", err
	}
	if x.made == nil {
		x.made = map[string]string{}
	}
	x.made[key] = v
	return v, nil
}

func decodeJSON[T any](v json.RawMessage) (T, error) {
	var out T
	err := json.Unmarshal(v, &out)
	return out, err
}

// hiddenFields: saveFields keys with json:"-".
var hiddenFields = map[string]fieldSpec{
	"guestRights": {get: func(c config.Config) any { return c.GuestRights }, set: func(c *config.Config, v json.RawMessage, _ *saveCtx) error {
		r, err := decodeJSON[config.Rights](v)
		c.GuestRights = r.WithExtraOf(c.GuestRights)
		return err
	}},
	"guestApiScopes": {get: func(c config.Config) any { return c.GuestAPIScopes }, set: func(c *config.Config, v json.RawMessage, _ *saveCtx) error {
		s, err := decodeJSON[[]string](v)
		c.GuestAPIScopes = s
		return err
	}},
}

func lookupField(key string) (fieldSpec, bool) {
	if sec, ok := saveFields[key]; ok {
		if s, ok := hiddenFields[key]; ok {
			s.section, s.hidden = sec, true
			return s, true
		}
		return fieldSpec{section: sec,
			get: func(c config.Config) any { return fieldOf(&c, key).Interface() },
			set: func(c *config.Config, v json.RawMessage, _ *saveCtx) error {
				return json.Unmarshal(v, fieldOf(c, key).Addr().Interface())
			}}, true
	}
	if key == swapKey {
		return swapSpec(), true
	}
	if rest, ok := strings.CutPrefix(key, "user/"); ok {
		id, attr, ok := strings.Cut(rest, "/")
		if !ok {
			return fieldSpec{}, false
		}
		if strings.HasPrefix(id, "new:") {
			if attr != "add" {
				return fieldSpec{}, false
			}
			return addUserSpec(key), true
		}
		return userSpec(id, attr)
	}
	if rest, ok := strings.CutPrefix(key, "client/"); ok {
		i := strings.LastIndex(rest, "/")
		if i <= 0 {
			return fieldSpec{}, false
		}
		return clientSpec(rest[:i], rest[i+1:])
	}
	return fieldSpec{}, false
}

// ---- users

const usersSection = "users"

var userAttrs = []string{"name", "role", "pin", "rights", "persistent", "noAutoLock", "remove"}

func userIndex(c *config.Config, id string) int {
	return slices.IndexFunc(c.Users, func(u config.User) bool { return u.ID == id })
}

var errUserGone = errors.New("this user was removed meanwhile: reload the user list")

func userSpec(id, attr string) (fieldSpec, bool) {
	s := fieldSpec{section: usersSection, hidden: true, item: "user/" + id}
	user := func(c config.Config) (config.User, bool) { return c.FindUserByID(id) }
	edit := func(f func(u *config.User, v json.RawMessage, x *saveCtx) error) func(*config.Config, json.RawMessage, *saveCtx) error {
		return func(c *config.Config, v json.RawMessage, x *saveCtx) error {
			i := userIndex(c, id)
			if i < 0 {
				return errUserGone
			}
			return f(&c.Users[i], v, x)
		}
	}
	attrOf := func(f func(config.User) any) func(config.Config) any {
		return func(c config.Config) any {
			if u, ok := user(c); ok {
				return f(u)
			}
			return nil
		}
	}
	switch attr {
	case "name":
		s.get = attrOf(func(u config.User) any { return u.Name })
		s.set = edit(func(u *config.User, v json.RawMessage, _ *saveCtx) error {
			n, err := decodeJSON[string](v)
			if err != nil {
				return err
			}
			u.Name = strings.TrimSpace(n)
			return userNameRefusal(u.Name)
		})
	case "role":
		s.get = attrOf(func(u config.User) any { return u.Role })
		s.set = edit(func(u *config.User, v json.RawMessage, _ *saveCtx) error {
			r, err := decodeJSON[string](v)
			// A PIN needed user keeps the flag only at a Role no higher than before.
			u.PinNeeded = u.PinNeeded && config.RoleAtLeast(u.Role, cmp.Or(r, config.RoleStandard))
			u.Role = r
			return err
		})
	case "pin":
		s.secret = true
		s.get = attrOf(func(u config.User) any { return u.Pin })
		s.set = func(c *config.Config, v json.RawMessage, x *saveCtx) error {
			i := userIndex(c, id)
			if i < 0 {
				return errUserGone
			}
			u := &c.Users[i]
			p, err := decodeJSON[string](v)
			if err != nil {
				return err
			}
			if p == "" { // removes the PIN
				u.Pin, u.Len, u.PinNeeded = "", 0, false
				return nil
			}
			h, err := hashNewPin(x, "user/"+id+"/pin", p, c.MaxPinLen)
			if err != nil {
				return err
			}
			u.Pin, u.Len, u.PinNeeded = h, len(p), false
			return nil
		}
	case "rights":
		s.get = attrOf(func(u config.User) any { return u.Rights })
		s.set = edit(func(u *config.User, v json.RawMessage, _ *saveCtx) error {
			r, err := decodeJSON[config.Rights](v)
			u.Rights = r.WithExtraOf(u.Rights)
			return err
		})
	case "persistent":
		s.get = attrOf(func(u config.User) any { return u.Persistent })
		s.set = edit(func(u *config.User, v json.RawMessage, _ *saveCtx) error { return json.Unmarshal(v, &u.Persistent) })
	case "noAutoLock":
		s.get = attrOf(func(u config.User) any { return u.NoAutoLock })
		s.set = edit(func(u *config.User, v json.RawMessage, _ *saveCtx) error { return json.Unmarshal(v, &u.NoAutoLock) })
	case "remove":
		// The value can name the new owner of the user's panels ({"owner": id});
		// removedUsersPanels checks it.
		s.get = func(c config.Config) any { _, ok := user(c); return !ok }
		s.set = func(c *config.Config, _ json.RawMessage, _ *saveCtx) error {
			if i := userIndex(c, id); i >= 0 {
				c.Users = slices.Delete(slices.Clone(c.Users), i, i+1)
			}
			if len(c.Users) == 0 && c.Locks.Any() {
				return errors.New("a screen lock is on: turn it off in Settings > Access (and save) before removing the last user")
			}
			return nil
		}
	default:
		return fieldSpec{}, false
	}
	return s, true
}

func hashNewPin(x *saveCtx, key, pin string, maxLen int) (string, error) {
	if err := config.CheckUserPinRules(pin, maxLen); err != nil {
		return "", err
	}
	return x.once(key, func() (string, error) { return config.HashPin(pin) })
}

// newUser: what an add key carries.
type newUser struct {
	Name string `json:"name"`
	Pin  string `json:"pin"`
	Role string `json:"role"`
	// Persistent and NoAutoLock: nil is the New user defaults.
	Persistent *bool          `json:"persistent"`
	NoAutoLock *bool          `json:"noAutoLock"`
	Rights     *config.Rights `json:"rights"`
}

func addUserSpec(key string) fieldSpec {
	return fieldSpec{section: usersSection, hidden: true, adds: true, item: key,
		get: func(config.Config) any { return nil },
		set: func(c *config.Config, v json.RawMessage, x *saveCtx) error {
			n, err := decodeJSON[newUser](v)
			if err != nil {
				return err
			}
			id, _ := x.once(key, func() (string, error) { return config.NewUserID(), nil })
			u := config.User{ID: id, Name: strings.TrimSpace(n.Name), Role: n.Role,
				Persistent: c.NewUserDefaults.Persistent, NoAutoLock: c.NewUserDefaults.NoAutoLock}
			if n.Persistent != nil {
				u.Persistent = *n.Persistent
			}
			if n.NoAutoLock != nil {
				u.NoAutoLock = *n.NoAutoLock
			}
			if err := userNameRefusal(u.Name); err != nil {
				return err
			}
			if n.Rights != nil {
				u.Rights = n.Rights.Clone()
			}
			if n.Pin != "" {
				if u.Pin, err = hashNewPin(x, key+"/pin", n.Pin, c.MaxPinLen); err != nil {
					return err
				}
				u.Len = len(n.Pin)
			}
			c.Users = append(slices.Clone(c.Users), u)
			return nil
		}}
}

// ---- API clients (by name, as keys name them)

var clientAttrs = []string{"scopes", "rights", "remove"}

func clientIndex(c *config.Config, name string) int {
	return slices.IndexFunc(c.APIClients, func(o config.APIClient) bool { return strings.EqualFold(o.Name, name) })
}

type newClient struct {
	Scopes []string      `json:"scopes"`
	Rights config.Rights `json:"rights"`
}

func clientSpec(name, attr string) (fieldSpec, bool) {
	s := fieldSpec{section: usersSection, hidden: true, item: "client/" + strings.ToLower(name)}
	attrOf := func(f func(config.APIClient) any) func(config.Config) any {
		return func(c config.Config) any {
			if cl, ok := c.FindAPIClient(name); ok {
				return f(cl)
			}
			return nil
		}
	}
	edit := func(f func(cl *config.APIClient, v json.RawMessage) error) func(*config.Config, json.RawMessage, *saveCtx) error {
		return func(c *config.Config, v json.RawMessage, _ *saveCtx) error {
			i := clientIndex(c, name)
			if i < 0 {
				return fmt.Errorf("the API client %s was removed meanwhile: reload the user list", name)
			}
			c.APIClients = slices.Clone(c.APIClients)
			return f(&c.APIClients[i], v)
		}
	}
	switch attr {
	case "scopes":
		s.get = attrOf(func(cl config.APIClient) any { return cl.Scopes })
		s.set = edit(func(cl *config.APIClient, v json.RawMessage) error {
			sc, err := decodeJSON[[]string](v)
			cl.Scopes = sc
			return err
		})
	case "rights":
		s.get = attrOf(func(cl config.APIClient) any { return cl.Rights })
		s.set = edit(func(cl *config.APIClient, v json.RawMessage) error {
			r, err := decodeJSON[config.Rights](v)
			cl.Rights = r.WithExtraOf(cl.Rights)
			return err
		})
	case "add":
		s.adds = true
		s.get = func(config.Config) any { return nil }
		s.set = func(c *config.Config, v json.RawMessage, _ *saveCtx) error {
			n, err := decodeJSON[newClient](v)
			if err != nil {
				return err
			}
			if clientIndex(c, name) >= 0 {
				return fmt.Errorf("there is already an API client named %s", name)
			}
			c.APIClients = append(slices.Clone(c.APIClients), config.APIClient{Name: strings.TrimSpace(name), Scopes: n.Scopes, Rights: n.Rights.Clone()})
			return nil
		}
	case "remove":
		s.get = func(c config.Config) any { _, ok := c.FindAPIClient(name); return !ok }
		s.set = func(c *config.Config, _ json.RawMessage, _ *saveCtx) error {
			i := clientIndex(c, name)
			if i < 0 {
				return nil
			}
			if o := c.APIClients[i]; o.Panel != "" || o.Owner != "" {
				return fmt.Errorf("the API client %s belongs to a panel or a user: remove it there", o.Name)
			}
			c.APIClients = slices.Delete(slices.Clone(c.APIClients), i, i+1)
			return nil
		}
	default:
		return fieldSpec{}, false
	}
	return s, true
}

// hiddenVersions: the versions usersList carries (Administrator only, after sign-in).
func hiddenVersions(cfg config.Config) map[string]string {
	out := map[string]string{}
	for k := range hiddenFields {
		out[k] = fieldVersion(cfg, k)
	}
	for _, u := range cfg.Users {
		for _, at := range userAttrs {
			k := "user/" + u.ID + "/" + at
			out[k] = fieldVersion(cfg, k)
		}
	}
	for _, cl := range cfg.APIClients {
		for _, at := range clientAttrs {
			k := "client/" + cl.Name + "/" + at
			out[k] = fieldVersion(cfg, k)
		}
	}
	return out
}

// addedResult: an add's result names the new item and the versions of its fields, so the page
// can edit it again without a reload.
func addedResult(cfg config.Config, key string, x *saveCtx) json.RawMessage {
	out := map[string]any{}
	vers := map[string]string{}
	if id, ok := x.made[key]; ok {
		out["id"] = id
		for _, at := range userAttrs {
			k := "user/" + id + "/" + at
			vers[k] = fieldVersion(cfg, k)
		}
	} else if rest, ok := strings.CutPrefix(key, "client/"); ok {
		name := rest[:strings.LastIndex(rest, "/")]
		for _, at := range clientAttrs {
			k := "client/" + name + "/" + at
			vers[k] = fieldVersion(cfg, k)
		}
	}
	out["versions"] = vers
	b, _ := json.Marshal(out)
	return b
}

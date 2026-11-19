package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// The users save applies it to a new row that does not say (internal/app/users.go).
type NewUserDefaults struct {
	Persistent bool           `yaml:"persistent" json:"persistent"`
	NoAutoLock bool           `yaml:"noAutoLock" json:"noAutoLock"`
	Extra      map[string]any `yaml:",inline" json:"-"`
}

// Each field is Critical (TestPresetFieldsAreCritical). Per-user fields are not here: a preset applies to users per ticked
// row.
type AccessValues struct {
	Locks           Locks
	AutoLockMinutes int
	GuestRole       string
	PinWaits        bool
	PublicPanels    bool
	KeylessAPI      bool
	KeylessScopes   []string
	NewPanelActs    string
	NewUserDefaults NewUserDefaults
}

// Name: the full name, since bare Standard is the Role. Nothing records which preset was applied.
type AccessPreset struct {
	Key, Name string
	Values    AccessValues
}

var AccessPresets = []AccessPreset{
	{"open", "Open access", AccessValues{
		GuestRole:       RoleTrusted,
		PublicPanels:    true,
		KeylessAPI:      true,
		KeylessScopes:   []string{ScopeNames, ScopeTally, ScopeProtected, ScopeStatus, ScopePresets, ScopeTake},
		NewPanelActs:    "public",
		NewUserDefaults: NewUserDefaults{Persistent: true, NoAutoLock: true},
	}},
	{"standard", "Standard access", AccessValues{
		GuestRole:       RoleStandard,
		PinWaits:        true,
		PublicPanels:    true,
		KeylessAPI:      true,
		KeylessScopes:   []string{ScopeNames, ScopeTally, ScopeProtected, ScopeStatus},
		NewPanelActs:    "public",
		NewUserDefaults: NewUserDefaults{Persistent: true, NoAutoLock: true},
	}},
	{"secured", "Secured access", AccessValues{
		Locks:         Locks{Main: true, Panel: true},
		GuestRole:     RoleStandard,
		PinWaits:      true,
		KeylessScopes: []string{ScopeNames, ScopeTally, ScopeProtected, ScopeStatus},
		NewPanelActs:  "session",
	}},
}

func (p AccessPreset) Apply(c *Config) {
	v := p.Values
	c.Locks = v.Locks
	c.AutoLockMinutes = v.AutoLockMinutes
	c.GuestRole = v.GuestRole
	c.PinWaits = v.PinWaits
	c.PublicPanels = v.PublicPanels
	c.API.GuestEnabled = v.KeylessAPI
	c.GuestAPIScopes = slices.Clone(v.KeylessScopes)
	c.NewPanelActs = v.NewPanelActs
	c.NewUserDefaults = NewUserDefaults{Persistent: v.NewUserDefaults.Persistent, NoAutoLock: v.NewUserDefaults.NoAutoLock, Extra: c.NewUserDefaults.Extra}
}

func AccessPresetByName(s string) (AccessPreset, error) {
	s = strings.TrimSpace(s)
	for _, p := range AccessPresets {
		if strings.EqualFold(s, p.Key) || strings.EqualFold(s, p.Name) {
			return p, nil
		}
	}
	return AccessPreset{}, fmt.Errorf("unknown access preset %q: want open, standard or secured", s)
}

// nil when unset.
func AccessPresetFromEnv() (*AccessPreset, error) {
	v := os.Getenv("SODE_ACCESS_PRESET")
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	p, err := AccessPresetByName(v)
	if err != nil {
		return nil, fmt.Errorf("SODE_ACCESS_PRESET: %w", err)
	}
	return &p, nil
}

// Page locks stay off: Validate refuses a lock with no user to open it.
func ApplyEnvOnlyPreset(c *Config) error {
	p, err := AccessPresetFromEnv()
	if p == nil {
		return err
	}
	q := *p
	q.Values.Locks = Locks{}
	q.Apply(c)
	return nil
}

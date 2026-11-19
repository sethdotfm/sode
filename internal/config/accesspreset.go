package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// NewUserDefaults: what a new user starts with. The users save applies it to a new row that
// does not say (internal/app/users.go).
type NewUserDefaults struct {
	Persistent bool           `yaml:"persistent" json:"persistent"`
	NoAutoLock bool           `yaml:"noAutoLock" json:"noAutoLock"`
	Extra      map[string]any `yaml:",inline" json:"-"`
}

// AccessValues are every field an Access preset writes. Each one is Critical
// (TestPresetFieldsAreCritical in internal/app). Guest Rights, Fire mode and existing users'
// Persistent and No auto-lock are not here: applying a preset to users is per ticked row.
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

// AccessPreset: Key is the short name (SODE_ACCESS_PRESET, the picker); Name is the full name
// docs and headings use, since bare Standard is the Role. Nothing records which was applied.
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

// Apply writes every preset field into c. Env-only mode keeps its page locks off (spec #543),
// so it puts c.Locks back after.
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

// AccessPresetByName takes the short or the full name, in any case.
func AccessPresetByName(s string) (AccessPreset, error) {
	s = strings.TrimSpace(s)
	for _, p := range AccessPresets {
		if strings.EqualFold(s, p.Key) || strings.EqualFold(s, p.Name) {
			return p, nil
		}
	}
	return AccessPreset{}, fmt.Errorf("unknown access preset %q: want open, standard or secured", s)
}

// AccessPresetFromEnv reads SODE_ACCESS_PRESET: nil when unset. The setup wizard preselects it
// and env-only mode applies it.
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

// ApplyEnvOnlyPreset applies SODE_ACCESS_PRESET in env-only mode, which starts with no users. The
// page locks stay off, since Validate refuses a lock with no user to open it.
func ApplyEnvOnlyPreset(c *Config) error {
	p, err := AccessPresetFromEnv()
	if p == nil {
		return err
	}
	p.Apply(c)
	c.Locks.Main, c.Locks.Log, c.Locks.Panel = false, false, false
	return nil
}

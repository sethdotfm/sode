package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Router holds one router's settings. JSON keys stay flat in the config message, so the
// Settings form and the hello did not change; the file nests them under routers.
type Router struct {
	// UID is made once (Normalize) and never changes, not even with the address. Never shown.
	UID string `yaml:"uid" json:"-" owned:"true"`
	// Prefix is A-Z, hidden while there is one router.
	Prefix  string `yaml:"prefix" json:"-" owned:"true"`
	Title   string `yaml:"title" json:"routerName" guard:"open"`
	Address string `yaml:"address" json:"router" guard:"router"`
	// RouterShow: what the status label shows (address, name or none).
	RouterShow      string `yaml:"statusLabel" json:"routerShow" guard:"open"`
	Matrix          int    `yaml:"matrix" json:"matrix" guard:"router"`
	Levels          int    `yaml:"levels" json:"levels" guard:"router"`
	LevelWireOffset int    `yaml:"levelOffset" json:"levelWireOffset" guard:"router"`
	// LevelNumbering: "1" (1-17), "0" (0-16) or "va" (V, 1-16).
	LevelNumbering string `yaml:"levelNumbering" json:"levelNumbering" guard:"open"`
	NumberOffset   int    `yaml:"numberOffset" json:"numberOffset" guard:"open"`
	DefaultLevels  string `yaml:"defaultLevels" json:"defaultLevels" guard:"open"`
	NameLength     int    `yaml:"nameLength" json:"nameLength" guard:"router"`
	NameRequests   string `yaml:"nameRequests" json:"nameRequests" guard:"router"`
	// NameRefreshSeconds is the router name re-read interval; 0 is off.
	NameRefreshSeconds int `yaml:"nameRereadSeconds" json:"nameRefreshSeconds" guard:"open"`
	// TallyResyncSeconds is the Re-read routes interval; 0 is off.
	TallyResyncSeconds int `yaml:"routeRereadSeconds" json:"tallyResyncSeconds" guard:"open"`
	// FireMode is Multi destination behavior. SalvoGroup (0-127): the router's list is shared by every client, so use one only Sode uses.
	FireMode   string         `yaml:"fireMode" json:"fireMode" guard:"pin"`
	SalvoGroup int            `yaml:"salvoGroup" json:"salvoGroup" guard:"pin"`
	Extra      map[string]any `yaml:",inline" json:"-" owned:"true"`
}

// Ref names one source, destination or level on one router; stores move to it from bare numbers.
// N is Sode's number: sources and destinations 0-based, levels 1..N. Shown and Name are for
// people and the remap check; import ignores Shown.
type Ref struct {
	Router string         `yaml:"router" json:"router"`
	N      int            `yaml:"n" json:"n"`
	Shown  string         `yaml:"shown,omitempty" json:"shown,omitempty"`
	Name   string         `yaml:"name,omitempty" json:"name,omitempty"`
	Extra  map[string]any `yaml:",inline" json:"-"`
}

// Shown carries no prefix: with one router it is hidden.
func (c Config) SourceRef(n int, name string) Ref {
	return Ref{Router: c.UID, N: n, Shown: strconv.Itoa(n + c.NumberOffset), Name: name}
}

func (c Config) DestinationRef(n int, name string) Ref { return c.SourceRef(n, name) }

func (c Config) LevelRef(ui int) Ref {
	return Ref{Router: c.UID, N: ui, Shown: c.LevelLabel(ui)}
}

func newRouterUID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// configFile is Config without its YAML methods.
type configFile Config

// Two steps, since yaml.v3 ignores an inline map (Extra) inside an inline struct.
func (c Config) MarshalYAML() (any, error) {
	c.Format = Format
	var top, routers yaml.Node
	if err := top.Encode(configFile(c)); err != nil {
		return nil, err
	}
	if err := routers.Encode([]Router{c.Router}); err != nil {
		return nil, err
	}
	// After format (Config's first field), so the marker stays the file's first key.
	key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "routers"}
	top.Content = slices.Insert(top.Content, 2, key, &routers)
	return &top, nil
}

func (c *Config) UnmarshalYAML(n *yaml.Node) error {
	f := configFile(*c)
	if err := n.Decode(&f); err != nil {
		return err
	}
	*c = Config(f)
	delete(c.Extra, "routers")
	if len(c.Extra) == 0 {
		c.Extra = nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value != "routers" {
			continue
		}
		list := n.Content[i+1]
		if list.Kind != yaml.SequenceNode {
			return fmt.Errorf("routers must be a list")
		}
		if len(list.Content) > 1 {
			return fmt.Errorf("routers holds %d routers; this release drives one router", len(list.Content))
		}
		// Decoded over the defaults, so a missing key keeps its default as top-level keys do.
		if len(list.Content) == 1 {
			if err := list.Content[0].Decode(&c.Router); err != nil {
				return err
			}
		}
	}
	return nil
}

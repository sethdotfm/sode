package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// JSON keys stay flat so the config message and hello keep their shape; the file nests them under routers.
type Router struct {
	// Normalize makes UID one time. It never changes, not even with the address. Never shown.
	UID string `yaml:"uid" json:"-"`
	// Prefix is A-Z, hidden while there is one router.
	Prefix  string `yaml:"prefix" json:"-"`
	Title   string `yaml:"title" json:"routerName"`
	Address string `yaml:"address" json:"router" reconnect:"true"`
	// RouterShow: what the status line shows (address, name or none).
	RouterShow      string `yaml:"statusLabel" json:"routerShow"`
	Matrix          int    `yaml:"matrix" json:"matrix" reconnect:"true"`
	Levels          int    `yaml:"levels" json:"levels" reconnect:"true"`
	LevelWireOffset int    `yaml:"levelOffset" json:"levelWireOffset" reconnect:"true"`
	// LevelNumbering: "1" (1-17), "0" (0-16) or "va" (V, 1-16).
	LevelNumbering string `yaml:"levelNumbering" json:"levelNumbering"`
	NumberOffset   int    `yaml:"numberOffset" json:"numberOffset"`
	DefaultLevels  string `yaml:"defaultLevels" json:"defaultLevels"`
	NameLength     int    `yaml:"nameLength" json:"nameLength" reconnect:"true"`
	NameRequests   string `yaml:"nameRequests" json:"nameRequests" reconnect:"true"`
	// NameRefreshSeconds: how often Sode reads the router names again, in seconds; 0 is off.
	NameRefreshSeconds int `yaml:"nameRereadSeconds" json:"nameRereadSeconds"`
	// TallyResyncSeconds is the Re-read routes interval; 0 is off.
	TallyResyncSeconds int `yaml:"routeRereadSeconds" json:"tallyResyncSeconds"`
	// SalvoGroup (0-127): every client shares the router's list, so pick one only Sode uses.
	FireMode   string `yaml:"fireMode" json:"fireMode"`
	SalvoGroup int    `yaml:"salvoGroup" json:"salvoGroup"`
	// ProtectDevice is the device number Sode sets Protects with (ADR 0012); 0 is off. Panel IDs share the
	// number space and the router checks no sender, so pick one no panel uses.
	ProtectDevice int            `yaml:"protectDevice" json:"protectDevice"`
	Extra         map[string]any `yaml:",inline" json:"-"`
}

func (r *Router) normalize() {
	r.Address = strings.TrimSpace(r.Address)
	r.Title = strings.TrimSpace(r.Title)
	if r.UID == "" {
		r.UID = newRouterUID()
	}
	r.Prefix = strings.ToUpper(strings.TrimSpace(r.Prefix))
	if r.Prefix == "" {
		r.Prefix = "A"
	}
	r.RouterShow = strings.ToLower(strings.TrimSpace(r.RouterShow))
	if r.RouterShow == "" {
		r.RouterShow = "address"
	}
	r.DefaultLevels = strings.TrimSpace(r.DefaultLevels)
	r.FireMode = strings.ToLower(strings.TrimSpace(r.FireMode))
	if r.FireMode == "" {
		r.FireMode = "sequential"
	}
	r.LevelNumbering = strings.ToLower(strings.TrimSpace(r.LevelNumbering))
	if r.LevelNumbering == "" {
		r.LevelNumbering = "1"
	}
	if r.DefaultLevels == "" && r.Levels > 0 {
		r.DefaultLevels = fmt.Sprintf("1-%d", r.Levels)
	}
}

// Routers is every router, the first one first.
func (c Config) Routers() []Router {
	return append([]Router{c.Router}, c.MoreRouters...)
}

// RouterByUID gives the router with this UID. Blank is the first router.
func (c Config) RouterByUID(uid string) (Router, bool) {
	if uid == "" {
		return c.Router, true
	}
	for _, r := range c.Routers() {
		if r.UID == uid {
			return r, true
		}
	}
	return Router{}, false
}

// N: sources and destinations 0-based, levels 1..N. Import ignores Shown. Only a request leaves Router blank
// (noRouterProblem).
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

func newRouterUID() string { return RandomHexID(8) }

// RandomHexID is n random bytes in hex. The first digit is a to f.
// YAML then never reads the id as a number, and never quotes it.
func RandomHexID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	b[0] |= 0xa0
	return hex.EncodeToString(b)
}

type configFile Config

// Two steps, since yaml.v3 ignores an inline map (Extra) inside an inline struct.
func (c Config) MarshalYAML() (any, error) {
	c.Format = Format
	var top, routers yaml.Node
	if err := top.Encode(configFile(c)); err != nil {
		return nil, err
	}
	if err := routers.Encode(c.Routers()); err != nil {
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
		// Each one is decoded over the defaults (c.Router before the first), so a missing key keeps its default as
		// top-level keys do.
		base := c.Router
		c.MoreRouters = nil
		for j, rn := range list.Content {
			r := base
			if err := rn.Decode(&r); err != nil {
				return err
			}
			if j == 0 {
				c.Router = r
			} else {
				c.MoreRouters = append(c.MoreRouters, r)
			}
		}
	}
	return nil
}

package config

// Missing is one reference of a Broken item that the target does not have (GLOSSARY.md, Broken).
// Ref is the reference as the item holds it, so its name is the one to show.
type Missing struct {
	Kind string `yaml:"kind" json:"kind"` // a Missing* kind
	Ref  Ref    `yaml:"ref" json:"ref"`
	// Now: filter-set exact terms only. The name the router put at Ref's number, for the Replace term offer.
	Now   string         `yaml:"now,omitempty" json:"now,omitempty"`
	Extra map[string]any `yaml:",inline" json:"-"`
}

const (
	MissingRouter      = "router"
	MissingSource      = "source"
	MissingDestination = "destination"
	MissingLevel       = "level"
	// MissingPreset: a panel's preset button whose preset the import skipped; Ref holds its name only.
	MissingPreset = "preset"
)

func NewPresetID() string { return newRouterUID() }

func NewFilterSetID() string { return newRouterUID() }

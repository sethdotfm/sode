package config

// Missing is one reference of a Broken item that the target does not have (GLOSSARY.md, Broken).
// Ref is the reference as the item holds it, so its name is the one to show.
type Missing struct {
	Kind string `yaml:"kind" json:"kind"` // a Missing* kind
	Ref  Ref    `yaml:"ref" json:"ref"`
	// Now: a filter set's exact term only. The name that the router put at the number where Ref's name was, when
	// a name read saw that rename. The filter editor's Replace term offers it.
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

// NewPresetID is a fresh hidden preset id, of the form normalizePresets gives.
func NewPresetID() string { return newRouterUID() }

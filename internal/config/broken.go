package config

// Missing is one reference of a Broken item that the target does not have (GLOSSARY.md, Broken).
// Ref is the reference as the item holds it, so its name is the one to show.
type Missing struct {
	Kind string `yaml:"kind" json:"kind"` // a Missing* kind
	Ref  Ref    `yaml:"ref" json:"ref"`
	// Now: the name the router put at Ref's number. A filter set's exact term has it for the Replace term offer, and a
	// tie line's Name moved mark to show the name now.
	Now string `yaml:"now,omitempty" json:"now,omitempty"`
	// Tie: the id of a tie line that a preset stores and Sode no longer has (MissingTie). Ref is empty then.
	Tie   string         `yaml:"tie,omitempty" json:"tie,omitempty"`
	Extra map[string]any `yaml:",inline" json:"-"`
}

const (
	MissingRouter      = "router"
	MissingSource      = "source"
	MissingDestination = "destination"
	MissingLevel       = "level"
	// MissingPreset: a panel's preset button whose preset the import skipped; Ref holds its name only.
	MissingPreset = "preset"
	// MissingTie: a preset's route across routers names a tie line that Sode does not have (docs/tielines-design.md,
	// Presets, rule 5). Normalize sets it from the tie lines, so it follows each tie line save and delete.
	MissingTie = "tie"
)

func NewPresetID() string { return newRouterUID() }

func NewFilterSetID() string { return newRouterUID() }

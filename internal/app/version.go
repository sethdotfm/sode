package app

// Version is the release number shown at the bottom of Settings. Bump it
// with every release, together with the "Version:" line in README.md;
// TestVersionNoted fails when the two disagree, and when SODE_RELEASE_TAG
// (say v0.9) names another version.
const (
	Stage   = "Beta"
	Version = "0.9"
)

// VersionLabel is how the version reads in the UI.
func VersionLabel() string { return Stage + " " + Version }

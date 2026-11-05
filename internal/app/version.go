package app

// Version is the release number shown at the bottom of Settings. Bump it
// with every release, together with the "Version:" line in README.md;
// TestVersionNoted fails when the two disagree, and when SODE_RELEASE_TAG
// (say v1.0) names another version.
const (
	Stage   = "Beta"
	Version = "1.5.0"
)

// Commit is the git commit the binary was built from. The image build sets it
// with -ldflags -X (see the Dockerfile); a plain `go build` leaves it empty.
// An rc and the release promoted from it are the same image, so this is what
// tells a bug report which build it came from.
var Commit = ""

// VersionLabel is how the version reads in the UI.
func VersionLabel() string { return Stage + " " + Version }

// BuildLabel is VersionLabel plus the short commit, when known.
func BuildLabel() string {
	if len(Commit) >= 7 {
		return VersionLabel() + " (" + Commit[:7] + ")"
	}
	return VersionLabel()
}

package app

// Version: bump every release with README's "Version:" line; TestVersionNoted checks both
// and SODE_RELEASE_TAG.
const (
	Stage   = "Beta"
	Version = "1.7.0"
)

// Commit: set by the image build via -ldflags -X; empty for plain go build. An rc and its
// promoted release are the same image, so this identifies the build.
var Commit = ""

func VersionLabel() string { return Stage + " " + Version }

func BuildLabel() string {
	if len(Commit) >= 7 {
		return VersionLabel() + " (" + Commit[:7] + ")"
	}
	return VersionLabel()
}

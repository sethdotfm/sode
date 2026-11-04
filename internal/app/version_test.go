package app

import (
	"os"
	"regexp"
	"testing"
)

// The version lives in version.go and is noted in README.md. A release
// that bumps only one of them fails here, and with SODE_RELEASE_TAG set
// (the tag about to be pushed, e.g. v0.9) so does a tag that names a
// version the code doesn't carry.
func TestVersionNoted(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`).MatchString(Version) {
		t.Fatalf("Version %q is not like 0.9 or 1.0.1", Version)
	}
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Skipf("no README.md to compare: %v", err)
	}
	want := "**Version:** " + VersionLabel()
	if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(want) + `$`).Match(readme) {
		t.Errorf("README.md needs the line %q (bump it with version.go)", want)
	}
	// CHANGELOG.md has a section for this version, and a release is no longer
	// "(unreleased)".
	if log, err := os.ReadFile("../../CHANGELOG.md"); err == nil {
		heading := "## " + VersionLabel()
		m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(heading) + `( \(unreleased\))?$`).FindSubmatch(log)
		switch {
		case m == nil:
			t.Errorf("CHANGELOG.md needs a %q section (or %q)", heading, heading+" (unreleased)")
		case len(m[1]) > 0 && os.Getenv("SODE_RELEASE_TAG") != "":
			t.Errorf("CHANGELOG.md still says %q: a release is no longer unreleased", heading+" (unreleased)")
		}
	}
	if tag := os.Getenv("SODE_RELEASE_TAG"); tag != "" && tag != "v"+Version {
		t.Errorf("release tag %q doesn't match version.go (v%s): bump Version first", tag, Version)
	}
}

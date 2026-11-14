package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The version lives in version.go and is noted in README.md. A release
// that bumps only one of them fails here, and with SODE_RELEASE_TAG set
// (the tag about to be pushed: v1.5.0, or a candidate v1.5.0-rc.1) so does a
// tag that names a version the code doesn't carry. A candidate and the
// release promoted from it are the same commit, so both refuse an
// "(unreleased)" heading.
var releaseTag = regexp.MustCompile(`^v\d+\.\d+\.\d+(-rc\.\d+)?$`)

func TestVersionNoted(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Version) {
		t.Fatalf("Version %q is not MAJOR.MINOR.PATCH (like 1.5.0)", Version)
	}
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Skipf("no README.md to compare: %v", err)
	}
	want := "**Version:** " + VersionLabel()
	if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(want) + `$`).Match(readme) {
		t.Errorf("README.md needs the line %q (bump it with version.go)", want)
	}
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
	if tag := os.Getenv("SODE_RELEASE_TAG"); tag != "" {
		if !releaseTag.MatchString(tag) {
			t.Errorf("release tag %q is not vX.Y.Z or vX.Y.Z-rc.N", tag)
		} else if rc := regexp.MustCompile(`-rc\.\d+$`).ReplaceAllString(tag, ""); rc != "v"+Version {
			t.Errorf("release tag %q doesn't match version.go (v%s): bump Version first", tag, Version)
		}
	}
}

// Each PR with a user-visible change adds its own changelog.d fragment, so
// parallel branches never edit the same CHANGELOG.md lines. A release folds
// them into the new heading and removes them.
func TestChangelogFragments(t *testing.T) {
	files, err := filepath.Glob("../../changelog.d/*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		name := filepath.Base(f)
		if name == "README.md" {
			continue
		}
		if os.Getenv("SODE_RELEASE_TAG") != "" {
			t.Errorf("changelog.d/%s: fold changelog.d into CHANGELOG.md before tagging", name)
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(body)) == "" {
			t.Errorf("changelog.d/%s is empty", name)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "- ") {
				t.Errorf("changelog.d/%s:%d must start with \"- \": %q", name, i+1, line)
			}
		}
	}
}

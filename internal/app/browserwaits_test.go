package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A wait on the Save button being disabled passes at once: Save disables it before the PUT returns.
// The pattern skips the `!` form, which waits for the button to be enabled before a click.
var saveDisabledWait = regexp.MustCompile(`(^|[^!])document\.getElementById\('ed-save'\)\.disabled`)

func TestBrowserChecksWaitForSaved(t *testing.T) {
	files, err := filepath.Glob("*browser*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no browser check files found: %v", err)
	}
	for _, f := range files {
		if f == "browserwaits_test.go" {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, ".waitFor(") && saveDisabledWait.MatchString(line) {
				t.Errorf("%s:%d waits on the Save button's disabled state, which is true before the save reply arrives; use b.waitSaved(what) (browser_test.go)", f, i+1)
			}
		}
	}
}

func TestSaveDisabledWaitPattern(t *testing.T) {
	for line, want := range map[string]bool{
		"b.waitFor(\"x\", `document.getElementById('ed-save').disabled`)":                 true,
		"b.waitFor(\"x\", `a && document.getElementById('ed-save').disabled && b`)":       true,
		"b.waitFor(\"x\", `!document.getElementById('ed-save').disabled`)":                false,
		"b.waitFor(\"x\", `document.getElementById('ed-state').textContent === 'Saved'`)": false,
	} {
		if got := saveDisabledWait.MatchString(line); got != want {
			t.Errorf("%s: got %v, want %v", line, got, want)
		}
	}
}

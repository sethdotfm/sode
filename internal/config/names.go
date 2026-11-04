package config

import (
	"fmt"
	"strings"
)

// ValidateNames is validateNames for callers outside the package (the export
// document's lists); same rules, same messages.
func ValidateNames(kind string, names []string, max int) []error {
	return validateNames(kind, names, max)
}

// validateNames checks the names of a list of things the config holds by
// name (kind is singular, e.g. "preset"): at most max of them, each named, at
// most 32 characters, no two alike ignoring case. The one place for these
// rules, for filter sets and presets.
func validateNames(kind string, names []string, max int) []error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if len(names) > max {
		bad("at most %d %ss", max, kind)
	}
	seen := map[string]bool{}
	for _, name := range names {
		key := strings.ToLower(name)
		switch n := len([]rune(name)); {
		case n == 0:
			bad("every %s needs a name", kind)
		case n > 32:
			bad("%s name %q must be at most 32 characters", kind, name)
		case seen[key]:
			bad("two %ss are named %q", kind, name)
		}
		seen[key] = true
	}
	return errs
}

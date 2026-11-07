package config

import (
	"fmt"
	"strings"
)

// validateNames: at most max, each named, 32 runes, unique ignoring case. kind is singular.
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

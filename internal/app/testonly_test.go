package app

import (
	"errors"

	"github.com/sethdotfm/sode/internal/config"
)

// Helpers only tests call.

func (a *App) update(mutate func(*config.Config) error) (old, nw *runtime, err error) {
	return a.updateWith(mutate, true)
}

func isRoleErr(err error) bool {
	var r roleErr
	return errors.As(err, &r)
}

func fixedNs(v any) []int {
	l, _ := v.([]config.Ref)
	out := make([]int, len(l))
	for i, f := range l {
		out[i] = f.N
	}
	return out
}

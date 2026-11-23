package app

import (
	"errors"
	"os"

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

func (s *namesStore) remove(uid string) error {
	path := s.path(uid)
	if path == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.held, uid)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

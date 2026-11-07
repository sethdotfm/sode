package app

import (
	"errors"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestCommit(t *testing.T) {
	a := newTestApp(t)
	called := false
	err := a.commit(func(cfg *config.Config) error {
		cfg.FilterSets = []config.FilterSet{{Name: "x"}}
		return nil
	}, func(old, nr *runtime) {
		called = true
		if len(old.cfg.FilterSets) != 0 || len(nr.cfg.FilterSets) != 1 {
			t.Errorf("after saw old=%d new=%d sets", len(old.cfg.FilterSets), len(nr.cfg.FilterSets))
		}
		if a.r() != nr {
			t.Error("runtime not stored before after")
		}
		if a.mu.TryLock() {
			t.Error("after ran without mu")
		}
	})
	if err != nil || !called || len(a.r().cfg.FilterSets) != 1 {
		t.Fatalf("commit: err=%v called=%v", err, called)
	}

	want := errors.New("no")
	called = false
	err = a.commit(func(*config.Config) error { return want }, func(_, _ *runtime) { called = true })
	if err != want || called || len(a.r().cfg.FilterSets) != 1 {
		t.Fatalf("failed commit: err=%v called=%v", err, called)
	}
}

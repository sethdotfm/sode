package app

import (
	"errors"
	"io/fs"
	"os"

	"github.com/sethdotfm/sode/internal/config"
)

// setupEnv: the wizard fields SODE_* filled, by field, each with its variable. Only on first start:
// cmd/sode reads SODE_* only when the config file does not exist, and Finish writes it.
func (a *App) setupEnv() map[string]string {
	if _, err := os.Stat(a.cfgPath); !errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if env := config.WizardEnv(); len(env) > 0 {
		return env
	}
	return nil
}

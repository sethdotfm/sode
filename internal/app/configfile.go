package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// Server config export/import (Settings > Access). The file is a plain config.yaml built
// here, since PIN hashes and profiles aren't in the browser's JSON; a private backup is
// marked only by a top comment and the file name, so any export is a drop-in config.yaml.
// All three messages need the Advanced Settings PIN (needAdmin): the file holds everything it
// guards, and with PINs, the hashes.

// maxConfigFile: a full config is ~300 KiB; leaves room for profiles. Only the read after
// configImportBegin may be this large.
const maxConfigFile = 2 << 20

const importSlack = 64 << 10

var errImportTooBig = fmt.Errorf("the file is over %d MiB; this isn't a Sode configuration", maxConfigFile>>20)

// importReadLimit: twice the size plus slack (JSON escaping), capped by maxConfigFile. ok
// is false for an impossible size.
func importReadLimit(size int) (limit int64, ok bool) {
	if size <= 0 || size > maxConfigFile {
		return 0, false
	}
	return int64(size)*2 + importSlack, true
}

// readBudget: wsReadLimit, widened for exactly one message by configImportBegin (the
// library applies the limit as each message starts); Reset after every read.
type readBudget struct {
	set     func(int64)
	widened bool
}

func newReadBudget(set func(int64)) *readBudget {
	set(wsReadLimit)
	return &readBudget{set: set}
}

func (b *readBudget) Grant(size int) bool {
	n, ok := importReadLimit(size)
	if ok {
		b.set(n)
		b.widened = true
	}
	return ok
}

func (b *readBudget) Reset() {
	if b.widened {
		b.set(wsReadLimit)
		b.widened = false
	}
}

func exportName(cfg config.Config) string {
	if s := strings.TrimSpace(cfg.BrandText); s != "" {
		return s
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "Sode"
}

// ExportConfig: without pins, the Advanced Settings PIN and users' PIN hashes are left out. The PIN
// is re-checked here under cfgMu (the handler runs in a goroutine): a lock or PIN change
// since admit must refuse it.
func (a *App) ExportConfig(c *client, pins bool) (site, text string, err error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, false); err != nil {
		return "", "", err
	}
	cfg := a.r().cfg.Clone()
	head := "# Sode server configuration, exported " + time.Now().Format("2006-01-02 15:04") + ".\n"
	if pins {
		head += "# PRIVATE BACKUP: this file holds the Advanced Settings PIN and every user's PIN hash. Keep it secret.\n"
	} else {
		cfg.SettingsPin = ""
		for i := range cfg.Users {
			cfg.Users[i].Pin = ""
			cfg.Users[i].PinTag = "" // equal tags would show who shares a PIN
		}
		head += "# No PINs in this file. Importing it keeps each user's current PIN and the Advanced Settings PIN.\n"
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return "", "", err
	}
	return exportName(cfg), head + string(b), nil
}

// ImportConfig replaces the whole config (no KeepOwned: everything comes from the file
// except PIN hashes it omits). Refused without the Advanced Settings PIN or for a file that doesn't
// parse or validate. Router changes reconnect via applyConfig.
func (a *App) ImportConfig(c *client, text string) error {
	if len(text) > maxConfigFile {
		return errImportTooBig
	}
	if blank(text) {
		return errors.New("the file is empty")
	}
	file, err := config.Parse([]byte(text))
	if err != nil {
		return fmt.Errorf("not a Sode configuration: %w", err)
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, false); err != nil {
		return err
	}
	return a.applyConfig(func(cur *config.Config) error {
		m, err := file.KeepPins(*cur)
		if err != nil {
			return err
		}
		*cur = m
		return nil
	})
}

// blank: only whitespace and comments (would load as defaults, wiping the config).
func blank(text string) bool {
	for _, line := range bytes.Split([]byte(text), []byte("\n")) {
		if l := bytes.TrimSpace(line); len(l) > 0 && l[0] != '#' && string(l) != "---" {
			return false
		}
	}
	return true
}

type configExportResult struct {
	T    string `json:"t"`
	ID   *int   `json:"id,omitempty"`
	OK   bool   `json:"ok"`
	Err  string `json:"err,omitempty"`
	Name string `json:"name,omitempty"` // the site: brand text, else host name
	YAML string `json:"yaml,omitempty"`
}

// Logs whether PINs were included, never the contents.
func (a *App) serveConfigExport(c *client, m *inMsg) {
	id := m.ID
	site, text, err := a.ExportConfig(c, m.Pins)
	if err != nil {
		a.replyOp(c, "configExport", id, 0, "", err)
		return
	}
	note := "without PINs"
	if m.Pins {
		note = "with PINs"
	}
	a.command(c, "configExport", "ok", note, nil)
	a.reply(c, configExportResult{T: "configExportResult", ID: &id, OK: true, Name: site, YAML: text})
}

func (a *App) serveConfigImport(c *client, m *inMsg) {
	go func(text string, id int) {
		a.replyOpNote(c, "configImport", id, fmt.Sprintf("%d bytes", len(text)), a.ImportConfig(c, text))
	}(m.YAML, m.ID)
}

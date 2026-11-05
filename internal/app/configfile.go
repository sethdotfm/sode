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

// Server configuration export and import (Settings > Access). The file is a
// plain config.yaml, made and read here: the PIN hashes and users' profiles
// are not in the JSON the browsers see, so the browser could not build it.
// A private backup is only marked by a comment at the top and the file name;
// there is no extra key, so any export is also a drop-in config.yaml.
//
// All three messages are settings-PIN only (needAdmin): the file holds
// everything the PIN guards (users, locks, router, profiles), and with PINs
// included it holds the hashes too.

// maxConfigFile is the most one imported file may be (a config with every
// owned list full is some 300 KiB; this leaves room for profiles). Only the
// one read after configImportBegin may be this large (importReadLimit).
const maxConfigFile = 2 << 20

// importSlack is what the JSON envelope may add to the message that carries
// the file.
const importSlack = 64 << 10

var errImportTooBig = fmt.Errorf("the file is over %d MiB; this isn't a Sode configuration", maxConfigFile>>20)

// importReadLimit is the read limit for the one message that follows a
// configImportBegin of size bytes. ok is false for a size that can't be a
// config file. JSON may escape a byte to several, but a config.yaml is mostly
// plain text, so twice the size plus slack is plenty; the file itself is
// capped at maxConfigFile, so the message is too.
func importReadLimit(size int) (limit int64, ok bool) {
	if size <= 0 || size > maxConfigFile {
		return 0, false
	}
	return int64(size)*2 + importSlack, true
}

// exportName is the site part of the file name (the page works out the rest).
func exportName(cfg config.Config) string {
	if s := strings.TrimSpace(cfg.BrandText); s != "" {
		return s
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "Sode"
}

// ExportConfig is the live config as config.yaml text. Without pins the
// settings PIN and every user's PIN hash are left out (the rest, users'
// lengths included, is plain settings).
func (a *App) ExportConfig(pins bool) (site, text string, err error) {
	cfg := a.r().cfg.Clone()
	head := "# Sode server configuration, exported " + time.Now().Format("2006-01-02 15:04") + ".\n"
	if pins {
		head += "# PRIVATE BACKUP: this file holds the settings PIN and every user's PIN hash. Keep it secret.\n"
	} else {
		cfg.SettingsPin = ""
		for i := range cfg.Users {
			cfg.Users[i].Pin = ""
		}
		head += "# No PINs in this file. Importing it keeps each user's current PIN and the settings PIN.\n"
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return "", "", err
	}
	return exportName(cfg), head + string(b), nil
}

// mergeImport is the config an imported file makes: the file as it is, except
// that a PIN hash it doesn't carry is the live one. The settings PIN: the
// file's hash, else the live one. A user's PIN hash and its length come from
// the same place: the file's both when it has a hash, else the live user of
// that name's both (the length in a file without a hash is ignored). A user
// with neither can't log in, so that is refused. Hashes in a file must look
// like hashes, so an imported file can't stop the next start.
func mergeImport(live, file config.Config) (config.Config, error) {
	if file.SettingsPin == "" {
		file.SettingsPin = live.SettingsPin
	} else if !config.ValidPinHash(file.SettingsPin) {
		return file, errors.New("settingsPin in the file isn't a PIN hash (a PIN can't be typed into a file; set it in Settings)")
	}
	users := make([]config.User, len(file.Users))
	for i, u := range file.Users {
		if u.Pin != "" {
			if !config.ValidPinHash(u.Pin) {
				return file, fmt.Errorf("the PIN of user %q in the file isn't a PIN hash", u.Name)
			}
		} else {
			keep, ok := live.FindUser(u.Name)
			if !ok || keep.Pin == "" {
				return file, fmt.Errorf("user %q has no PIN in the file and no PIN here to keep: export with PINs included, or add the user in Settings first", u.Name)
			}
			u.Pin, u.Len = keep.Pin, keep.Len
		}
		users[i] = u
	}
	file.Users = users
	return file, nil
}

// ImportConfig replaces the whole live config with the file's, from c. Not
// KeepOwned: everything comes from the file (profiles, filter sets, presets,
// users, locks) except the PIN hashes it leaves out. Refused for anything but
// the settings PIN, and for a file that doesn't parse or validate (the live
// config is left alone). Router changes reconnect through applyConfig.
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
		m, err := mergeImport(*cur, file)
		if err != nil {
			return err
		}
		*cur = m
		return nil
	})
}

// blank: no content but whitespace and comments (which would load as the
// defaults, wiping the configuration).
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

// serveConfigExport answers a configExport. The command log gets the fact and
// whether PINs were included, never the contents.
func (a *App) serveConfigExport(c *client, m *inMsg) {
	id := m.ID
	site, text, err := a.ExportConfig(m.Pins)
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

// serveConfigImport applies an imported file in the background and answers.
func (a *App) serveConfigImport(c *client, m *inMsg) {
	go func(text string, id int) {
		a.replyOpNote(c, "configImport", id, fmt.Sprintf("%d bytes", len(text)), a.ImportConfig(c, text))
	}(m.YAML, m.ID)
}

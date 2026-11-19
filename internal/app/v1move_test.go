package app

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

var v1Files = []string{
	"config.yaml", "config.yaml.prev", "config.yaml.before-1.8.3", "config.yaml.unknown-20261101T000000Z",
	"tokens.json", "keys.json", "keys-ended.json", "keys.json.damaged-20261101T000000Z",
	"panels/p-0000abcd.yaml", "panels/p-0000abcd.checkpoints", "panels/p-0000abcd.art/a-00000001",
	"panelsets/everyone.yaml", "templates/me/t-00000001.yaml", "panelwatch.yaml",
	"icons.yaml", "icons/0000abcd.png", "artdefaults.yaml", "artdefaults.art/d-00000001",
	"history.jsonl", "sent.jsonl", "sent.1.jsonl", "sent.jsonl.bad-20261101-000000",
	"uptime.jsonl", "uptime.jsonl.alive",
}

func writeTree(t *testing.T, dir string, names []string, body func(string) string) {
	t.Helper()
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body(n)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func logsOf(t *testing.T, f func()) []string {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	f()
	return strings.FieldsFunc(buf.String(), func(r rune) bool { return r == '\n' })
}

func moveV1(t *testing.T, dir string) (string, []string) {
	t.Helper()
	var moved string
	lines := logsOf(t, func() {
		var err error
		moved, err = MoveV1Files(filepath.Join(dir, "config.yaml"),
			filepath.Join(dir, "history.jsonl"), filepath.Join(dir, "sent.jsonl"), filepath.Join(dir, "uptime.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
	})
	return moved, lines
}

func TestV1DataStartsFresh(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, v1Files, func(n string) string {
		if n == "config.yaml" {
			return "router: 10.0.0.5:8910\nbrandText: Old\n"
		}
		return n
	})
	writeTree(t, dir, []string{"notes.txt"}, func(string) string { return "not Sode's" })

	moved, lines := moveV1(t, dir)
	want := filepath.Join(dir, "v1")
	if moved != want {
		t.Fatalf("moved to %q, want %q", moved, want)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], want) {
		t.Fatalf("log = %q, want one line naming %s", lines, want)
	}
	for _, n := range v1Files {
		if _, err := os.Stat(filepath.Join(dir, n)); !os.IsNotExist(err) {
			t.Errorf("%s still in the data directory (%v)", n, err)
		}
		b, err := os.ReadFile(filepath.Join(want, n))
		if err != nil {
			t.Errorf("%s not in v1: %v", n, err)
		} else if n != "config.yaml" && string(b) != n {
			t.Errorf("%s changed: %q", n, b)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Errorf("a file Sode does not write was moved: %v", err)
	}
	if _, exists, err := config.Load(filepath.Join(dir, "config.yaml")); exists || err != nil {
		t.Fatalf("after the move: exists=%v err=%v, want a fresh install", exists, err)
	}
}

func TestV2DataStays(t *testing.T) {
	for name, body := range map[string]string{"v2": "format: 2\nrouters: [{address: 10.0.0.5:8910}]\n", "none": ""} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			names := []string{"tokens.json", "history.jsonl"}
			if body != "" {
				names = append(names, "config.yaml")
			}
			writeTree(t, dir, names, func(n string) string {
				if n == "config.yaml" {
					return body
				}
				return n
			})
			moved, lines := moveV1(t, dir)
			if moved != "" || len(lines) != 0 {
				t.Fatalf("moved %q, log %q", moved, lines)
			}
			for _, n := range names {
				if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
					t.Errorf("%s: %v", n, err)
				}
			}
		})
	}
}

func TestV1MoveKeepsAnEarlierV1Directory(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, []string{"v1/config.yaml", "config.yaml", "tokens.json"}, func(n string) string { return "# " + n })
	moved, _ := moveV1(t, dir)
	if moved != filepath.Join(dir, "v1-2") {
		t.Fatalf("moved to %q", moved)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "v1", "config.yaml")); string(b) != "# v1/config.yaml" {
		t.Fatalf("earlier v1 directory changed: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "v1-2", "tokens.json")); err != nil {
		t.Fatal(err)
	}
}

func TestV1MoveTakesLogsFromElsewhere(t *testing.T) {
	dir, other := t.TempDir(), t.TempDir()
	writeTree(t, dir, []string{"config.yaml"}, func(string) string { return "router: 10.0.0.5:8910\n" })
	writeTree(t, other, []string{"hist.jsonl", "hist.jsonl.tmp", "keep.jsonl"}, func(n string) string { return n })
	var moved string
	logsOf(t, func() {
		var err error
		moved, err = MoveV1Files(filepath.Join(dir, "config.yaml"), filepath.Join(other, "hist.jsonl"), "")
		if err != nil {
			t.Fatal(err)
		}
	})
	for _, n := range []string{"hist.jsonl", "hist.jsonl.tmp"} {
		if _, err := os.Stat(filepath.Join(moved, n)); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
	if _, err := os.Stat(filepath.Join(other, "keep.jsonl")); err != nil {
		t.Errorf("keep.jsonl moved: %v", err)
	}
}

func TestV1MoveTakesImageLogsWithoutPaths(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, []string{"config.yaml", "history.jsonl", "sent.jsonl", "sent.1.jsonl", "uptime.jsonl"}, func(n string) string { return "# " + n })
	var moved string
	logsOf(t, func() {
		var err error
		if moved, err = MoveV1Files(filepath.Join(dir, "config.yaml")); err != nil {
			t.Fatal(err)
		}
	})
	for _, n := range []string{"history.jsonl", "sent.jsonl", "sent.1.jsonl", "uptime.jsonl"} {
		if _, err := os.Stat(filepath.Join(moved, n)); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
}

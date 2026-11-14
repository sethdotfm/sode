package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A read of the Save button's disabled state, by id or by selector. Why it is banned:
// docs/agents/browser-checks.md.
var saveDisabledWait = regexp.MustCompile(
	`(?:getElementById\(\s*['"]ed-save['"]\s*\)|querySelector\(\s*['"]#ed-save['"]\s*\))\s*\??\.disabled`)

type waitForCall struct {
	line int
	text string
}

// waitsOnSaveDisabled reports whether a waitFor call reads the disabled state. A single `!` is waiting
// for enabled before a click and passes; `!!` is a disabled wait.
func waitsOnSaveDisabled(call string) bool {
	for _, loc := range saveDisabledWait.FindAllStringIndex(call, -1) {
		before := strings.TrimSuffix(call[:loc[0]], "document.")
		if strings.HasSuffix(before, "!") && !strings.HasSuffix(before, "!!") {
			continue
		}
		return true
	}
	return false
}

// waitForCalls returns each `.waitFor(` call up to its closing paren, ignoring parens inside quotes.
func waitForCalls(src string) []waitForCall {
	var out []waitForCall
	for from := 0; ; {
		i := strings.Index(src[from:], ".waitFor(")
		if i < 0 {
			return out
		}
		start := from + i
		depth, quote := 0, byte(0)
		end := len(src)
	scan:
		for k := start + len(".waitFor"); k < len(src); k++ {
			c := src[k]
			switch {
			case quote != 0:
				if c == '\\' && quote != '`' {
					k++
				} else if c == quote {
					quote = 0
				}
			case c == '"' || c == '`' || c == '\'':
				quote = c
			case c == '(':
				depth++
			case c == ')':
				depth--
				if depth == 0 {
					end = k + 1
					break scan
				}
			}
		}
		out = append(out, waitForCall{strings.Count(src[:start], "\n") + 1, src[start:end]})
		from = end
	}
}

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
		for _, c := range waitForCalls(string(raw)) {
			if waitsOnSaveDisabled(c.text) {
				t.Errorf("%s:%d waits on the Save button's disabled state, which is true before the save reply arrives; use b.waitSaved(what) (browser_test.go)", f, c.line)
			}
		}
	}
}

func TestSaveDisabledWaitPattern(t *testing.T) {
	for call, want := range map[string]bool{
		"b.waitFor(\"x\", `document.getElementById('ed-save').disabled`)":                 true,
		"b.waitFor(\"x\", `document.getElementById(\"ed-save\").disabled`)":               true,
		"b.waitFor(\"x\", `document.querySelector('#ed-save').disabled`)":                 true,
		"b.waitFor(\"x\", `document.querySelector(\"#ed-save\").disabled`)":               true,
		"b.waitFor(\"x\", `a && document.getElementById('ed-save').disabled && b`)":       true,
		"b.waitFor(\"x\", `!!document.getElementById('ed-save').disabled`)":               true,
		"b.waitFor(\"x\", `!!document.querySelector('#ed-save').disabled`)":               true,
		"b.waitFor(\"x\",\n\t`a &&\n\tdocument.getElementById('ed-save').disabled`)":      true,
		"b.waitFor(\n\t\"x\",\n\t`document.getElementById('ed-save').disabled`,\n)":       true,
		"b.waitFor(\"x\", `!document.getElementById('ed-save').disabled`)":                false,
		"b.waitFor(\"x\", `!document.querySelector('#ed-save').disabled`)":                false,
		"b.waitFor(\"x\", `document.getElementById('ed-state').textContent === 'Saved'`)": false,
	} {
		calls := waitForCalls(call)
		if len(calls) != 1 {
			t.Fatalf("%q: found %d waitFor calls", call, len(calls))
		}
		if got := waitsOnSaveDisabled(calls[0].text); got != want {
			t.Errorf("%q: got %v, want %v", call, got, want)
		}
	}
}

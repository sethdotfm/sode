package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Keeps the repo in step with GLOSSARY.md. Every retired word (the _Avoid_ lines) has a row in
// retiredWords. A scope with a pending ticket is skipped until that ticket lands, and its
// ticket then clears the field, so the list only shrinks. Kept names go in vocabAllow.

// A word is matched as the whole token: not inside an identifier, class, key or path.
const (
	vocabBefore = `(?:^|[^\w#./-])`
	vocabAfter  = `(?:[^\w-]|$)`
)

type retiredWord struct {
	// Terms are the glossary _Avoid_ terms this row covers, as vocabTerms normalises them.
	Terms []string
	// Re finds the word in the docs or the UI text. Nil means no mechanical scan exists and
	// the word is reviewed by hand, with Why saying so.
	Re  *regexp.Regexp
	Why string
	// Docs and UI hold the ticket that will sweep that scope; empty means enforced now.
	Docs, UI string
}

func wordRe(expr string) *regexp.Regexp {
	return regexp.MustCompile(vocabBefore + `(` + expr + `)` + vocabAfter)
}

var retiredWords = []retiredWord{
	{Terms: []string{"input", "output"}, Re: wordRe(`(?i)router (?:input|output)s?`)},
	{Terms: []string{"wire level"}, Re: wordRe(`(?i)wire levels?`), UI: "#210"},
	{Terms: []string{"layer"}, Re: wordRe(`(?i)signal layers?`)},
	{Terms: []string{"label"}, Re: wordRe(`(?i)(?:router|source|destination) labels?`)},
	{Terms: []string{"refresh"}, Re: wordRe(`(?i)names? refreshe?s?|refresh(?:es|ed|ing)? (?:of )?(?:the )?(?:router )?names?`), UI: "#210"},
	{Terms: []string{"protect"}, Re: wordRe(`(?i)protects|the protect|protect (?:flag|state|status|badge|icon)`)},
	{Terms: []string{"locked"}, Re: wordRe(`(?i)locked (?:destination|dst)s?`)},
	{Terms: []string{"batch"}, Re: wordRe(`(?i)batch(?:es)?`), UI: "#210"},
	{
		Terms: []string{`using "take" for any other command`, "a plan or saved entry", `as an ordinary verb meaning "accept" or "interpret literally"`},
		Why:   "a scan cannot tell a route or a preset entry from a take, so #206, #207 and #208 judge it by hand",
	},
	{Terms: []string{"pending take"}, Re: wordRe(`(?i)pending takes?`)},
	{Terms: []string{"salvo"}, Re: wordRe(`(?i)salvos?`), UI: "#210"},
	{Terms: []string{"Recent"}, Re: wordRe(`Recent`), UI: "#207"},
	{Terms: []string{"folder"}, Re: wordRe(`(?i)folders?`), UI: "#210"},
	{Terms: []string{"sent log"}, Re: wordRe(`(?i)sent[ -]logs?`)},
	{Terms: []string{"panel change"}, Re: wordRe(`(?i)panel[ -](?:changes?|entr(?:y|ies)|rows?)|changes? (?:from|on) (?:a |the )?(?:hardware )?panel`)},
	{Terms: []string{"outside change"}, Re: wordRe(`(?i)outside changes?`)},
	{Terms: []string{"follows"}, Re: wordRe(`(?i)follows`), UI: "#207"},
	{Terms: []string{"automation"}, Re: wordRe(`(?i)automation`), UI: "#207"},
	{Terms: []string{"operator"}, Re: wordRe(`(?i)operators?`), UI: "#210"},
	{Terms: []string{"account"}, Re: wordRe(`(?i)accounts?`), UI: "#209"},
	{Terms: []string{"log in", "log out"}, Re: wordRe(`(?i)log(?:s|ged|ging)? (?:in|out)|log-?(?:in|out)s?`), UI: "#209"},
	{Terms: []string{"unlock"}, Re: wordRe(`(?i)unlock(?:s|ed|ing)?`), UI: "#209"},
	{Terms: []string{"Settings PIN"}, Re: wordRe(`(?i)settings pin`), UI: "#209"},
	{Terms: []string{"admin PIN"}, Re: wordRe(`(?i)admin pin`)},
	{Terms: []string{"hand lock", "soft lock", "screen lock"}, Re: wordRe(`(?i)(?:hand|soft|screen)[ -]lock`), UI: "#209"},
	{Terms: []string{"idle timeout"}, Re: wordRe(`(?i)idle[ -]timeout`)},
	{Terms: []string{"This device"}, Re: wordRe(`(?i)this device`), UI: "#210"},
}

// vocabMask blanks phrases that contain a retired word but stay: the Advanced Settings PIN,
// the router's own salvo and the native group-send fire mode.
var vocabMask = []*regexp.Regexp{
	regexp.MustCompile(`(?i)advanced settings pin`),
	regexp.MustCompile(`(?i)\bas follows\b`),
	regexp.MustCompile(`(?i)\b(?:native|router'?s?(?: own)?) salvos?\b`),
	regexp.MustCompile(`(?i)\bfire salvo \d+`),
	regexp.MustCompile(`\*\*Salvo\*\*`),
}

// vocabAllow exempts whole lines from one row, named by its first term (empty means any).
// File is a path suffix and Line a regexp; Why says what is kept.
var vocabAllow = []struct{ Term, File, Line, Why string }{
	{"salvo", "docs/agents/salvos.md", ".", "the native group-send fire mode and the router's salvos"},
	{"unlock", "", "`unlock(?:ed)?`", "wire message and bus event names keep this name"},
	{"log in", "", "`[^`]*log-?(?:in|out)[^`]*`", "bus event names keep this name"},
	{"salvo", "docs/agents/presets.md", "Old data still loads", "legacy data keys keep the old name so old files load"},
	{"batch", "", "`batch`", "the wire message type and command kind keep this name"},
	{"", "docs/vocabulary-plan.md", `^\|`, "the checklist tables quote the old words by design"},
}

type vocabHit struct {
	File string
	Line int
	Text string
}

func repoRoot() string { return filepath.Join("..", "..") }

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(), rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// vocabTerms reads the _Avoid_ terms: split on commas outside parentheses, drop the
// parenthetical note and a leading "or".
func vocabTerms(glossary string) []string {
	var out []string
	for _, line := range strings.Split(glossary, "\n") {
		rest, ok := strings.CutPrefix(line, "_Avoid_:")
		if !ok {
			continue
		}
		depth, start := 0, 0
		parts := []string{}
		for i, r := range rest {
			switch r {
			case '(':
				depth++
			case ')':
				depth--
			case ',':
				if depth == 0 {
					parts = append(parts, rest[start:i])
					start = i + 1
				}
			}
		}
		parts = append(parts, rest[start:])
		for _, p := range parts {
			p = regexp.MustCompile(`\s*\([^)]*\)`).ReplaceAllString(p, "")
			p = strings.TrimSpace(p)
			p = strings.TrimPrefix(p, "or ")
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// termRe matches a glossary term as written, folding only the first letter of a lowercase
// term so "Input" at a sentence start still counts while "recent" never matches "Recent".
func termRe(term string) *regexp.Regexp {
	first := term[:1]
	expr := regexp.QuoteMeta(term)
	if strings.ToLower(first) == first && strings.ToUpper(first) != first {
		expr = "[" + first + strings.ToUpper(first) + "]" + regexp.QuoteMeta(term[1:])
	}
	return wordRe(expr + `s?`)
}

func masked(line string) string {
	for _, m := range vocabMask {
		line = m.ReplaceAllStringFunc(line, func(s string) string { return strings.Repeat(" ", len(s)) })
	}
	return line
}

func allowed(term, file, line string) bool {
	for _, a := range vocabAllow {
		if (a.Term == "" || a.Term == term) && strings.HasSuffix(file, a.File) && regexp.MustCompile(a.Line).MatchString(line) {
			return true
		}
	}
	return false
}

func scanLines(term, file, text string, re *regexp.Regexp, skip func(string) bool) []vocabHit {
	var hits []vocabHit
	for i, line := range strings.Split(text, "\n") {
		if skip != nil && skip(line) {
			continue
		}
		if re.MatchString(masked(line)) && !allowed(term, file, line) {
			hits = append(hits, vocabHit{file, i + 1, strings.TrimSpace(line)})
		}
	}
	return hits
}

func isAvoidLine(line string) bool { return strings.HasPrefix(line, "_Avoid_:") }

// docFiles are the prose files: the root guides and everything under docs/. CHANGELOG.md
// records history and keeps the old words.
func docFiles(t *testing.T) []string {
	t.Helper()
	files := []string{"AGENTS.md", "GLOSSARY.md", "README.md", "RELEASING.md"}
	err := filepath.WalkDir(filepath.Join(repoRoot(), "docs"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			rel, _ := filepath.Rel(repoRoot(), p)
			files = append(files, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

var (
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->|<(script|style)\b.*?</(script|style)>`)
	htmlTag     = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlAttr    = regexp.MustCompile(`\b(?:title|placeholder|aria-label|alt|data-label|data-pb-tip)="([^"]*)"`)
)

// htmlText returns the text a person reads in a page: text nodes and the display attributes.
func htmlText(src string) string {
	src = htmlComment.ReplaceAllString(src, "")
	var out []string
	for _, m := range htmlAttr.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	out = append(out, htmlTag.Split(src, -1)...)
	return strings.Join(out, "\n")
}

// jsStrings returns the string literals of a script, comments skipped, one per line. A
// literal that spans lines (a template) is split so line numbers stay approximate.
func jsStrings(src string) []string {
	var out []string
	for i := 0; i < len(src); {
		switch c := src[i]; {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return out
			}
			i += end + 4
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < len(src) && src[j] != c && (c == '`' || src[j] != '\n') {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			if j > len(src) {
				j = len(src)
			}
			out = append(out, src[i+1:min(j, len(src))])
			i = j + 1
		default:
			i++
		}
	}
	return out
}

func uiTexts(t *testing.T) map[string]string {
	t.Helper()
	texts := map[string]string{}
	pages, _ := filepath.Glob(filepath.Join(repoRoot(), "web", "*.html"))
	for _, p := range pages {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		texts["web/"+filepath.Base(p)] = htmlText(string(b))
	}
	scripts, _ := filepath.Glob(filepath.Join(repoRoot(), "web", "*.js"))
	for _, p := range scripts {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		texts["web/"+filepath.Base(p)] = strings.Join(jsStrings(string(b)), "\n")
	}
	return texts
}

func hitReport(hits []vocabHit) string {
	var b strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&b, "\n  %s:%d  %s", h.File, h.Line, h.Text)
	}
	return b.String()
}

func TestRetiredWordsCoverGlossary(t *testing.T) {
	covered := map[string]bool{}
	for _, w := range retiredWords {
		if w.Re == nil && !regexp.MustCompile(`#\d+`).MatchString(w.Why) {
			t.Errorf("%v has no scan, so Why must say which ticket judges it", w.Terms)
		}
		for _, term := range w.Terms {
			covered[term] = true
		}
	}
	for _, term := range vocabTerms(readRepo(t, "GLOSSARY.md")) {
		if !covered[term] {
			t.Errorf("GLOSSARY.md _Avoid_ term %q has no row in retiredWords", term)
		}
	}
	for _, w := range retiredWords {
		for _, term := range w.Terms {
			found := false
			for _, g := range vocabTerms(readRepo(t, "GLOSSARY.md")) {
				found = found || g == term
			}
			if !found {
				t.Errorf("retiredWords lists %q, which is on no _Avoid_ line", term)
			}
		}
		for _, ticket := range []string{w.Docs, w.UI} {
			if ticket != "" && !regexp.MustCompile(`^#\d+$`).MatchString(ticket) {
				t.Errorf("%v: pending ticket %q is not like #206", w.Terms, ticket)
			}
		}
	}
}

// The scan itself: it must flag a stray retired word and keep the allowed phrases.
func TestVocabularyScanMechanics(t *testing.T) {
	terms := vocabTerms("_Avoid_: batch, using \"take\" for any, or log in (for a session), hand lock\n**X**\n_Avoid_: input")
	want := []string{"batch", `using "take" for any`, "log in", "hand lock", "input"}
	if strings.Join(terms, "|") != strings.Join(want, "|") {
		t.Errorf("vocabTerms = %q, want %q", terms, want)
	}
	re := wordRe(`Recent`)
	for line, hit := range map[string]bool{
		"Clear Recent":                   true,
		"the most recent take":           false,
		"sode__recent and desktoprecent": false,
		"Settings PIN":                   false,
	} {
		if got := len(scanLines("x", "a.md", line, re, nil)) > 0; got != hit {
			t.Errorf("Recent in %q: got %v, want %v", line, got, hit)
		}
	}
	salvo := wordRe(`(?i)salvos?`)
	for line, hit := range map[string]bool{
		"fire salvo 12 on the router": false,
		"the router's own salvo":      false,
		"native salvo mode":           false,
		"save it as a salvo":          true,
	} {
		if got := len(scanLines("salvo", "a.md", line, salvo, nil)) > 0; got != hit {
			t.Errorf("salvo in %q: got %v, want %v", line, got, hit)
		}
	}
	pin := wordRe(`(?i)settings pin`)
	if len(scanLines("Settings PIN", "a.md", "the Advanced Settings PIN", pin, nil)) > 0 {
		t.Error("Advanced Settings PIN is kept")
	}
	if len(scanLines("Settings PIN", "a.md", "the Settings PIN", pin, nil)) == 0 {
		t.Error("a bare Settings PIN is retired")
	}
	if got := jsStrings("// 'no'\nconst a = 'Recent' /* \"no\" */ + \"x\";"); strings.Join(got, "|") != "Recent|x" {
		t.Errorf("jsStrings = %q", got)
	}
}

func TestGlossaryAvoidsItsOwnTerms(t *testing.T) {
	glossary := readRepo(t, "GLOSSARY.md")
	var hits []vocabHit
	for _, term := range vocabTerms(glossary) {
		hits = append(hits, scanLines("", "GLOSSARY.md", glossary, termRe(term), isAvoidLine)...)
	}
	if len(hits) > 0 {
		t.Errorf("GLOSSARY.md uses a term from an _Avoid_ line outside any _Avoid_ line:%s", hitReport(hits))
	}
}

// checkScope fails on a hit in an enforced scope, and on a pending scope with no hits left,
// so a finished sweep has to clear its ticket and the pending list only shrinks.
func checkScope(t *testing.T, scope string, ticket func(retiredWord) string, texts map[string]string) {
	t.Helper()
	files := make([]string, 0, len(texts))
	for f := range texts {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, w := range retiredWords {
		if w.Re == nil {
			continue
		}
		var hits []vocabHit
		for _, f := range files {
			var skip func(string) bool
			if f == "GLOSSARY.md" {
				skip = isAvoidLine
			}
			hits = append(hits, scanLines(w.Terms[0], f, texts[f], w.Re, skip)...)
		}
		switch pending := ticket(w); {
		case pending == "" && len(hits) > 0:
			t.Errorf("retired word %q in %s (sweep it, or list it as pending with its ticket):%s", w.Terms[0], scope, hitReport(hits))
		case pending != "" && len(hits) == 0:
			t.Errorf("%q is clean in %s: clear its pending ticket %s so it stays enforced", w.Terms[0], scope, pending)
		}
	}
}

func TestDocsUseGlossaryWords(t *testing.T) {
	texts := map[string]string{}
	for _, f := range docFiles(t) {
		texts[f] = readRepo(t, f)
	}
	checkScope(t, "the docs", func(w retiredWord) string { return w.Docs }, texts)
}

func TestUIStringsUseGlossaryWords(t *testing.T) {
	checkScope(t, "the UI strings", func(w retiredWord) string { return w.UI }, uiTexts(t))
}

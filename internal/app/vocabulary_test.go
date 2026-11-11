package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
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
	// Docs, UI and Ident hold the ticket that will sweep that scope; empty means enforced now.
	Docs, UI, Ident string
}

func wordRe(expr string) *regexp.Regexp {
	return regexp.MustCompile(vocabBefore + `(` + expr + `)` + vocabAfter)
}

var retiredWords = []retiredWord{
	{Terms: []string{"input", "output"}, Re: wordRe(`(?i)router (?:input|output)s?`)},
	{Terms: []string{"wire level"}, Re: wordRe(`(?i)wire levels?`)},
	{Terms: []string{"layer"}, Re: wordRe(`(?i)signal layers?`)},
	{Terms: []string{"label"}, Re: wordRe(`(?i)(?:router|source|destination) labels?`)},
	{Terms: []string{"refresh"}, Re: wordRe(`(?i)names? refreshe?s?|refresh(?:es|ed|ing)? (?:of )?(?:the )?(?:router )?names?`)},
	{Terms: []string{"protect"}, Re: wordRe(`(?i)protects|the protect|protect (?:flag|state|status|badge|icon)`)},
	{Terms: []string{"locked"}, Re: wordRe(`(?i)locked (?:destination|dst)s?`)},
	{Terms: []string{"batch"}, Re: wordRe(`(?i)batch(?:es)?`)},
	{
		Terms: []string{`using "take" for any other command`, "a plan or saved entry", `as an ordinary verb meaning "accept" or "interpret literally"`},
		Why:   "a scan cannot tell a route or a preset entry from a take, so a reviewer judges it by hand",
	},
	{Terms: []string{"pending take"}, Re: wordRe(`(?i)pending takes?`)},
	{Terms: []string{"salvo"}, Re: wordRe(`(?i)salvos?`)},
	{Terms: []string{"Recent"}, Re: wordRe(`Recent`)},
	{Terms: []string{"folder"}, Re: wordRe(`(?i)folders?`)},
	{Terms: []string{"sent log"}, Re: wordRe(`(?i)sent[ -]logs?`)},
	{Terms: []string{"panel change"}, Re: wordRe(`(?i)panel[ -](?:changes?|entr(?:y|ies)|rows?)|changes? (?:from|on) (?:a |the )?(?:hardware )?panel`)},
	{Terms: []string{"external change"}, Re: wordRe(`(?i)external changes?`)},
	{Terms: []string{"outside change"}, Re: wordRe(`(?i)outside changes?`)},
	{Terms: []string{"follows"}, Re: wordRe(`(?i)follows`)},
	{Terms: []string{"automation"}, Re: wordRe(`(?i)automation`)},
	{Terms: []string{"operator"}, Re: wordRe(`(?i)operators?`)},
	{Terms: []string{"account"}, Re: wordRe(`(?i)accounts?`)},
	{Terms: []string{"log in", "log out"}, Re: wordRe(`(?i)log(?:s|ged|ging)? (?:in|out)|log-?(?:in|out)s?`)},
	{Terms: []string{"unlock"}, Re: wordRe(`(?i)unlock(?:s|ed|ing)?`)},
	{Terms: []string{"Settings PIN"}, Re: wordRe(`(?i)settings pin`)},
	{Terms: []string{"admin PIN"}, Re: wordRe(`(?i)admin pin`)},
	{Terms: []string{"hand lock", "soft lock", "screen lock"}, Re: wordRe(`(?i)(?:hand|soft|screen)[ -]lock`)},
	{Terms: []string{"idle timeout"}, Re: wordRe(`(?i)idle[ -]timeout`)},
	{Terms: []string{"This device"}, Re: wordRe(`(?i)this device`)},
	// Only the panel sense: "lock the screen", "screen readers" and "home screen" stay.
	{Terms: []string{"screen"}, Re: wordRe(`(?i)(?:this|that|one|each|every|per|no|watched|named|lost an?) screens?|screens?(?:'s)? (?:names?|offline|drops|signs? out|signed out|has opened|of (?:one|a|the) panel)|panels? (?:and|or) screens?`)},
	{Terms: []string{"anonymous panel"}, Re: wordRe(`(?i)anonymous panels?`)},
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
var vocabAllow = []allowEntry{
	{"salvo", "docs/agents/salvos.md", ".", "the native group-send fire mode and the router's salvos"},
	{"account", "web/settingsindex.js", ".", "search aliases keep the retired words so a search for them still finds the setting"},
	{"log in", "web/settingsindex.js", ".", "search aliases keep the retired words so a search for them still finds the setting"},
	{"unlock", "web/settingsindex.js", ".", "search aliases keep the retired words so a search for them still finds the setting"},
	{"Settings PIN", "web/settingsindex.js", ".", "search aliases keep the retired words so a search for them still finds the setting"},
	{"hand lock", "web/settingsindex.js", ".", "search aliases keep the retired words so a search for them still finds the setting"},
	{"This device", "web/settingsindex.js", ".", "search aliases keep the retired words so a search for them still finds the setting"},
	{"", ".js", `^(?:login|logout|unlock|unlocked)$`, "bus and wire event names, which do not change"},
	{"unlock", "", "`unlock(?:ed)?`", "wire message and bus event names keep this name"},
	{"log in", "", "`[^`]*log-?(?:in|out)[^`]*`", "bus event names keep this name"},
	{"salvo", "docs/agents/presets.md", "Old data still loads", "legacy data keys keep the old name so old files load"},
	{"batch", "", "`batch`", "the wire message type and command kind keep this name"},
	{"batch", "web/plans.js", `^batch$`, "the wire message type keeps this name"},
	{"batch", "web/settingsindex.js", ".", "search aliases keep the retired words so a search for them still finds the setting"},
	{"salvo", "web/settingsindex.js", ".", "search aliases keep the retired words so a search for them still finds the setting"},
	{"operator", "web/settingsindex.js", ".", "search aliases keep the retired words so a search for them still finds the setting"},
	{"", "docs/vocabulary-plan.md", `^\|`, "the checklist tables quote the old words by design"},
	{"", "docs/persisted-names.md", `^\|`, "the audit tables name each kept name and the retired word it carries"},
	{"salvo", "AGENTS.md", "^  - (?:Config and env|`salvoGroup`)", "the kept-names list names legacy keys and the router's own salvo"},
}

// identAllow exempts web identifiers only, so a UI string or a doc line gets no pass. Wire message
// types and `sode-*` keys are strings and never reach that scan; these names hold a wire or logged value.
var identAllow = []allowEntry{
	{"panel change", "web/panelpage.js", `panelRows`, "the rows of a panel's grid; the avoided phrase means a change made on a hardware panel"},
	{"Recent", "web/core.js", `m\.recent`, "the `recent` message carries its entries under this field"},
	{"refresh", "web/logkinds.js", `^\s*refreshNames:`, "`log` command kind, as the server logs it"},
	{"salvo", "web/log.js", `^\s*salvos?(?:Stage|Go)?:`, "`log` command kinds and the router's own salvo"},
	{"salvo", "web/logkinds.js", `^\s*salvos?:`, "`log` command kinds, as the server logs them"},
	{"salvo", "web/test/settingsindex.test.mjs", `^\s*salvo:`, "a search alias keeps the retired word"},
	{"unlock", "web/logkinds.js", `^\s*(?:unlock|userUnlock|adminUnlock):`, "`log` command kinds, as the server logs them"},
	{"unlock", "", `\b(?:unlock|unlocked|isUnlocked|lostUnlock|sendUnlock)\b`, "names for the `unlock` and `userUnlock` messages and the access they grant (pin.js, lock.js); they hold a wire value, not a UI word"},
	{"follows", ".js", `[Ff]ollows`, "the entries that followed one, built from the wire `follow` id; the glossary's External follow is the idea"},
	{"follows", ".mjs", `[Ff]ollows`, "the entries that followed one, built from the wire `follow` id; the glossary's External follow is the idea"},
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

type allowEntry = struct{ Term, File, Line, Why string }

// allowUsed records which allowlist entries exempted a real hit, so
// TestAllowlistsMatchSomething can fail on an entry that exempts nothing.
var (
	allowMu   sync.Mutex
	allowUsed = map[string]bool{}
)

func allowKey(kind string, a allowEntry) string {
	return kind + "|" + a.Term + "|" + a.File + "|" + a.Line
}

// matchAllow reports whether any entry exempts the line and marks every entry that does,
// so an entry is not called unused because an earlier one matched first.
func matchAllow(kind string, list []allowEntry, exactTerm bool, term, file, line string) bool {
	found := false
	for _, a := range list {
		if (a.Term == "" && !exactTerm || a.Term == term) && strings.HasSuffix(file, a.File) && regexp.MustCompile(a.Line).MatchString(line) {
			allowMu.Lock()
			allowUsed[allowKey(kind, a)] = true
			allowMu.Unlock()
			found = true
		}
	}
	return found
}

func allowed(term, file, line string) bool {
	return matchAllow("vocab", vocabAllow, false, term, file, line)
}

func identAllowed(term, file, line string) bool {
	a := allowed(term, file, line)
	b := matchAllow("ident", identAllow, true, term, file, line)
	return a || b
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
		if w.Re == nil && w.Why == "" {
			t.Errorf("%v has no scan, so Why must say who judges it", w.Terms)
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
				t.Errorf("%v: pending ticket %q is not a ticket number", w.Terms, ticket)
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

var (
	identWord  = regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]*`)
	identParts = regexp.MustCompile(`[A-Z]+[a-z0-9]*|[a-z0-9]+`)
)

// jsCode returns a script with its comments and string literals blanked, line breaks kept,
// so what is left is identifiers and punctuation. Wire types and `sode-*` keys are strings
// and so never reach the identifier scan.
func jsCode(src string) string {
	b := []byte(src)
	blank := func(from, to int) {
		for k := from; k < to && k < len(b); k++ {
			if b[k] != '\n' {
				b[k] = ' '
			}
		}
	}
	for i := 0; i < len(b); {
		switch c := b[i]; {
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			j := i
			for j < len(b) && b[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			end := strings.Index(string(b[i+2:]), "*/")
			if end < 0 {
				blank(i, len(b))
				return string(b)
			}
			blank(i, i+end+4)
			i += end + 4
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < len(b) && b[j] != c && (c == '`' || b[j] != '\n') {
				if b[j] == '\\' {
					j++
				}
				j++
			}
			blank(i, j+1)
			i = j + 1
		default:
			i++
		}
	}
	return string(b)
}

// identWords splits each identifier of a line into lowercase words (recentRow becomes
// "recent row", lock__pad "lock pad"), so the rows that scan prose scan names too.
func identWords(line string) string {
	var out []string
	for _, id := range identWord.FindAllString(line, -1) {
		out = append(out, strings.ToLower(strings.Join(identParts.FindAllString(id, -1), " ")))
	}
	return strings.Join(out, " ; ")
}

// identOnly holds names a prose scan cannot judge: a per-destination unit is a Route, and the
// word take is for the whole confirm. takeResult stays a take: it is the wire answer to one.
var identOnly = []retiredWord{
	{Terms: []string{"take (a route)"}, Re: regexp.MustCompile(`take (?:item|text)|from takes|picked takes|\br ; takes\b`)},
}

// identRe makes a row's prose pattern work on lowercase identifier words.
func identRe(w retiredWord) *regexp.Regexp {
	return regexp.MustCompile("(?i)" + w.Re.String())
}

// identFiles returns the code of every web script and test, and every file name under web/
// and web/test/.
func identFiles(t *testing.T) (code map[string]string, names []string) {
	t.Helper()
	code = map[string]string{}
	for _, pat := range []string{"web/*", "web/test/*"} {
		paths, _ := filepath.Glob(filepath.Join(repoRoot(), pat))
		for _, p := range paths {
			if st, err := os.Stat(p); err != nil || st.IsDir() {
				continue
			}
			rel, _ := filepath.Rel(repoRoot(), p)
			rel = filepath.ToSlash(rel)
			names = append(names, rel)
			if strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".mjs") {
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				code[rel] = jsCode(string(b))
			}
		}
	}
	sort.Strings(names)
	return code, names
}

func TestIdentifierScanMechanics(t *testing.T) {
	src := "const recentRow = 'Recent'; // folder here\nfunction sendBatch() {}\n/* salvo */ let m__recent;"
	code := jsCode(src)
	words := identWords(code)
	for _, want := range []string{"recent row", "send batch", "m recent"} {
		if !strings.Contains(words, want) {
			t.Errorf("identWords missing %q in %q", want, words)
		}
	}
	for _, leaked := range []string{"folder", "salvo"} {
		if strings.Contains(words, leaked) {
			t.Errorf("comment text %q leaked: %q", leaked, words)
		}
	}
	if strings.Contains(code, "'Recent'") {
		t.Error("string literal not blanked")
	}
	if !identRe(retiredWords[7]).MatchString("send batch") || !identRe(retiredWords[11]).MatchString("recent row") {
		t.Error("identRe does not match identifier words")
	}
}

// TestIdentifiersUseGlossaryWords scans names, not text: variables, functions, properties
// and file names. Wire message types and `sode-*` keys are strings and stay out of scope.
func TestIdentifiersUseGlossaryWords(t *testing.T) {
	code, names := identFiles(t)
	files := make([]string, 0, len(code))
	for f := range code {
		files = append(files, f)
	}
	sort.Strings(files)
	baseWords := strings.NewReplacer("_", " ", "-", " ", ".", " ")
	for _, w := range append(append([]retiredWord{}, retiredWords...), identOnly...) {
		if w.Re == nil {
			continue
		}
		re := identRe(w)
		var hits []vocabHit
		for _, f := range files {
			for i, line := range strings.Split(code[f], "\n") {
				if re.MatchString(masked(identWords(line))) && !identAllowed(w.Terms[0], f, line) {
					hits = append(hits, vocabHit{f, i + 1, strings.TrimSpace(line)})
				}
			}
		}
		for _, n := range names {
			base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(n), ".mjs"), ".js")
			if re.MatchString(baseWords.Replace(base)) && !identAllowed(w.Terms[0], n, base) {
				hits = append(hits, vocabHit{n, 0, "file name"})
			}
		}
		switch pending := w.Ident; {
		case pending == "" && len(hits) > 0:
			t.Errorf("retired word %q in web identifiers (rename it, or list it as pending with its ticket):%s", w.Terms[0], hitReport(hits))
		case pending != "" && len(hits) == 0:
			t.Errorf("%q is clean in web identifiers: clear its pending ticket %s so it stays enforced", w.Terms[0], pending)
		}
	}
}

// An allowlist entry that exempts no real hit names something renamed or removed, and would
// hide the next stray use. The three scans run here to record which entries they needed.
func TestAllowlistsMatchSomething(t *testing.T) {
	t.Run("docs", TestDocsUseGlossaryWords)
	t.Run("ui", TestUIStringsUseGlossaryWords)
	t.Run("identifiers", TestIdentifiersUseGlossaryWords)
	allowMu.Lock()
	defer allowMu.Unlock()
	for _, a := range vocabAllow {
		if !allowUsed[allowKey("vocab", a)] {
			t.Errorf("vocabAllow entry exempts nothing: %q in %q matching %q (%s)", a.Term, a.File, a.Line, a.Why)
		}
	}
	for _, a := range identAllow {
		if !allowUsed[allowKey("ident", a)] {
			t.Errorf("identAllow entry exempts nothing: %q in %q matching %q (%s)", a.Term, a.File, a.Line, a.Why)
		}
	}
}

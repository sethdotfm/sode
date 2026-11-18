package app

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Checks the rules of docs/ste.md that a program can check. A scope set false in steEnforced
// is still scanned, so allowlist use is counted, but its hits are only logged.
// STE_REPORT=<file> writes every hit, one per line, as the work list for a sweep.

var steEnforced = map[string]bool{
	"docs":       true,
	"agentdocs":  true,
	"ui":         true,
	"gocomments": true,
	"gostrings":  true,
	"jscomments": true,
}

type steWord struct {
	ID string
	Re *regexp.Regexp
}

// Keep in step with the word table in docs/ste.md.
var steWords = []steWord{
	{"utilize", wordRe(`(?i)utili[sz](?:e|es|ed|ing)|leverag(?:e|es|ed|ing)`)},
	{"ensure", wordRe(`(?i)ensur(?:e|es|ed|ing)`)},
	{"prior to", wordRe(`(?i)prior to`)},
	{"in order to", wordRe(`(?i)in order to`)},
	{"via", wordRe(`(?i)via`)},
	{"facilitate", wordRe(`(?i)facilitat(?:e|es|ed|ing)`)},
	{"numerous", wordRe(`(?i)numerous`)},
	{"approximately", wordRe(`(?i)approximately`)},
	{"subsequently", wordRe(`(?i)subsequently`)},
	{"e.g.", regexp.MustCompile(`(?i)(?:^|\W)(?:e\.g\.|i\.e\.|etc\.)`)},
	{"simply", wordRe(`(?i)simply|just`)},
	{"please", wordRe(`(?i)please`)},
	{"in case", wordRe(`(?i)in case`)},
	{"whether or not", wordRe(`(?i)whether or not`)},
	{"may", wordRe(`(?i)may|might`)},
	{"contraction", regexp.MustCompile(`(?i)\b(?:\w+n't|(?:it|that|there|here|what|who|you|we|they|i|let)['’](?:s|re|ll|ve|d|m))\b|\b\w+n’t\b`)},
}

// steAllow exempts lines from one rule (Term is a steWords ID, or "length"), as vocabAllow does.
var steAllow = []allowEntry{
	{"length", "GLOSSARY.md", `^What a cell does: `, "a list of the cell kinds, not a sentence to split"},
	{"via", "internal/app/apiaudit.go", `^unverified via:`, "the Communication Log note names the Via field, and tests and docs/agents/comm-log.md match it"},
	{"length", "internal/app/http.go", `^default-src `, "the Content-Security-Policy header, not prose"},
	{"length", "internal/app/settings.go", `^a change to these settings needs`, "a list of the guarded settings after a colon, not a sentence to split"},
}

const (
	steDescriptiveMax = 25
	steUIMax          = 20
)

// steUnit is one paragraph, comment group or UI string, with the line it starts on.
type steUnit struct {
	File string
	Line int
	Text string
}

type steHit struct {
	Scope, File, Rule, Text string
	Line                    int
}

var (
	mdFence     = regexp.MustCompile("^\\s*(```|~~~)")
	mdListStart = regexp.MustCompile(`^\s*(?:[-*+]|\d+\.)\s`)
	mdSkipLine  = regexp.MustCompile(`^\s*(?:#|\||_Avoid_:|\*\*[^*]+\*\*:?\s*$|DISCLOSURE:|<!--|[-*_]{3,}\s*$)`)
	mdCode      = regexp.MustCompile("`[^`]*`")
	mdLink      = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	mdURL       = regexp.MustCompile(`https?://\S+`)
	mdEmph      = regexp.MustCompile(`\*\*|__`)
	mdQuote     = regexp.MustCompile(`"[^"\n]*"`)
	sentenceEnd = regexp.MustCompile(`[.!?]["')\]]*(?:\s+|$)`)
	wordToken   = regexp.MustCompile(`[\p{L}\p{N}]`)
	uiSplit     = regexp.MustCompile(`<[^>]*>|\$\{[^}]*\}|\n`)
)

// steClean blanks what is out of scope inside prose: code spans, quotations, URLs and link
// targets. A quotation counts as one word, like a code span.
func steClean(s string) string {
	s = mdCode.ReplaceAllString(s, "X")
	s = mdQuote.ReplaceAllString(s, "X")
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdURL.ReplaceAllString(s, "X")
	return mdEmph.ReplaceAllString(s, "")
}

func mdUnits(file, src string) []steUnit {
	var units []steUnit
	var cur []string
	start := 0
	flush := func() {
		if len(cur) > 0 {
			units = append(units, steUnit{file, start, strings.Join(cur, " ")})
		}
		cur = nil
	}
	fence := false
	for i, line := range strings.Split(src, "\n") {
		if mdFence.MatchString(line) {
			fence = !fence
			flush()
			continue
		}
		if fence || strings.TrimSpace(line) == "" || mdSkipLine.MatchString(line) {
			flush()
			continue
		}
		if mdListStart.MatchString(line) {
			flush()
		}
		if len(cur) == 0 {
			start = i + 1
		}
		cur = append(cur, strings.TrimSpace(mdListStart.ReplaceAllString(line, "")))
	}
	flush()
	return units
}

func steDocFiles(t *testing.T) (docs, agent []string) {
	t.Helper()
	for _, f := range docFiles(t) {
		switch {
		case strings.HasPrefix(f, "docs/adr/"), strings.HasPrefix(f, "docs/superpowers/"):
		case f == "AGENTS.md", strings.HasPrefix(f, "docs/agents/"):
			agent = append(agent, f)
		default:
			docs = append(docs, f)
		}
	}
	frags, _ := filepath.Glob(filepath.Join(repoRoot(), "changelog.d", "*.md"))
	for _, p := range frags {
		docs = append(docs, "changelog.d/"+filepath.Base(p))
	}
	return docs, agent
}

func lineAt(src string, off int) int { return strings.Count(src[:off], "\n") + 1 }

// regexStart reports whether a / at i opens a regex literal. After an operator, an opening
// bracket or a keyword it does. After a value it divides.
func regexStart(src string, i int) bool {
	j := i - 1
	for j >= 0 && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
		j--
	}
	if j < 0 || strings.ContainsRune("(,=:[!&|?{};+-*%<>~^", rune(src[j])) {
		return true
	}
	k := j
	for k >= 0 && (src[k] >= 'a' && src[k] <= 'z') {
		k--
	}
	switch src[k+1 : j+1] {
	case "return", "typeof", "case", "of", "in", "void", "delete", "throw":
		return true
	}
	return false
}

// skipRegex returns the index after the end of the regex literal that opens at i.
func skipRegex(src string, i int) int {
	class := false
	for j := i + 1; j < len(src) && src[j] != '\n'; j++ {
		switch {
		case src[j] == '\\':
			j++
		case src[j] == '[':
			class = true
		case src[j] == ']':
			class = false
		case src[j] == '/' && !class:
			return j + 1
		}
	}
	return i + 1
}

// jsLex returns the string literals and the comments of a script with their lines.
func jsLex(src string) (strs, comments []steUnit) {
	for i := 0; i < len(src); {
		switch c := src[i]; {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			j := i
			for j < len(src) && src[j] != '\n' {
				j++
			}
			comments = append(comments, steUnit{"", lineAt(src, i), src[i+2 : j]})
			i = j
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return
			}
			comments = append(comments, steUnit{"", lineAt(src, i), src[i+2 : i+2+end]})
			i += end + 4
		case c == '/' && regexStart(src, i):
			i = skipRegex(src, i)
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < len(src) && src[j] != c && (c == '`' || src[j] != '\n') {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			strs = append(strs, steUnit{"", lineAt(src, i), src[i+1 : min(j, len(src))]})
			i = j + 1
		default:
			i++
		}
	}
	return
}

// groupLineComments joins // comments on consecutive lines into one unit.
func groupLineComments(file string, cs []steUnit) []steUnit {
	var out []steUnit
	for _, c := range cs {
		text := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(c.Text), "*"))
		if strings.HasPrefix(text, "eslint") || strings.HasPrefix(text, "@ts-") {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Line+strings.Count(out[n-1].Text, "\x00")+1 == c.Line && !strings.Contains(c.Text, "\n") {
			out[n-1].Text += "\x00" + text
			continue
		}
		out = append(out, steUnit{file, c.Line, text})
	}
	for i := range out {
		out[i].Text = strings.ReplaceAll(out[i].Text, "\x00", " ")
		out[i].Text = regexp.MustCompile(`\n\s*\*?`).ReplaceAllString(out[i].Text, " ")
	}
	return out
}

var htmlAttrAt = regexp.MustCompile(`\b(?:title|placeholder|aria-label|alt|data-label|data-pb-tip)="([^"]*)"`)

// uiUnits are the strings a person reads: HTML text and display attributes, and JS string
// literals split at tags, template holes and newlines. Keys and classes come through too,
// but they have no sentences and none of the table's words.
func uiUnits(t *testing.T) []steUnit {
	t.Helper()
	var out []steUnit
	pages, _ := filepath.Glob(filepath.Join(repoRoot(), "web", "*.html"))
	for _, p := range pages {
		file := "web/" + filepath.Base(p)
		src := readRepo(t, file)
		src = htmlComment.ReplaceAllStringFunc(src, func(s string) string {
			return regexp.MustCompile(`[^\n]`).ReplaceAllString(s, " ")
		})
		for _, m := range htmlAttrAt.FindAllStringSubmatchIndex(src, -1) {
			out = append(out, steUnit{file, lineAt(src, m[2]), src[m[2]:m[3]]})
		}
		last := 0
		for _, m := range append(htmlTag.FindAllStringIndex(src, -1), []int{len(src), len(src)}) {
			if text := strings.TrimSpace(src[last:m[0]]); text != "" {
				out = append(out, steUnit{file, lineAt(src, last), strings.Join(strings.Fields(text), " ")})
			}
			last = m[1]
		}
	}
	scripts, _ := filepath.Glob(filepath.Join(repoRoot(), "web", "*.js"))
	for _, p := range scripts {
		file := "web/" + filepath.Base(p)
		strs, _ := jsLex(readRepo(t, file))
		for _, s := range strs {
			for _, part := range uiSplit.Split(s.Text, -1) {
				if part = strings.TrimSpace(part); part != "" {
					out = append(out, steUnit{file, s.Line, part})
				}
			}
		}
	}
	return out
}

func jsCommentUnits(t *testing.T) []steUnit {
	t.Helper()
	var out []steUnit
	for _, glob := range []string{"web/*.js", "web/test/*.mjs"} {
		files, _ := filepath.Glob(filepath.Join(repoRoot(), glob))
		for _, p := range files {
			rel, _ := filepath.Rel(repoRoot(), p)
			_, cs := jsLex(readRepo(t, filepath.ToSlash(rel)))
			out = append(out, groupLineComments(filepath.ToSlash(rel), cs)...)
		}
	}
	return out
}

var goCodeLine = regexp.MustCompile(`^(?:\t| {2,})`)

// goMessageText joins the string literals of a message argument; anything else (a name, a
// call) reads as one word, X.
func goMessageText(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.BasicLit:
		if v, err := strconv.Unquote(e.Value); err == nil && e.Kind == token.STRING {
			return regexp.MustCompile(`%[-+# 0]*\d*(?:\.\d+)?[a-zA-Z]`).ReplaceAllString(v, "X")
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return goMessageText(e.X) + goMessageText(e.Y)
		}
	case *ast.ParenExpr:
		return goMessageText(e.X)
	}
	return "X"
}

// goMessages returns the string arguments and constants of a file that read as prose: three
// words or more. Import paths and struct tags are not prose, and a message built with + is
// joined first, so its parts count as one string.
func goMessages(f *ast.File) []ast.Expr {
	var out []ast.Expr
	skip := map[ast.Node]bool{}
	for _, im := range f.Imports {
		skip[im.Path] = true
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			if n.Tag != nil {
				skip[n.Tag] = true
			}
		case *ast.BinaryExpr:
			if n.Op == token.ADD && !skip[n] {
				if strings.Count(goMessageText(n), " ") >= 2 {
					out = append(out, n)
				}
				ast.Inspect(n, func(m ast.Node) bool { skip[m] = true; return true })
				return false
			}
		case *ast.BasicLit:
			if n.Kind == token.STRING && !skip[n] && strings.Count(goMessageText(n), " ") >= 2 {
				out = append(out, n)
			}
		}
		return true
	})
	return out
}

// goUnits returns the comment groups of every Go file, and the messages of non-test files.
func goUnits(t *testing.T) (comments, messages []steUnit) {
	t.Helper()
	var out []steUnit
	fset := token.NewFileSet()
	err := filepath.WalkDir(repoRoot(), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") && p != repoRoot() || d.Name() == "vendor" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot(), p)
		for _, g := range f.Comments {
			var prose []string
			for l := range strings.SplitSeq(g.Text(), "\n") {
				if goCodeLine.MatchString(l) {
					continue
				}
				prose = append(prose, strings.TrimSpace(l))
			}
			text := strings.TrimSpace(strings.Join(prose, " "))
			if text != "" {
				out = append(out, steUnit{filepath.ToSlash(rel), fset.Position(g.Pos()).Line, text})
			}
		}
		if !strings.HasSuffix(p, "_test.go") {
			for _, e := range goMessages(f) {
				if text := strings.TrimSpace(goMessageText(e)); wordToken.MatchString(text) {
					messages = append(messages, steUnit{filepath.ToSlash(rel), fset.Position(e.Pos()).Line, text})
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out, messages
}

var (
	yamlProse  = regexp.MustCompile(`^(\s*)(?:summary|description):\s*(.*)$`)
	yamlIndent = regexp.MustCompile(`^(\s*)\S`)
)

// yamlUnits reads the summary and description values of an OpenAPI file, block scalars too.
func yamlUnits(file, src string) []steUnit {
	var out []steUnit
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		m := yamlProse.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		val := strings.TrimSpace(m[2])
		if val == "|" || val == ">" || val == "|-" || val == ">-" {
			var block []string
			for i+1 < len(lines) {
				ind := yamlIndent.FindStringSubmatch(lines[i+1])
				if ind != nil && len(ind[1]) <= len(m[1]) {
					break
				}
				i++
				block = append(block, strings.TrimSpace(lines[i]))
			}
			for _, u := range mdUnits(file, strings.Join(block, "\n")) {
				out = append(out, steUnit{file, i + 1, u.Text})
			}
			continue
		}
		val = strings.Trim(val, `"'`)
		if val != "" {
			out = append(out, steUnit{file, i + 1, val})
		}
	}
	return out
}

func sentences(text string) []string {
	var out []string
	last := 0
	for _, m := range sentenceEnd.FindAllStringIndex(text, -1) {
		out = append(out, text[last:m[1]])
		last = m[1]
	}
	if rest := strings.TrimSpace(text[last:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

func wordCount(s string) int {
	n := 0
	for f := range strings.FieldsSeq(s) {
		if wordToken.MatchString(f) {
			n++
		}
	}
	return n
}

func steAllowed(rule string, u steUnit, raw string) bool {
	return matchAllow("ste", steAllow, false, rule, u.File, raw)
}

// steCheck applies the word table and the sentence limit to one unit. raw is the text the
// allowlist sees; the scans read the cleaned text.
func steCheck(scope string, u steUnit, limit int) []steHit {
	var hits []steHit
	clean := steClean(u.Text)
	for _, w := range steWords {
		if m := w.Re.FindString(clean); m != "" && !steAllowed(w.ID, u, u.Text) {
			hits = append(hits, steHit{scope, u.File, w.ID, strings.TrimSpace(m), u.Line})
		}
	}
	for _, s := range sentences(clean) {
		if n := wordCount(s); n > limit && !steAllowed("length", u, u.Text) {
			hits = append(hits, steHit{scope, u.File, fmt.Sprintf("length %d>%d", n, limit), strings.TrimSpace(s), u.Line})
		}
	}
	return hits
}

func steScan(t *testing.T) []steHit {
	t.Helper()
	var hits []steHit
	docs, agent := steDocFiles(t)
	for scope, files := range map[string][]string{"docs": docs, "agentdocs": agent} {
		for _, f := range files {
			for _, u := range mdUnits(f, readRepo(t, f)) {
				hits = append(hits, steCheck(scope, u, steDescriptiveMax)...)
			}
		}
	}
	for _, u := range uiUnits(t) {
		hits = append(hits, steCheck("ui", u, steUIMax)...)
	}
	comments, messages := goUnits(t)
	for _, u := range comments {
		hits = append(hits, steCheck("gocomments", u, steDescriptiveMax)...)
	}
	for _, u := range messages {
		hits = append(hits, steCheck("gostrings", u, steUIMax)...)
	}
	for _, u := range yamlUnits("internal/app/openapi.yaml", readRepo(t, "internal/app/openapi.yaml")) {
		hits = append(hits, steCheck("docs", u, steDescriptiveMax)...)
	}
	for _, u := range jsCommentUnits(t) {
		hits = append(hits, steCheck("jscomments", u, steDescriptiveMax)...)
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return hits
}

func TestSTE(t *testing.T) {
	hits := steScan(t)
	counts := map[string]int{}
	var report strings.Builder
	for _, h := range hits {
		counts[h.Scope]++
		fmt.Fprintf(&report, "%s\t%s:%d\t%s\t%s\n", h.Scope, h.File, h.Line, h.Rule, h.Text)
		if steEnforced[h.Scope] {
			t.Errorf("%s:%d: %s: %q (docs/ste.md)", h.File, h.Line, h.Rule, h.Text)
		}
	}
	for scope, enforced := range steEnforced {
		if !enforced && counts[scope] > 0 {
			t.Logf("%s: %d hits (not enforced yet)", scope, counts[scope])
		}
	}
	if path := os.Getenv("STE_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range steAllow {
		if !allowUsed[allowKey("ste", a)] {
			t.Errorf("steAllow entry %+v exempts nothing; drop it", a)
		}
	}
}

func TestSTEMechanics(t *testing.T) {
	long := strings.Repeat("word ", 26) + "end."
	for text, want := range map[string]int{
		"Sode refuses the plan.":                  0,
		"Use `e.g.` in code only.":                0,
		"This is, e.g., wrong.":                   1,
		"You don't need it.":                      1,
		"The router's name stays.":                0,
		"It may fail.":                            1,
		long:                                      1,
		"Short one. " + strings.Repeat("w ", 24):  0,
		"See [the guide](https://x/e.g./etc.).":   0,
		"Make sure the destination is protected.": 0,
		`Write "do not", not "don't".`:            0,
		"It wasn’t sent.":                         1,
	} {
		if got := len(steCheck("x", steUnit{"a.md", 1, text}, steDescriptiveMax)); got != want {
			t.Errorf("%q: %d hits, want %d", text, got, want)
		}
	}
	strs, _ := jsLex("const re = /['\"]/g; const n = a / b; x('Late text.');")
	if len(strs) != 1 || strs[0].Text != "Late text." {
		t.Errorf("jsLex past a regex literal = %+v", strs)
	}
	src := "package p\nimport \"fmt\"\ntype T struct{ A int `json:\"a b c\"` }\n" +
		"func f(n string) error { return fmt.Errorf(\"no panel \"+n+\" that this key may use\") }\n" +
		"const short = \"two words\"\n"
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var msgs []string
	for _, e := range goMessages(f) {
		msgs = append(msgs, goMessageText(e))
	}
	if strings.Join(msgs, "|") != "no panel X that this key may use" {
		t.Errorf("goMessages = %q", msgs)
	}
	yu := yamlUnits("o.yaml", "paths:\n  /x:\n    get:\n      summary: List it.\n      description: |\n        One line\n        wraps.\n      responses: {}\n")
	if len(yu) != 2 || yu[0].Text != "List it." || yu[1].Text != "One line wraps." {
		t.Errorf("yamlUnits = %+v", yu)
	}
	units := mdUnits("a.md", "# Head\n\nOne line\nwraps here.\n- item one\n- item two\n```\ncode e.g.\n```\n| a | e.g. |\n")
	if len(units) != 3 || units[0].Text != "One line wraps here." || units[0].Line != 3 {
		t.Errorf("mdUnits = %+v", units)
	}
}

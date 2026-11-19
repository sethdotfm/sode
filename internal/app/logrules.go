package app

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// The /log page's filter, sort and words, for the files the server builds. Keep in step with
// web/logfilter.js, KINDS and WHY_LABEL in web/logkinds.js and KIND_LABEL in web/log.js.
// TestLogRulesMatchPage and web/test/log_filter.json check that they agree.

type logKindGroup struct {
	id    string
	kinds []string
}

// logKindGroups: KINDS in web/logkinds.js, in its order.
var logKindGroups = []logKindGroup{
	{"connect", []string{"connect", "salvoStage", "salvoStageExt", "salvoGo"}},
	{"interrogate", []string{"interrogate"}},
	{"keepalive", []string{"keepalive"}},
	{"dump", []string{"tallyDump", "protectDump"}},
	{"names", []string{"srcNames", "dstNames"}},
	{"other", []string{"other"}},
	{"browser", []string{"browser"}},
	{"command", []string{"command"}},
	{"api", []string{"api"}},
	{"link", []string{"link", "server"}},
	{"settings", []string{"settings"}},
}

// logKindLabel: KIND_LABEL in web/log.js.
var logKindLabel = map[string]string{
	"connect":     "Connect",
	"interrogate": "Interrogate",
	"keepalive":   "Keepalive",
	"tallyDump":   "Tally dump",
	"protectDump": "Protect dump",
	"srcNames":    "Source names",
	"dstNames":    "Destination names",
	"other":       "Other",
	"salvoStage":  "Router salvo stage",
	"salvoGo":     "Router salvo go",
	"browser":     "Browser",
	"command":     "Command",
	"api":         "API",
	"link":        "Router link",
	"server":      "Sode",
	"settings":    "Settings save",
}

// logWhyLabel: WHY_LABEL in web/logkinds.js. A why with no entry shows as written.
var logWhyLabel = map[string]string{
	"take":             "Take",
	"undo":             "Undo",
	"retry":            "Retry",
	"check":            "Check before take",
	"confirm":          "Confirm take",
	"resync":           "Resync",
	"names":            "Re-read names (timer)",
	"keepalive":        "Link check",
	"connect":          "WebSocket connected",
	"disconnect":       "WebSocket dropped",
	"rereadNames":      "Re-read names",
	"routerNames":      "Router names",
	"clearHistory":     "Clear history",
	"approvalRequest":  "Request approval",
	"stepIn":           "Step in",
	"stepOut":          "Step out",
	"config":           "Settings saved",
	"filterSets":       "Filter sets saved",
	"presets":          "Presets saved",
	"salvos":           "Presets saved",
	"salvo":            "Router salvo",
	"userSignIn":       "Sign in",
	"userLock":         "User locked",
	"userSignOut":      "User signed out",
	"adminUnlock":      "PIN sign-in (old rows)",
	"users":            "Users saved",
	"streamConnect":    "Stream connected",
	"streamDisconnect": "Stream dropped",
	"panelLost":        "Panel instance lost",
	"panelBack":        "Panel instance back",
	"panelAlarm":       "Panel alarm",
	"panelSetup":       "Panel instance set up",
	"assign":           "Instance assigned",
	"rename":           "Instance renamed",
	"reload":           "Panel page reloaded",
	"publicBack":       "Public panel page back",
	"save":             "Settings saved",
	"import":           "Import",
	"api":              "Settings saved through the API",
	"setup":            "Setup finished",
	"setupClaim":       "Setup take control",
	"setupRouter":      "Setup router address",
	"setupFinish":      "Setup finished",
	"inboxSend":        "Copy sent",
	"inboxAccept":      "Copy accepted",
	"inboxDecline":     "Copy declined",
	"inboxWithdraw":    "Copy withdrawn",
	"inboxExpired":     "Copy expired",
	"download":         "Communication Log downloaded",
}

// logEventKinds: EVENT_KINDS in web/logkinds.js. Event rows have no router message.
var logEventKinds = map[string]bool{"browser": true, "command": true, "api": true, "link": true, "server": true, "settings": true}

// logEventProblem: EVENT_PROBLEM in web/logkinds.js.
var logEventProblem = map[string][]string{
	"command": {"refused", "locked", "error", "fallback"},
	"api":     {"refused", "error"},
	"link":    {"down"},
}

func logProblem(e Sent) bool {
	if !logEventKinds[e.Kind] {
		return e.Result != "ack" || e.Tries > 1
	}
	if e.Kind == kindServer {
		return e.Err != ""
	}
	return slices.Contains(logEventProblem[e.Kind], e.Result)
}

// logFilter: the page's filters. Kinds are chip ids; From and To are in ms, To counts to its own
// end. Src and Dst use the History filter language.
type logFilter struct {
	Kinds    []string `json:"kinds"`
	Problems bool     `json:"problems"`
	Src      string   `json:"src"`
	Dst      string   `json:"dst"`
	From     *int64   `json:"from"`
	To       *int64   `json:"to"`
}

// numMatch: a compiled Source or Destination filter. num is the number as the page shows it.
type numMatch func(num int, name string) bool

// compile: the page's passes(), for numbers shown with off added.
func (f logFilter) compile(off int) (func(Sent) bool, error) {
	on := map[string]bool{}
	for _, g := range logKindGroups {
		if slices.Contains(f.Kinds, g.id) {
			for _, k := range g.kinds {
				on[k] = true
			}
		}
	}
	if len(on) == 0 { // as normalizeKinds: no known kind means All
		for _, g := range logKindGroups {
			for _, k := range g.kinds {
				on[k] = true
			}
		}
	}
	grouped := func(k string) bool {
		for _, g := range logKindGroups {
			if slices.Contains(g.kinds, k) {
				return true
			}
		}
		return false
	}
	src, err := compileNumFilter(f.Src, "Source")
	if err != nil {
		return nil, err
	}
	dst, err := compileNumFilter(f.Dst, "Destination")
	if err != nil {
		return nil, err
	}
	return func(e Sent) bool {
		t := e.Time.UnixMilli()
		if f.From != nil && t < *f.From || f.To != nil && t > *f.To {
			return false
		}
		k := e.Kind
		if !grouped(k) {
			k = "other"
		}
		if !on[k] {
			return false
		}
		if f.Problems && !logProblem(e) {
			return false
		}
		if src != nil && (e.Src < 0 || !src(e.Src+off, e.SrcName)) {
			return false
		}
		if dst != nil && (e.Dst < 0 || !dst(e.Dst+off, e.DstName)) {
			return false
		}
		return true
	}, nil
}

// compileNumFilter: compileFilter in web/filter.js with fuzzy off and no router. nil for blank text.
func compileNumFilter(text, side string) (numMatch, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	terms, ok := filterTerms(text)
	if !ok {
		return nil, fmt.Errorf("the %s filter is not valid", side)
	}
	r := []rune(text)
	var inc, exc []numMatch
	for _, t := range terms {
		m, err := termMatch(t, string(r[t.at:t.end]))
		if err != nil {
			return nil, fmt.Errorf("the %s filter: %v", side, err)
		}
		if t.Not {
			exc = append(exc, m)
		} else {
			inc = append(inc, m)
		}
	}
	return func(num int, name string) bool {
		if len(inc) > 0 && !slices.ContainsFunc(inc, func(m numMatch) bool { return m(num, name) }) {
			return false
		}
		return !slices.ContainsFunc(exc, func(m numMatch) bool { return m(num, name) })
	}, nil
}

// termMatch: span is the term's text as typed, quotes and slashes included.
func termMatch(t filterTerm, span string) (numMatch, error) {
	switch t.Kind {
	case "num":
		return func(num int, _ string) bool { return num == t.A }, nil
	case "range":
		var a, b int
		m := filterRange.FindStringSubmatch(span)
		fmt.Sscan(m[1], &a)
		fmt.Sscan(m[2], &b)
		a, b = min(a, b), max(a, b)
		return func(num int, _ string) bool { return num >= a && num <= b }, nil
	case "re":
		end := strings.LastIndex(span, "/")
		re, err := jsRegexp(span[1:end], span[end+1:])
		if err != nil {
			return nil, err
		}
		return func(_ int, name string) bool { return re.MatchString(name) }, nil
	case "exact":
		want := strings.ToLower(t.Text)
		return func(_ int, name string) bool { return strings.ToLower(name) == want }, nil
	}
	want := strings.ToLower(t.Text)
	return func(_ int, name string) bool { return strings.Contains(strings.ToLower(name), want) }, nil
}

// jsRegexp: new RegExp(pattern, flags without g and y, else 'i'). Go has no lookaround or
// backreferences, so such a pattern is refused, not matched wrongly.
func jsRegexp(pattern, flags string) (*regexp.Regexp, error) {
	flags = strings.NewReplacer("g", "", "y", "").Replace(flags)
	if flags == "" {
		flags = "i"
	}
	var goFlags string
	for _, c := range "ims" {
		if strings.ContainsRune(flags, c) {
			goFlags += string(c)
		}
	}
	if goFlags != "" {
		pattern = "(?" + goFlags + ")" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("/%s/ works on the page but Sode cannot run it on the server", pattern)
	}
	return re, nil
}

// logSort: the page's sort. Col is a data-sort value of log.html; Dir is asc or desc.
type logSort struct {
	Col string `json:"col"`
	Dir string `json:"dir"`
}

var logSortCols = map[string]bool{"time": true, "message": true, "level": true, "dst": true, "src": true, "why": true, "result": true, "hex": true}

// logRowPos: rowPos in web/logpager.js. A Settings saves row sits below its anchor.
func logRowPos(e Sent) float64 {
	if e.Kind == kindSettings {
		return float64(e.Seq) - 0.5
	}
	return float64(e.Seq)
}

// logNewer: byNewest in web/logpager.js, as a less function.
func logNewer(a, b Sent) int {
	if d := logRowPos(b) - logRowPos(a); d != 0 {
		if d > 0 {
			return 1
		}
		return -1
	}
	return int(b.ID - a.ID)
}

func logWhyText(why string) string {
	if l, ok := logWhyLabel[why]; ok {
		return l
	}
	return why
}

func logKindText(kind string) string {
	if l, ok := logKindLabel[kind]; ok {
		return l
	}
	return kind
}

// logSortKey: SORTS in web/logfilter.js. nil is a row without the column. Each value is a
// float64, a string or a []any of those.
func logSortKey(col string, e Sent) any {
	switch col {
	case "time":
		return logRowPos(e)
	case "message":
		return logKindText(e.Kind)
	case "level":
		if e.Level != 0 {
			return float64(e.Level)
		}
		if e.Wire >= 0 {
			return float64(1000 + e.Wire)
		}
		return nil
	case "dst", "src":
		n := e.Dst
		if col == "src" {
			n = e.Src
		}
		if n < 0 {
			return nil
		}
		return float64(n)
	case "why":
		if e.Why == "" {
			return nil
		}
		return logWhyText(e.Why) + " " + e.By
	case "result":
		rank := 2.0
		switch {
		case logEventKinds[e.Kind] && !logProblem(e):
			rank = 0
		case logEventKinds[e.Kind]:
		case e.Result == "ack" && e.Tries > 1:
			rank = 1
		case e.Result == "ack":
			rank = 0
		}
		return []any{rank, e.Result, float64(e.Tries)}
	case "hex":
		return e.Hex
	}
	return nil
}

func logCmpVal(a, b any) int {
	switch x := a.(type) {
	case []any:
		y := b.([]any)
		for i := range x {
			if c := logCmpVal(x[i], y[i]); c != 0 {
				return c
			}
		}
		return 0
	case string:
		return naturalCmp(x, b.(string))
	case float64:
		y := b.(float64)
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
	}
	return 0
}

// sortLogRows puts rows in the page's order (shownRows in web/log.js).
func sortLogRows(rows []Sent, s logSort) {
	if !logSortCols[s.Col] || s.Col == "time" && s.Dir != "asc" {
		sort.SliceStable(rows, func(i, j int) bool { return logNewer(rows[i], rows[j]) < 0 })
		return
	}
	sign := 1
	if s.Dir != "asc" {
		sign = -1
	}
	keys := make([]any, len(rows))
	for i, e := range rows {
		keys[i] = logSortKey(s.Col, e)
	}
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		x, y := idx[i], idx[j]
		xv, yv := keys[x], keys[y]
		if xv == nil || yv == nil {
			if xv == nil && yv == nil {
				return logNewer(rows[x], rows[y]) < 0
			}
			return yv == nil
		}
		if c := sign * logCmpVal(xv, yv); c != 0 {
			return c < 0
		}
		return logNewer(rows[x], rows[y]) < 0
	})
	out := make([]Sent, len(rows))
	for i, k := range idx {
		out[i] = rows[k]
	}
	copy(rows, out)
}

// naturalCmp is near localeCompare with numeric and base sensitivity. It ignores case and compares
// digit runs as numbers. Spaces and punctuation come before digits, and digits before letters.
func naturalCmp(a, b string) int {
	x, y := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	class := func(r rune) int {
		switch {
		case unicode.IsDigit(r):
			return 1
		case unicode.IsLetter(r):
			return 2
		}
		return 0
	}
	digits := func(s []rune, i int) (string, int) {
		j := i
		for j < len(s) && unicode.IsDigit(s[j]) {
			j++
		}
		return strings.TrimLeft(string(s[i:j]), "0"), j
	}
	i, j := 0, 0
	for i < len(x) && j < len(y) {
		if class(x[i]) == 1 && class(y[j]) == 1 {
			var dx, dy string
			dx, i = digits(x, i)
			dy, j = digits(y, j)
			if len(dx) != len(dy) {
				return cmpInt(len(dx), len(dy))
			}
			if c := strings.Compare(dx, dy); c != 0 {
				return c
			}
			continue
		}
		if cx, cy := class(x[i]), class(y[j]); cx != cy {
			return cmpInt(cx, cy)
		}
		if x[i] != y[j] {
			return cmpInt(int(x[i]), int(y[j]))
		}
		i++
		j++
	}
	return cmpInt(len(x)-i, len(y)-j)
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

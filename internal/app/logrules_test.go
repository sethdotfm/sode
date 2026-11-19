package app

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// web/test/logfilter.test.mjs runs the same cases against web/logfilter.js.
func TestLogFilterCases(t *testing.T) {
	b, err := os.ReadFile("../../web/test/log_filter.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Offset int    `json:"offset"`
		Rows   []Sent `json:"rows"`
		Cases  []struct {
			Name   string    `json:"name"`
			Filter logFilter `json:"filter"`
			Sort   *logSort  `json:"sort"`
			Want   []string  `json:"want"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	for _, c := range data.Cases {
		pass, err := c.Filter.compile(data.Offset)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		var rows []Sent
		for _, e := range data.Rows {
			if pass(e) {
				rows = append(rows, e)
			}
		}
		s := logSort{Col: "time", Dir: "desc"}
		if c.Sort != nil {
			s = *c.Sort
		}
		sortLogRows(rows, s)
		got := []string{}
		for _, e := range rows {
			k := strconv.FormatInt(e.Seq, 10)
			if e.Kind == kindSettings {
				k = "s" + strconv.FormatInt(e.ID, 10)
			}
			got = append(got, k)
		}
		if !slices.Equal(got, c.Want) {
			t.Errorf("%s: %v, want %v", c.Name, got, c.Want)
		}
	}
}

// jsObject reads `name = { key: 'text', ... }` from a web module.
func jsObject(t *testing.T, file, name string) map[string]string {
	t.Helper()
	b, err := os.ReadFile("../../web/" + file)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, name+" = {")
	if start < 0 {
		t.Fatalf("no %s in %s", name, file)
	}
	end := strings.Index(src[start:], "};")
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`(\w+): '([^']*)'`).FindAllStringSubmatch(src[start:start+end], -1) {
		out[m[1]] = m[2]
	}
	return out
}

// The server's copy of the page's words and kind groups. The filter rules themselves are checked by
// TestLogFilterCases.
func TestLogRulesMatchPage(t *testing.T) {
	if page := jsObject(t, "log.js", "KIND_LABEL"); !reflect.DeepEqual(page, logKindLabel) {
		t.Errorf("KIND_LABEL in log.js %v\nlogKindLabel %v", page, logKindLabel)
	}
	if page := jsObject(t, "logkinds.js", "WHY_LABEL"); !reflect.DeepEqual(page, logWhyLabel) {
		t.Errorf("WHY_LABEL in logkinds.js %v\nlogWhyLabel %v", page, logWhyLabel)
	}
	b, err := os.ReadFile("../../web/logkinds.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	var page []logKindGroup
	for _, m := range regexp.MustCompile(`\{ id: '(\w+)', label: '[^']*', kinds: \[([^\]]*)\] \}`).FindAllStringSubmatch(src, -1) {
		g := logKindGroup{id: m[1]}
		for _, k := range strings.Split(m[2], ",") {
			g.kinds = append(g.kinds, strings.Trim(strings.TrimSpace(k), "'"))
		}
		page = append(page, g)
	}
	if !reflect.DeepEqual(page, logKindGroups) {
		t.Errorf("KINDS in logkinds.js %v\nlogKindGroups %v", page, logKindGroups)
	}
	m := regexp.MustCompile(`EVENT_KINDS = new Set\(\[([^\]]*)\]\)`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no EVENT_KINDS in logkinds.js")
	}
	events := map[string]bool{}
	for _, k := range strings.Split(m[1], ",") {
		events[strings.Trim(strings.TrimSpace(k), "'")] = true
	}
	if !reflect.DeepEqual(events, logEventKinds) {
		t.Errorf("EVENT_KINDS %v\nlogEventKinds %v", events, logEventKinds)
	}
	m = regexp.MustCompile(`EVENT_PROBLEM = \{([^}]*)\}`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no EVENT_PROBLEM in logkinds.js")
	}
	problems := map[string][]string{}
	for _, p := range regexp.MustCompile(`(\w+): \[([^\]]*)\]`).FindAllStringSubmatch(m[1], -1) {
		for _, r := range strings.Split(p[2], ",") {
			problems[p[1]] = append(problems[p[1]], strings.Trim(strings.TrimSpace(r), "'"))
		}
	}
	if !reflect.DeepEqual(problems, logEventProblem) {
		t.Errorf("EVENT_PROBLEM %v\nlogEventProblem %v", problems, logEventProblem)
	}
}

// A regex the page can run but Go cannot is refused, never matched wrongly.
func TestLogFilterRefusesALookahead(t *testing.T) {
	if _, err := (logFilter{Dst: "/^(?!MON)/"}).compile(0); err == nil {
		t.Fatal("a lookahead compiled")
	}
	if _, err := (logFilter{Dst: "/mon/u"}).compile(0); err != nil {
		t.Fatalf("a u flag: %v", err)
	}
}

func TestNaturalCmp(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"CAM 2", "cam 10", -1},
		{"Take Ana", "Take 10.0.0.5", 1},
		{"", "01 00", -1},
		{"a b", "ab", -1},
		{"x", "X", 0},
	} {
		if got := naturalCmp(c.a, c.b); got != c.want {
			t.Errorf("naturalCmp(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

package app

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// filterTerm is one term of a filter set's text, read as parseFilter in web/filter.js reads it. Keep in step
// (web/test/filter_terms.json is shared by both tests). Only the kinds a Broken check needs are kept apart.
type filterTerm struct {
	Not  bool   `json:"not"`
	Kind string `json:"kind"` // "num", "range", "exact", "name" (contains) or "re"
	A    int    `json:"a,omitempty"`
	Text string `json:"text,omitempty"`
}

var (
	filterNum   = regexp.MustCompile(`^\d+$`)
	filterRange = regexp.MustCompile(`(?i)^(\d+)\s*(?:-|\s(?:thru|through|to)\s)\s*(\d+)$`)
	filterNot   = regexp.MustCompile(`(?i)^not\s+['"/\d]`)
)

// filterTerms returns nil, false for text that does not parse; such a set is left to the page.
func filterTerms(text string) ([]filterTerm, bool) {
	r := []rune(text)
	n := len(r)
	i := 0
	ws := func() {
		for i < n && unicode.IsSpace(r[i]) {
			i++
		}
	}
	var out []filterTerm
	for i < n {
		ws()
		if i >= n {
			break
		}
		if r[i] == ',' {
			i++
			continue
		}
		t := filterTerm{}
		if r[i] == '!' {
			t.Not = true
			i++
			ws()
		} else if filterNot.MatchString(string(r[i:])) {
			t.Not = true
			i += 3
			ws()
		}
		switch {
		case i < n && (r[i] == '"' || r[i] == '\''):
			q := r[i]
			var s strings.Builder
			i++
			for {
				if i >= n {
					return nil, false
				}
				if r[i] == q {
					if i+1 < n && r[i+1] == q {
						s.WriteRune(q)
						i += 2
						continue
					}
					i++
					break
				}
				s.WriteRune(r[i])
				i++
			}
			if s.Len() == 0 {
				return nil, false
			}
			t.Text, t.Kind = s.String(), "name"
			if q == '"' {
				t.Kind = "exact"
			}
		case i < n && r[i] == '/':
			i++
			for {
				if i >= n {
					return nil, false
				}
				if r[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if r[i] == '/' {
					i++
					break
				}
				i++
			}
			for i < n && (r[i] >= 'a' && r[i] <= 'z' || r[i] >= 'A' && r[i] <= 'Z') {
				i++
			}
			t.Kind = "re"
		default:
			end := i
			for end < n && r[end] != ',' {
				end++
			}
			s := strings.TrimSpace(string(r[i:end]))
			i = end
			if s == "" {
				return nil, false
			}
			if filterNum.MatchString(s) {
				t.Kind = "num"
				t.A, _ = strconv.Atoi(s)
			} else if filterRange.MatchString(s) {
				t.Kind = "range"
			} else {
				t.Kind, t.Text = "name", s
			}
		}
		ws()
		if i < n && r[i] != ',' {
			return nil, false
		}
		out = append(out, t)
	}
	return out, true
}

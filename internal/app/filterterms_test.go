package app

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// web/test/filterterms.test.mjs checks the same cases against parseFilter.
func TestFilterTermsMatchFilterJS(t *testing.T) {
	b, err := os.ReadFile("../../web/test/filter_terms.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Text  string        `json:"text"`
		Terms *[]filterTerm `json:"terms"`
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		got, ok := filterTerms(c.Text)
		if c.Terms == nil {
			if ok {
				t.Errorf("%q parsed: %+v", c.Text, got)
			}
			continue
		}
		if got == nil {
			got = []filterTerm{}
		}
		for i := range got {
			got[i].at, got[i].end = 0, 0
		}
		if !ok || !reflect.DeepEqual(got, *c.Terms) {
			t.Errorf("%q: %+v, want %+v", c.Text, got, *c.Terms)
		}
	}
}

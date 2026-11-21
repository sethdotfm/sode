package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseRefusesReferencesWithNoRouter(t *testing.T) {
	head := "format: 2\nrouters:\n  - uid: 2c5bb739e7cf2f2a\n    address: 127.0.0.1:8910\n"
	for name, body := range map[string]string{
		"preset src no router":   "presets:\n  - id: p1\n    name: Show\n    items:\n      - src: {n: 4}\n        dsts: [{router: 2c5bb739e7cf2f2a, n: 1}]\n",
		"preset dst no router":   "presets:\n  - id: p1\n    name: Show\n    items:\n      - src: {router: 2c5bb739e7cf2f2a, n: 4}\n        dsts: [{n: 1}]\n",
		"preset src number":      "presets:\n  - id: p1\n    name: Show\n    items:\n      - src: 4\n        dsts: []\n",
		"guest rights no router": "guestRights:\n  sources: [{n: 1}]\n",
		"guest rights numbers":   "guestRights:\n  dsts: [1, 2]\n",
		"client rights":          "apiClients:\n  - name: Bot\n    rights:\n      dsts: [{n: 2}]\n",
		"user rights":            "users:\n  - id: u1\n    name: Ana\n    rights:\n      sources: [{n: 1}]\n",
		"profile preset":         "users:\n  - id: u1\n    name: Ana\n    profile:\n      presets:\n        - id: p2\n          name: Mine\n          items:\n            - src: {n: 1}\n              dsts: []\n",
	} {
		if _, err := Parse([]byte(head + body)); err == nil {
			t.Errorf("%s: loaded", name)
		} else if !strings.Contains(err.Error(), "router") && !strings.Contains(err.Error(), "unmarshal") {
			t.Errorf("%s: %v", name, err)
		}
	}
	ok := head + "presets:\n  - id: p1\n    name: Show\n    items:\n      - src: {router: 2c5bb739e7cf2f2a, n: 4}\n        dsts: [{router: 2c5bb739e7cf2f2a, n: 1}]\n" +
		"guestRights:\n  sources: [{router: 2c5bb739e7cf2f2a, n: 1}]\n"
	if _, err := Parse([]byte(ok)); err != nil {
		t.Fatalf("references with their router: %v", err)
	}
}

func TestRefReadsNoPlainNumber(t *testing.T) {
	var r Ref
	if err := json.Unmarshal([]byte(`4`), &r); err == nil {
		t.Fatalf("a plain number read as %+v", r)
	}
	if err := json.Unmarshal([]byte(`{"n":4}`), &r); err != nil || r.N != 4 || r.Router != "" {
		t.Fatalf("{n}: %+v %v", r, err)
	}
}

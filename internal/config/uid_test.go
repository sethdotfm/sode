package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGeneratedIDsNeverParseAsNumber(t *testing.T) {
	for range 20000 {
		for _, id := range []string{newRouterUID(), NewPresetID(), RandomHexID(4)} {
			var n yaml.Node
			if err := yaml.Unmarshal([]byte(id), &n); err != nil {
				t.Fatal(err)
			}
			if tag := n.Content[0].Tag; tag != "!!str" {
				t.Fatalf("%q parses as %s", id, tag)
			}
		}
	}
}

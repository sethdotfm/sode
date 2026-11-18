package app

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// In a flow mapping, a plain scalar ends at the first comma. An unquoted description with a comma
// leaves its tail as a key with no value, and the served spec shows the description cut short.
func TestOpenAPIFlowMappingsKeepWholeValues(t *testing.T) {
	var doc yaml.Node
	if err := yaml.Unmarshal(openAPIFile, &doc); err != nil {
		t.Fatal(err)
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode && n.Style&yaml.FlowStyle != 0 {
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				if v.Tag == "!!null" && v.Value == "" {
					t.Errorf("openapi.yaml:%d: %q has no value; quote the description before it", k.Line, k.Value)
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&doc)
}

package app

import (
	"encoding"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// The export file's keys are the yaml tags of the Go types that read it. Free-form values (map[string]any,
// cells, settings) and types that read themselves (yaml.Unmarshaler) are not walked.

var (
	yamlUnmarshalerType = reflect.TypeFor[yaml.Unmarshaler]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
	timeType            = reflect.TypeFor[time.Time]()
)

// yamlFields maps each yaml key of struct t to its field type, with inline structs merged in. An inline
// map is the unknown keys and names nothing.
func yamlFields(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for i := range t.NumField() {
		f := t.Field(i)
		name, opts, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		if slices.Contains(strings.Split(opts, ","), "inline") {
			if ft := f.Type; ft.Kind() == reflect.Struct {
				for k, v := range yamlFields(ft) {
					out[k] = v
				}
			}
			continue
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		out[name] = f.Type
	}
	return out
}

func readsItself(t reflect.Type) bool {
	return t == timeType || t.Implements(yamlUnmarshalerType) || reflect.PointerTo(t).Implements(yamlUnmarshalerType) ||
		t.Implements(textUnmarshalerType) || reflect.PointerTo(t).Implements(textUnmarshalerType)
}

func joinPath(p, k string) string {
	if p == "" {
		return k
	}
	return p + "." + k
}

// out nil: the paths are not listed.
func pruneUnknown(n *yaml.Node, t reflect.Type, path string, out *[]string) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if n.Kind == yaml.DocumentNode {
		for _, c := range n.Content {
			pruneUnknown(c, t, path, out)
		}
		return
	}
	if readsItself(t) {
		return
	}
	switch t.Kind() {
	case reflect.Struct:
		if n.Kind == yaml.MappingNode {
			pruneMapping(n, yamlFields(t), path, out)
		}
	case reflect.Slice, reflect.Array:
		if n.Kind == yaml.SequenceNode {
			for i, c := range n.Content {
				pruneUnknown(c, t.Elem(), fmt.Sprintf("%s[%d]", path, i), out)
			}
		}
	case reflect.Map:
		if n.Kind == yaml.MappingNode && t.Key().Kind() == reflect.String {
			for i := 0; i+1 < len(n.Content); i += 2 {
				pruneUnknown(n.Content[i+1], t.Elem(), joinPath(path, n.Content[i].Value), out)
			}
		}
	}
}

func pruneMapping(n *yaml.Node, fields map[string]reflect.Type, path string, out *[]string) {
	for i := 0; i+1 < len(n.Content); {
		k := n.Content[i].Value
		ft, ok := fields[k]
		if !ok {
			if out != nil {
				*out = append(*out, joinPath(path, k))
			}
			n.Content = slices.Delete(n.Content, i, i+2)
			continue
		}
		pruneUnknown(n.Content[i+1], ft, joinPath(path, k), out)
		i += 2
	}
}

// pruneSettings is pruneUnknown for settings.yaml: sections, each with the config keys exportSections lists.
func pruneSettings(n *yaml.Node, out *[]string) {
	if n.Kind == yaml.DocumentNode {
		for _, c := range n.Content {
			pruneSettings(c, out)
		}
		return
	}
	if n.Kind != yaml.MappingNode {
		return
	}
	cfgFields := yamlFields(reflect.TypeFor[config.Config]())
	for i := 0; i+1 < len(n.Content); {
		name, sec := n.Content[i].Value, n.Content[i+1]
		j := slices.IndexFunc(exportSections, func(x exportSection) bool { return x.Name == name })
		if j < 0 {
			if out != nil {
				*out = append(*out, "settings."+name)
			}
			n.Content = slices.Delete(n.Content, i, i+2)
			continue
		}
		if sec.Kind == yaml.MappingNode {
			fields := map[string]reflect.Type{}
			for _, k := range exportSections[j].Keys {
				fields[k] = cfgFields[k]
			}
			pruneMapping(sec, fields, "settings."+name, out)
		}
		i += 2
	}
}

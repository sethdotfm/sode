package config

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"

	"gopkg.in/yaml.v3"
)

// RouterLevels is Rights `levels`: UI levels 1..N, under the UID of the router they belong to. A router the map does
// not name is not limited, so a limit on router A does not limit router B. A nil map limits nothing, and an empty
// list allows no level on its router.
//
// A request can send a plain list, which names the one router. It reads under the UID "", and Normalize fills it, as
// it fills a Ref with no router. A file must name the router (noRouterProblem).
type RouterLevels map[string][]int

func (m *RouterLevels) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		*m = nil
		return nil
	}
	var l []int
	if err := json.Unmarshal(b, &l); err == nil {
		*m = RouterLevels{"": nonNil(l)}
		return nil
	}
	var by map[string][]int
	if err := json.Unmarshal(b, &by); err != nil {
		return err
	}
	*m = filled(by)
	return nil
}

func (m *RouterLevels) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.SequenceNode {
		var l []int
		if err := n.Decode(&l); err != nil {
			return err
		}
		*m = RouterLevels{"": nonNil(l)}
		return nil
	}
	var by map[string][]int
	if err := n.Decode(&by); err != nil {
		return err
	}
	*m = filled(by)
	return nil
}

func nonNil(l []int) []int {
	if l == nil {
		return []int{}
	}
	return l
}

// filled: a router with a null list allows no level, as an empty one does.
func filled(by map[string][]int) RouterLevels {
	if by == nil {
		return nil
	}
	out := make(RouterLevels, len(by))
	for uid, l := range by {
		out[uid] = nonNil(l)
	}
	return out
}

// Allows: uid is the router of the level. A router with no list is not limited.
func (m RouterLevels) Allows(uid string, ui int) bool {
	l, ok := m[uid]
	return !ok || slices.Contains(l, ui)
}

// within: every router that o limits, m limits to a subset of o's list.
func (m RouterLevels) within(o RouterLevels) bool {
	for uid, ol := range o {
		l, ok := m[uid]
		if !ok || !subset(&l, &ol) {
			return false
		}
	}
	return true
}

func (m RouterLevels) intersect(o RouterLevels) RouterLevels {
	switch {
	case m == nil:
		return o.Clone()
	case o == nil:
		return m.Clone()
	}
	out := m.Clone()
	for uid, ol := range o {
		if l, ok := out[uid]; ok {
			out[uid] = *intersectBy(&l, &ol, eq[int])
		} else {
			out[uid] = nonNil(slices.Clone(ol))
		}
	}
	return out
}

// Clone: nil stays nil.
func (m RouterLevels) Clone() RouterLevels {
	if m == nil {
		return nil
	}
	out := make(RouterLevels, len(m))
	for uid, l := range m {
		out[uid] = nonNil(slices.Clone(l))
	}
	return out
}

// withRouter puts the list of the blank UID under uid, joined to any list uid has. It returns a copy, since stored
// Rights share their maps.
func (m RouterLevels) withRouter(uid string) RouterLevels {
	blank, ok := m[""]
	if !ok {
		return m
	}
	out := maps.Clone(m)
	delete(out, "")
	if l, has := out[uid]; has {
		blank = append(slices.Clone(l), blank...)
		slices.Sort(blank)
		blank = slices.Compact(blank)
	}
	out[uid] = nonNil(blank)
	return out
}

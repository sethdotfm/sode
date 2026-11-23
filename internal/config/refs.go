package config

import "slices"

// RefWalk visits router references. Fn answers what goes in a slot that holds r. Nil keeps r, an empty list drops r
// from a list, and a slot for one reference takes the first. Dedupe drops a later entry for the same number from a
// list where Fn answered.
// What changes is a copy, since stored items share their lists.
// Tie visits the tie line ids of a preset item: nil keeps the id, an empty list drops it, else the list takes its
// place. A walk with no Tie leaves the ids as they are.
type RefWalk struct {
	Fn     func(kind string, r Ref) []Ref
	Tie    func(id string) []string
	Dedupe bool
}

func (w RefWalk) One(kind string, r Ref) Ref {
	if out := w.Fn(kind, r); len(out) > 0 {
		return out[0]
	}
	return r
}

// List is l itself when nothing answered; nil stays nil.
func (w RefWalk) List(kind string, l []Ref) []Ref {
	out, _ := w.list(kind, l)
	return out
}

func (w RefWalk) list(kind string, l []Ref) ([]Ref, bool) {
	if l == nil {
		return nil, false
	}
	out, answered := make([]Ref, 0, len(l)), false
	for _, r := range l {
		if got := w.Fn(kind, r); got != nil {
			out, answered = append(out, got...), true
		} else {
			out = append(out, r)
		}
	}
	if !answered {
		return l, false
	}
	if w.Dedupe {
		out = DedupeRefs(out)
	}
	return out, true
}

// DedupeRefs drops a later reference to the same number.
func DedupeRefs(l []Ref) []Ref {
	out := l[:0:0]
	for _, r := range l {
		if !slices.ContainsFunc(out, func(o Ref) bool { return o.Router == r.Router && o.N == r.N }) {
			out = append(out, r)
		}
	}
	return out
}

// Preset visits sources and destinations; levels are not references.
func (w RefWalk) Preset(p *Preset) {
	p.Items = slices.Clone(p.Items)
	for i := range p.Items {
		w.Item(&p.Items[i])
	}
}

func (w RefWalk) Item(it *PresetItem) {
	it.Src, it.Dsts = w.One(MissingSource, it.Src), w.List(MissingDestination, it.Dsts)
	if w.Tie == nil || it.Ties == nil {
		return
	}
	out := make([]string, 0, len(it.Ties))
	for _, id := range it.Ties {
		if got := w.Tie(id); got != nil {
			out = append(out, got...)
		} else {
			out = append(out, id)
		}
	}
	it.Ties = out
}

// TieLine visits the upstream end (a destination), then the downstream end (a source).
func (w RefWalk) TieLine(t *TieLine) {
	t.From, t.To = w.One(MissingDestination, t.From), w.One(MissingSource, t.To)
}

func (w RefWalk) Rights(r *Rights) {
	for _, s := range []struct {
		l    **[]Ref
		kind string
	}{{&r.Sources, MissingSource}, {&r.Dsts, MissingDestination}} {
		if *s.l == nil {
			continue
		}
		if l, changed := w.list(s.kind, **s.l); changed {
			*s.l = &l
		}
	}
}

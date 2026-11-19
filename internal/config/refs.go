package config

import "slices"

// RefWalk visits router references. Fn answers what goes in a slot that holds r: nil keeps r, an empty list drops r
// from a list, and a slot for one reference takes the first. Dedupe drops a later list entry for the same number.
// What changes is a copy, since stored items share their lists.
type RefWalk struct {
	Fn     func(kind string, r Ref) []Ref
	Dedupe bool
}

func (w RefWalk) One(kind string, r Ref) Ref {
	if out := w.Fn(kind, r); len(out) > 0 {
		return out[0]
	}
	return r
}

// List is l itself when nothing answered and Dedupe is off; nil stays nil.
func (w RefWalk) List(kind string, l []Ref) []Ref {
	if l == nil {
		return nil
	}
	out, answered := make([]Ref, 0, len(l)), false
	for _, r := range l {
		got := w.Fn(kind, r)
		if got == nil {
			got = []Ref{r}
		} else {
			answered = true
		}
		for _, x := range got {
			if !w.Dedupe || !slices.ContainsFunc(out, func(y Ref) bool { return y.Router == x.Router && y.N == x.N }) {
				out = append(out, x)
			}
		}
	}
	if !answered && !w.Dedupe {
		return l
	}
	return out
}

// Preset visits sources and destinations; levels are not references.
func (w RefWalk) Preset(p *Preset) {
	p.Items = slices.Clone(p.Items)
	for i := range p.Items {
		it := &p.Items[i]
		it.Src, it.Dsts = w.One(MissingSource, it.Src), w.List(MissingDestination, it.Dsts)
	}
}

func (w RefWalk) Rights(r *Rights) {
	for _, s := range []struct {
		l    **[]Ref
		kind string
	}{{&r.Sources, MissingSource}, {&r.Dsts, MissingDestination}} {
		if *s.l != nil {
			l := w.List(s.kind, **s.l)
			*s.l = &l
		}
	}
}

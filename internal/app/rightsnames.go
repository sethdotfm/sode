package app

import (
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

func everyRights(c *config.Config) []*config.Rights {
	out := []*config.Rights{&c.GuestRights}
	for i := range c.Users {
		out = append(out, &c.Users[i].Rights)
	}
	for i := range c.APIClients {
		out = append(out, &c.APIClients[i].Rights)
	}
	return out
}

// Browsers and API callers send {n} with no name. With no router name the stored name stays, so a save while the router is
// away keeps it. Keep in step with namePresets.
func (a *App) nameRights(c *config.Config, old config.Config) {
	type key struct {
		dst bool
		n   int
	}
	was := map[key]string{}
	for _, r := range everyRights(&old) {
		for _, side := range []struct {
			l   *[]config.Ref
			dst bool
		}{{r.Sources, false}, {r.Dsts, true}} {
			if side.l == nil {
				continue
			}
			for _, f := range *side.l {
				if f.Name != "" && (f.Router == "" || f.Router == old.UID) {
					was[key{side.dst, f.N}] = f.Name
				}
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	fill := func(l *[]config.Ref, dst bool, names map[int]string) {
		if l == nil {
			return
		}
		for i := range *l {
			f := &(*l)[i]
			if f.Name != "" || (f.Router != "" && f.Router != c.UID) {
				continue
			}
			if n := names[f.N]; strings.TrimSpace(n) != "" {
				f.Name = n
			} else {
				f.Name = was[key{dst, f.N}]
			}
		}
	}
	for _, r := range everyRights(c) {
		fill(r.Sources, false, a.router.srcNames)
		fill(r.Dsts, true, a.router.dstNames)
	}
}

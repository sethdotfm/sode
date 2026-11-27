package app

import (
	"strconv"
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
	was := map[string]string{}
	note := config.RefWalk{Fn: func(kind string, f config.Ref) []config.Ref {
		if f.Name != "" && (f.Router == "" || f.Router == old.DefaultRouter().UID) {
			was[kind+strconv.Itoa(f.N)] = f.Name
		}
		return nil
	}}
	for _, r := range everyRights(&old) {
		note.Rights(r)
	}
	first := a.firstUnit()
	a.mu.Lock()
	defer a.mu.Unlock()
	fill := config.RefWalk{Fn: func(kind string, f config.Ref) []config.Ref {
		if f.Name != "" || f.Router != "" && f.Router != c.DefaultRouter().UID {
			return nil
		}
		if f.Name = sideNames(kind, first.router.srcNames, first.router.dstNames)[f.N]; strings.TrimSpace(f.Name) == "" {
			f.Name = was[kind+strconv.Itoa(f.N)]
		}
		return []config.Ref{f}
	}}
	for _, r := range everyRights(c) {
		fill.Rights(r)
	}
}

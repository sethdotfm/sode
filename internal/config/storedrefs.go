package config

import (
	"errors"
	"fmt"
)

// Only a request can leave the router out; Normalize fills it there. Rights levels name their router too.
func (c Config) noRouterProblem() error {
	bare := false
	w := RefWalk{Fn: func(_ string, r Ref) []Ref { bare = bare || r.Router == ""; return nil }}
	check := func(who string, visit func()) error {
		bare = false
		if visit(); bare {
			return fmt.Errorf("%s: a source, destination or level list has no router", who)
		}
		return nil
	}
	rights := func(r *Rights) func() {
		return func() {
			w.Rights(r)
			_, blank := r.Levels[""]
			bare = bare || blank
		}
	}
	presets := func(who string, ps []Preset) error {
		for _, p := range ps {
			if err := check(fmt.Sprintf("%s preset %q", who, p.Name), func() { w.Preset(&p) }); err != nil {
				return err
			}
		}
		return nil
	}
	errs := []error{presets("Everyone", c.Presets), check("Guest rights", rights(&c.GuestRights))}
	for _, u := range c.Users {
		errs = append(errs, check("rights of "+u.Name, rights(&u.Rights)), presets(u.Name+"'s Me", u.Profile.Presets))
	}
	for _, a := range c.APIClients {
		errs = append(errs, check("rights of API client "+a.Name, rights(&a.Rights)))
	}
	return errors.Join(errs...)
}

package config

import (
	"errors"
	"fmt"
	"slices"
)

// noRouterProblem names the references in a config file with no router. Only a request can leave the router out,
// and Normalize fills it there.
func (c Config) noRouterProblem() error {
	blank := func(l []Ref) bool { return slices.ContainsFunc(l, func(r Ref) bool { return r.Router == "" }) }
	presets := func(who string, ps []Preset) error {
		for _, p := range ps {
			for _, it := range p.Items {
				if it.Src.Router == "" || blank(it.Dsts) {
					return fmt.Errorf("%s preset %q: a source or destination has no router", who, p.Name)
				}
			}
		}
		return nil
	}
	rights := func(who string, r Rights) error {
		for _, l := range []*[]Ref{r.Sources, r.Dsts} {
			if l != nil && blank(*l) {
				return fmt.Errorf("%s: a source or destination has no router", who)
			}
		}
		return nil
	}
	errs := []error{presets("Everyone", c.Presets), rights("Guest rights", c.GuestRights)}
	for _, u := range c.Users {
		errs = append(errs, rights("rights of "+u.Name, u.Rights), presets(u.Name+"'s Me", u.Profile.Presets))
	}
	for _, a := range c.APIClients {
		errs = append(errs, rights("rights of API client "+a.Name, a.Rights))
	}
	return errors.Join(errs...)
}

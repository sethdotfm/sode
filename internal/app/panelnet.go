package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Allowed networks: an optional CIDR list on a panel's API client. A key from an address outside
// it is refused with network_not_allowed, so a link that leaks does not work from outside the
// building. A bare address is a single-host entry (/32 or /128). Stored masked and canonical.
// Docs: docs/panels-design.md, docs/persisted-names.md.

const panelNetworksMax = 16

func init() {
	registerAPI(apiRoute{Method: "PUT", Path: apiBase + "/panels/{id}/networks", Scope: config.ScopePanelsManage, Handle: (*App).apiPanelNetworks})
}

// parseNetworks validates and canonicalizes a list; blank entries are dropped.
func parseNetworks(in []string) ([]string, error) {
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		p, err := netip.ParsePrefix(s)
		if err != nil {
			a, aerr := netip.ParseAddr(s)
			if aerr != nil {
				return nil, fmt.Errorf("%q is not an address or a CIDR such as 192.168.1.0/24", s)
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		p = p.Masked()
		if str := p.String(); !slices.Contains(out, str) {
			out = append(out, str)
		}
	}
	if len(out) > panelNetworksMax {
		return nil, fmt.Errorf("at most %d networks", panelNetworksMax)
	}
	return out, nil
}

// networkAllowed: an empty list allows any address. An entry that does not parse allows nothing,
// so a hand-edited config.yaml cannot open the list by mistake.
func networkAllowed(nets []string, ip string) bool {
	if len(nets) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, n := range nets {
		if p, err := netip.ParsePrefix(n); err == nil && p.Contains(addr) {
			return true
		}
	}
	return false
}

func refuseNetwork(w http.ResponseWriter, ip string) {
	apiFail(w, http.StatusForbidden, "network_not_allowed", "this key may not be used from "+ip+": the panel's allowed networks do not include it")
}

// apiPanelNetworks replaces the list; an empty one lifts the limit. The caller's own address is
// not checked: the owner may be setting this from elsewhere.
func (a *App) apiPanelNetworks(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, _, ok := a.panelKeyFor(w, r, cl)
	if !ok {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	var req struct {
		Networks []string `json:"networks"`
	}
	if err == nil {
		err = json.Unmarshal(body, &req)
	}
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", `send {"networks":["192.168.1.0/24"]}`)
		return
	}
	nets, err := parseNetworks(req.Networks)
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	a.cfgMu.Lock()
	err = a.applyConfig(func(cur *config.Config) error {
		cur.APIClients = slices.Clone(cur.APIClients)
		for i := range cur.APIClients {
			if cur.APIClients[i].Panel == p.ID {
				cur.APIClients[i].Networks = nets
			}
		}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the networks could not be saved: "+err.Error())
		return
	}
	a.apiEvent(cl.Actor, "panelNetworks", "ok", "panel "+p.ID+" "+p.Name, nil)
	apiJSON(w, map[string]any{"networks": nets})
}

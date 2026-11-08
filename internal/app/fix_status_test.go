package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

const fixRouterAddr = "172.17.60.101:8910"

func fixStatusApp(t *testing.T) *App {
	t.Helper()
	a, _ := apiTestApp(t, func(c *config.Config) {
		c.API.GuestEnabled = true
		c.APIClients = append(c.APIClients, config.APIClient{Name: "StatLog", Scopes: []string{config.ScopeStatus, config.ScopeLog}})
	})
	a.mu.Lock()
	a.status = statusMsg{T: "status", Router: "down", Addr: fixRouterAddr, Err: "dial tcp " + fixRouterAddr + ": connection refused"}
	a.mu.Unlock()
	return a
}

func TestStatusHidesRouterAddressWithoutLogScope(t *testing.T) {
	a := fixStatusApp(t)
	statLog := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "StatLog"}, "Wall", time.Time{})
	for name, key := range map[string]string{"guest": "", "client": botKey(t, a)} {
		rec := apiDo(a, "GET", "/api/v1/status", key, nil)
		if rec.Code != 200 {
			t.Fatalf("%s: %d", name, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "172.17") {
			t.Errorf("%s sees the router address: %s", name, rec.Body.String())
		}
		st := apiBody(t, rec)
		if st["routerUp"] != false || st["router"] != "down" || st["error"] != true || st["server"] != "up" {
			t.Errorf("%s: status %v", name, st)
		}
		if _, ok := st["address"]; ok {
			t.Errorf("%s: address key present", name)
		}
	}
	st := apiBody(t, apiDo(a, "GET", "/api/v1/status", statLog, nil))
	if st["address"] != fixRouterAddr || st["err"] == nil || st["error"] != true {
		t.Errorf("log scope: %v", st)
	}
}

func TestStreamStatusHidesRouterAddressWithoutLogScope(t *testing.T) {
	a := fixStatusApp(t)
	srv := strmServer(t, a)
	plain := strmDial(t, srv, botKey(t, a), "status")
	logger := strmDial(t, srv, apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "StatLog"}, "Wall", time.Time{}), "status")
	for _, c := range []*strmConn{plain, logger} {
		c.next() // hello
	}
	a.mu.Lock()
	a.broadcast(a.status)
	a.mu.Unlock()
	for i := 0; i < 2; i++ {
		for name, c := range map[string]*strmConn{"plain": plain, "log": logger} {
			if i == 0 {
				strmWantType(t, c.next(), "snapshot", "status")
				continue
			}
			m := c.next()
			strmWantType(t, m, "status", "")
			has := strings.Contains(string(m.Data), "172.17")
			if has != (name == "log") {
				t.Errorf("%s frame %s", name, m.Data)
			}
		}
	}
}

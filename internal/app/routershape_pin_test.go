package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

var updatePins = flag.Bool("update-pins", false, "write the router shape pins in testdata/pin as the new want")

// pinVolatile: keys whose values change from run to run or from release to release.
var pinVolatile = map[string]bool{"version": true, "uptimeSeconds": true, "at": true, "savedAt": true, "protectAt": true, "since": true}

// pinJSON writes b indented with the volatile values blanked, then compares it with testdata/pin/name. -update-pins
// writes it.
func pinJSON(t *testing.T, name string, b []byte) {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s: %v: %s", name, err, b)
	}
	got, _ := json.MarshalIndent(blankVolatile(v), "", "  ")
	got = append(got, '\n')
	path := filepath.Join("testdata", "pin", name)
	if *updatePins {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update-pins to make it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s changed\n got %s\nwant %s", name, got, want)
	}
}

func blankVolatile(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if pinVolatile[k] {
				x[k] = "*"
				continue
			}
			x[k] = blankVolatile(e)
		}
	case []any:
		for i, e := range x {
			x[i] = blankVolatile(e)
		}
	}
	return v
}

const (
	pinUIDA = "a1a1a1a1a1a1a1a1"
	pinUIDB = "b2b2b2b2b2b2b2b2"
	pinUIDC = "c3c3c3c3c3c3c3c3"
)

// shapeApp: one router, or three, with fixed UIDs. Each has a fake router with its link up and a Protect. An API client
// has the status scope.
func shapeApp(t *testing.T, routers int) (*App, string) {
	t.Helper()
	a := newTestApp(t, func(c *config.Config) {
		c.InstallUID = "c0ffee0000000001"
		r := c.EditDefaultRouter()
		r.UID, r.Title, r.Address, r.ProtectDevice = pinUIDA, "Studio", "10.0.0.50:8910", 9
		if routers == 3 {
			b := moreRouter("B", "Second", "10.0.0.9:8910", 2)
			b.UID, b.NumberOffset, b.ProtectDevice = pinUIDB, 0, 11
			cc := moreRouter("C", "", "10.0.0.10:8910", 4)
			cc.UID, cc.LevelNumbering, cc.SalvoGroup = pinUIDC, "va", 3
			c.Routers = append(c.Routers[:1:1], []config.Router{b, cc}...)
		}
		c.APIClients = []config.APIClient{{Name: "PanelsBot", Scopes: []string{config.ScopeStatus, config.ScopeLog}}}
	})
	key := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Panel 43", time.Time{})
	fa := newFakeRouter(t, a)
	fa.restore()
	for _, rc := range a.r().cfg.Routers[1:] {
		f, _ := addFakeUnit(t, a, rc)
		f.restore()
	}
	a.mu.Lock()
	for i, u := range a.unitsInOrder(a.r()) {
		u.router.protect[3+i] = map[int]protHold{1: {dev: 5 + i, kind: 1}}
	}
	a.mu.Unlock()
	return a, key
}

func pinHello(a *App) []byte {
	page := a.readHello()
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.hello(page)
}

func pinName(routers int, what string) string {
	if routers == 1 {
		return "one-router." + what + ".json"
	}
	return "three-routers." + what + ".json"
}

// These keep their shape byte for byte, with one router and with three: the hello, /api/health and GET /api/v1/status.
// The hello after a config message and after a one Save also keeps it. A config message ignores the old flat router
// keys.
func TestPinRouterShapes(t *testing.T) {
	for _, n := range []int{1, 3} {
		t.Run(map[int]string{1: "one-router", 3: "three-routers"}[n], func(t *testing.T) {
			a, key := shapeApp(t, n)
			pinJSON(t, pinName(n, "hello"), pinHello(a))

			rec, _ := getHealth(t, a, http.MethodGet)
			pinJSON(t, pinName(n, "health"), rec.Body.Bytes())
			pinJSON(t, pinName(n, "status"), apiDo(a, "GET", "/api/v1/status", key, nil).Body.Bytes())

			c := &client{ip: "10.0.0.1"}
			next := registerAll(a, c)
			if err := a.saveSettings(c, json.RawMessage(`{"routerName":"Studio One","levels":4,"brandText":"Desk","routerShow":"name"}`)); err != nil {
				t.Fatal(err)
			}
			pinJSON(t, pinName(n, "config-hello"), jsonOf(lastHello(t, next())))

			// The flat key routerName is gone: the first router's title is router/<uid>/title.
			kv := map[string]any{"routerName": "Studio X", "router/" + pinUIDA + "/title": "Studio Two", "router/" + pinUIDA + "/nameLength": 10}
			if n == 3 {
				kv["router/"+pinUIDB+"/levels"] = 5
			}
			res, err := a.saveKeys(c, kv)
			if err != nil {
				t.Fatal(err)
			}
			pinJSON(t, pinName(n, "settings-save"), jsonOf(res))
			pinJSON(t, pinName(n, "settings-save-hello"), jsonOf(lastHello(t, next())))
		})
	}
}

// A lost router: the flat fields of /api/health and /api/v1/status are of the first router, with its error.
func TestPinRouterShapesLost(t *testing.T) {
	a, key := shapeApp(t, 3)
	a.mu.Lock()
	ua, ub := a.firstUnit(), a.units()[pinUIDB]
	a.mu.Unlock()
	for _, u := range []*routerUnit{ua, ub} {
		a.onUnitState(u, swp08.StateDown, errors.New("connection refused"))
	}
	rec, _ := getHealth(t, a, http.MethodGet)
	pinJSON(t, "three-routers.health-lost.json", rec.Body.Bytes())
	pinJSON(t, "three-routers.status-lost.json", apiDo(a, "GET", "/api/v1/status", key, nil).Body.Bytes())
}

func lastHello(t *testing.T, msgs []map[string]any) map[string]any {
	t.Helper()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i]["t"] == "hello" {
			return msgs[i]
		}
	}
	t.Fatalf("no hello in %d messages", len(msgs))
	return nil
}

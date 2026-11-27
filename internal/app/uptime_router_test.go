package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// uptimeRouters: the first router and router B, each with its own uptime: A came up, B came up and went down.
func uptimeRouters(t *testing.T) (*App, *routerUnit) {
	t.Helper()
	a := newTestApp(t, func(c *config.Config) {
		c.Routers = append(c.Routers[:1:1], []config.Router{moreRouter("B", "Truck", "10.0.0.9:8910", 3)}...)
	})
	_, b := addFakeUnit(t, a, a.r().cfg.Routers[1])
	a.firstUnit().up.State(true)
	b.up.State(true)
	b.up.State(false)
	b.up.Note(Sent{Result: "nak", Tries: 3})
	return a, b
}

// The uptime request names its router. Blank is the first router, and a UID with no router gets nothing recorded.
func TestUptimeReplyPerRouter(t *testing.T) {
	a, b := uptimeRouters(t)
	last := func(m uptimeMsg) string {
		if len(m.Trans) == 0 {
			return ""
		}
		return m.Trans[len(m.Trans)-1].S
	}
	if m := a.uptimeReply(1, "", 0, 0, 60_000); m.Router != a.firstUnit().uid() || last(m) != "up" {
		t.Fatalf("first router: %+v", m)
	}
	if m := a.uptimeReply(2, b.uid(), 0, 0, 60_000); m.Router != b.uid() || m.ID != 2 || last(m) != "down" || len(m.Counts) != 1 {
		t.Fatalf("router B: %+v", m)
	}
	if m := a.uptimeReply(3, "gone", 0, 0, 60_000); m.Router != "gone" || m.Start != "" || len(m.Trans) != 0 || m.Trans == nil {
		t.Fatalf("a router that is gone: %+v", m)
	}
}

// The download's uptime parts hold every router, the first router first, and each item names its router.
func TestLogDownloadUptimePerRouter(t *testing.T) {
	a, b := uptimeRouters(t)
	rec := logDownload(t, a, "", logDownloadReq{Parts: []string{partUptimeStates, partUptimeMinutes}, For: "full"})
	var got struct {
		States []struct {
			S      string `json:"s"`
			Router string `json:"router"`
		} `json:"uptimeStates"`
		Minutes []struct {
			Bad    int64  `json:"bad"`
			Router string `json:"router"`
		} `json:"uptimeMinutes"`
	}
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("%v in %s", err, rec.Body)
	}
	// Each router's run, in order, with the state it ends on: a unit starts with a down state of its own.
	var order []string
	for _, s := range got.States {
		if n := len(order); n > 0 && strings.HasPrefix(order[n-1], s.Router+":") {
			order[n-1] = s.Router + ":" + s.S
			continue
		}
		order = append(order, s.Router+":"+s.S)
	}
	if want := a.firstUnit().uid() + ":up " + b.uid() + ":down"; strings.Join(order, " ") != want {
		t.Fatalf("states %v, want %s", order, want)
	}
	if len(got.Minutes) != 1 || got.Minutes[0].Router != b.uid() || got.Minutes[0].Bad != 1 {
		t.Fatalf("minutes %+v", got.Minutes)
	}
}

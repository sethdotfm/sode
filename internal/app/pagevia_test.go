package app

import (
	"encoding/json"
	"testing"

	"github.com/sethdotfm/sode/internal/swp08"
)

type pageViaIn struct {
	T         string        `json:"t"`
	RouterUID string        `json:"routerUid"`
	Rows      []apiRouteRow `json:"rows"`
}

// viaFrames drains c and gives the last `viaRows` message of each router.
func viaFrames(c *client) map[string]pageViaIn {
	out := map[string]pageViaIn{}
	for {
		select {
		case b := <-c.send:
			var m pageViaIn
			json.Unmarshal(b, &m)
			if m.T == "viaRows" {
				out[m.RouterUID] = m
			}
		default:
			return out
		}
	}
}

func rowAt(rows []apiRouteRow, dst, level int) (apiRouteRow, bool) {
	for _, r := range rows {
		if r.Dst == dst && r.Level == level {
			return r, true
		}
	}
	return apiRouteRow{}, false
}

// The page gets the Via of each router in the hello. It gets a `viaRows` message when the Via changes upstream, or
// when a destination downstream routes a tie line.
func TestPageViaMessages(t *testing.T) {
	a, fa, fb, _, _ := threeApp(t, nil)
	a.flush()
	a.mu.Lock()
	var h struct {
		Routers []struct {
			UID string        `json:"uid"`
			Via []apiRouteRow `json:"via"`
		} `json:"routers"`
	}
	json.Unmarshal(a.hello(helloPage{}), &h)
	a.mu.Unlock()
	by := map[string][]apiRouteRow{}
	for _, r := range h.Routers {
		by[r.UID] = r.Via
	}
	if r, ok := rowAt(by[uidB], 20, 1); !ok || r.Origin == nil || r.Origin.N != 3 || len(r.Via) != 1 || r.Via[0].Name != "TIE 3" {
		t.Fatalf("hello B.20 %+v (%v)", r, by[uidB])
	}
	if _, ok := rowAt(by[uidB], 2, 1); ok {
		t.Error("B.2 routes no tie line, yet the hello gives it a Via row")
	}
	if r, ok := rowAt(by[uidC], 1, 1); !ok || len(r.Via) != 2 {
		t.Errorf("hello C.1 %+v", r)
	}

	c := &client{ip: "10.0.0.7", send: make(chan []byte, 64)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()

	// Upstream: TIE 3 goes mixed. B and C get new rows; A has none.
	fa.msg(swp08.ExtConnected(0, a.r().byUI[2].Wire, 10, 4))
	a.flush()
	got := viaFrames(c)
	if r, ok := rowAt(got[uidB].Rows, 20, 1); !ok || !r.Mixed || len(r.Levels) == 0 || r.LevelsRouter != a.firstUnit().uid() {
		t.Errorf("B after the upstream change %+v", got[uidB])
	}
	if r, ok := rowAt(got[uidC].Rows, 1, 1); !ok || !r.Mixed || r.LevelsRouter != uidB {
		t.Errorf("C after the upstream change %+v", got[uidC])
	}
	if _, ok := got[a.firstUnit().uid()]; ok {
		t.Error("router A has no Via, yet a viaRows message came for it")
	}

	// Downstream: B.22 now routes the far end of TIE 3. Only B changes.
	fb.msg(swp08.ExtConnected(0, a.r().byUI[1].Wire, 22, 5))
	a.flush()
	got = viaFrames(c)
	if _, ok := rowAt(got[uidB].Rows, 22, 1); !ok {
		t.Errorf("B.22 %+v", got[uidB])
	}
	if _, ok := got[uidC]; ok {
		t.Error("C did not change, yet a viaRows message came for it")
	}

	// No change: no message.
	a.flush()
	if got = viaFrames(c); len(got) != 0 {
		t.Errorf("a flush with no change sent %+v", got)
	}
}

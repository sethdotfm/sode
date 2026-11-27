package app

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// panelTieApp is tieApp with two more tie lines. TIE 4 (A.11 to B.6) is a second tie line from A to B. TIE 9
// (B.30 to A.12) goes from B to A.
func panelTieApp(t *testing.T, mut func(*config.Config)) (*App, *fakeRouter, *fakeRouter, string) {
	t.Helper()
	return tieApp(t, func(c *config.Config) {
		c.TieLines = append(c.TieLines,
			config.TieLine{ID: "t2", Name: "TIE 4", From: config.Ref{Router: c.Routers[0].UID, N: 11}, To: config.Ref{Router: "b2b2b2b2", N: 6}},
			config.TieLine{ID: "t9", Name: "TIE 9", From: config.Ref{Router: "b2b2b2b2", N: 30}, To: config.Ref{Router: c.Routers[0].UID, N: 12}})
		if mut != nil {
			mut(c)
		}
	})
}

func tiePanel(name, ties string) string {
	if ties == "" {
		return fmt.Sprintf(`{"name":%q}`, name)
	}
	return fmt.Sprintf(`{"name":%q,"tieLines":%s}`, name, ties)
}

// One tie line can be Reserved by one panel only, checked at the save. One link gets one tie line. Off is no setting.
func TestPanelTiesSave(t *testing.T) {
	a, _, _, _ := panelTieApp(t, nil)
	ada := userKey(t, a, "Ada")
	lobby := makePanel(t, a, ada, tiePanel("Lobby", `{"mode":"reserved","reserved":["t1","t9","gone"]}`))
	p, _ := a.panels.get(lobby)
	if p.TieLines == nil || p.TieLines.Mode != "reserved" || strings.Join(p.TieLines.Reserved, ",") != "t1,t9" {
		t.Fatalf("stored %+v", p.TieLines)
	}
	if got, _ := apiBody(t, apiSend(a, "GET", "/api/v1/panels/"+lobby, ada, ""))["tieLines"].(map[string]any); got["mode"] != "reserved" {
		t.Errorf("read %v", got)
	}

	rec := apiSend(a, "POST", "/api/v1/panels", ada, tiePanel("Studio", `{"mode":"reserved","reserved":["t1"]}`))
	wantErr(t, rec, http.StatusConflict, "conflict")
	if !strings.Contains(rec.Body.String(), "TIE 3 is Reserved by panel Lobby") {
		t.Errorf("message %s", rec.Body)
	}
	studio := makePanel(t, a, ada, tiePanel("Studio", `{"mode":"reserved","reserved":["t2"]}`))
	wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+studio, ada, tiePanel("Studio", `{"mode":"reserved","reserved":["t1"]}`)), http.StatusConflict, "conflict")
	// The panel's own save keeps its own tie lines.
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+lobby, ada, tiePanel("Lobby", `{"mode":"reserved","reserved":["t1"]}`)); rec.Code != http.StatusOK {
		t.Fatalf("own save: %d %s", rec.Code, rec.Body)
	}

	wantErr(t, apiSend(a, "POST", "/api/v1/panels", ada, tiePanel("Two", `{"mode":"reserved","reserved":["t1","t2"]}`)), http.StatusBadRequest, "bad_request")
	wantErr(t, apiSend(a, "POST", "/api/v1/panels", ada, tiePanel("Bad", `{"mode":"sometimes"}`)), http.StatusBadRequest, "bad_request")
	off := makePanel(t, a, ada, tiePanel("Off", `{"mode":"off","reserved":["t1"]}`))
	if p, _ := a.panels.get(off); p.TieLines != nil {
		t.Errorf("off stored %+v", p.TieLines)
	}

	// GET /tielines names the panel of each Reserve.
	body := apiSend(a, "GET", "/api/v1/tielines", ada, "").Body.String()
	if !strings.Contains(body, `"reserve":{"id":"`+lobby+`","name":"Lobby"}`) || !strings.Contains(body, `"name":"Studio"`) {
		t.Errorf("tielines %s", body)
	}
}

// Save a copy, Push and a restore leave the tie line setting out.
func TestPanelTiesStayWithThePanel(t *testing.T) {
	a, _, _, _ := panelTieApp(t, nil)
	ada := userKey(t, a, "Ada")
	src := makePanel(t, a, ada, tiePanel("Lobby", `{"mode":"reserved","reserved":["t1"]}`))
	rec := apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", ada, `{}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body)
	}
	cp := apiBody(t, rec)
	if _, ok := cp["tieLines"]; ok {
		t.Errorf("copy has %v", cp["tieLines"])
	}
	dst := makePanel(t, a, ada, tiePanel("Studio", `{"mode":"reserved","reserved":["t2"]}`))
	if rec := apiSend(a, "POST", "/api/v1/panels/"+src+"/push", ada, `{"targets":["`+dst+`"]}`); rec.Code != http.StatusOK {
		t.Fatalf("push: %d %s", rec.Code, rec.Body)
	}
	if p, _ := a.panels.get(dst); p.TieLines == nil || strings.Join(p.TieLines.Reserved, ",") != "t2" {
		t.Errorf("after the Push %+v", p.TieLines)
	}
	// A checkpoint holds the layout only: a restore keeps the setting of now.
	a.panels.mu.Lock()
	cps := a.panels.cpsLocked(dst)
	a.panels.mu.Unlock()
	if len(cps) != 1 {
		t.Fatalf("checkpoints %d", len(cps))
	}
	if _, ok, err := a.panels.restoreCheckpoint(dst, cps[0].ID, "Ada", nil); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if p, _ := a.panels.get(dst); p.TieLines == nil || strings.Join(p.TieLines.Reserved, ",") != "t2" {
		t.Errorf("after the restore %+v", p.TieLines)
	}
}

// panelHdr makes a user's key act for a panel, as the panel page does.
func panelHdr(id string) map[string]string {
	return map[string]string{"X-Sode-Panel": id, "X-Sode-Instance": "Desk"}
}

// A panel at Off gets 403 tie_lines_off, also as an API client. Nothing goes to a router.
func TestPanelTiesOffRefuses(t *testing.T) {
	a, fa, fb, _ := panelTieApp(t, nil)
	ada := userKey(t, a, "Ada")
	off := makePanel(t, a, ada, tiePanel("Lobby", ""))
	log := connectLog(fa, fb)
	uidA := a.r().cfg.Routers[0].UID
	for _, body := range []string{tieRoutes(uidA, [2]int{4, 2}), ends(uidA, "b2b2b2b2", 4, 2)} {
		rec := apiPost(a, "/api/v1/takes", ada, body, panelHdr(off))
		wantErr(t, rec, http.StatusForbidden, "tie_lines_off")
		if !strings.Contains(rec.Body.String(), "panel Lobby has tie lines Off") {
			t.Errorf("message %s", rec.Body)
		}
	}
	// Without the panel, the same user sends it.
	if rec := apiPost(a, "/api/v1/takes", ada, tieRoutes(uidA, [2]int{4, 2}), nil); rec.Code != http.StatusOK {
		t.Fatalf("no panel: %d %s", rec.Code, rec.Body)
	}
	if got := slices.Compact(log()); len(got) != 2 {
		t.Errorf("connects %v", got)
	}

	// A panel's own API client.
	b, _, _, key := panelTieApp(t, func(c *config.Config) { c.APIClients[0].Panel = "p-0000aaaa" })
	if _, err := b.panels.createAs(Panel{ID: "p-0000aaaa", Name: "Wall", Owner: ownerOf(b, "Ada").User, Rows: 1, Cols: 1}); err != nil {
		t.Fatal(err)
	}
	wantErr(t, apiPost(b, "/api/v1/takes", key, tieRoutes(b.r().cfg.Routers[0].UID, [2]int{4, 2}), nil), http.StatusForbidden, "tie_lines_off")
}

// A Reserved panel plans and sends through its own tie line only, even when another path costs less.
func TestPanelTiesReservedPath(t *testing.T) {
	a, fa, fb, _ := panelTieApp(t, nil)
	ada := userKey(t, a, "Ada")
	uidA := a.r().cfg.Routers[0].UID
	id := makePanel(t, a, ada, tiePanel("Lobby", `{"mode":"reserved","reserved":["t2"]}`))

	// Another caller sees TIE 4 Reserved by panel Lobby, sorted after TIE 3.
	rec := apiPost(a, "/api/v1/takes", ada, ends(uidA, "b2b2b2b2", 4, 2), nil)
	b := tieBody(t, rec.Body)
	if len(b.Routes) != 1 || len(b.Routes[0].Paths) != 2 {
		t.Fatalf("plan %s", rec.Body)
	}
	if p := b.Routes[0].Paths[1]; !p.Reserved || p.Hops[0].Reserve != "Lobby" || p.Hops[0].Tie.ID != "t2" {
		t.Errorf("reserved path %+v", p)
	}

	// The panel sees only TIE 4, not Reserved for itself.
	rec = apiPost(a, "/api/v1/takes", ada, ends(uidA, "b2b2b2b2", 4, 2), panelHdr(id))
	wantErr(t, rec, http.StatusConflict, "tie_line_needed")
	b = tieBody(t, rec.Body)
	if len(b.Routes[0].Paths) != 1 {
		t.Fatalf("panel plan %s", rec.Body)
	}
	if p := b.Routes[0].Paths[0]; p.Reserved || p.Hops[0].Tie.ID != "t2" || p.Hops[0].Reserve != "" {
		t.Errorf("panel path %+v", p)
	}

	// ties through a tie line that the panel does not Reserve: 409 conflict.
	rec = apiPost(a, "/api/v1/takes", ada, tieRoutes(uidA, [2]int{4, 2}), panelHdr(id))
	wantErr(t, rec, http.StatusConflict, "conflict")
	if !strings.Contains(rec.Body.String(), "TIE 3 is not Reserved by panel Lobby") {
		t.Errorf("message %s", rec.Body)
	}

	log := connectLog(fa, fb)
	body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"id":"t2","expect":null}]}]}`, uidA)
	if rec := apiPost(a, "/api/v1/takes", ada, body, panelHdr(id)); rec.Code != http.StatusOK {
		t.Fatalf("take: %d %s", rec.Code, rec.Body)
	}
	if got := strings.Join(slices.Compact(log()), " "); got != "A 4>11 B 6>2" {
		t.Errorf("connects %q", got)
	}
}

// The refusals say why: a missing Reserve, two sources through one Reserved tie line, a Protect of someone else.
func TestPanelTiesReservedRefusals(t *testing.T) {
	a, _, _, _ := panelTieApp(t, nil)
	ada := userKey(t, a, "Ada")
	uidA := a.r().cfg.Routers[0].UID
	id := makePanel(t, a, ada, tiePanel("Lobby", `{"mode":"reserved","reserved":["t1"]}`))
	try := func(body, want string) {
		t.Helper()
		rec := apiPost(a, "/api/v1/takes", ada, body, panelHdr(id))
		wantErr(t, rec, http.StatusConflict, "no_path")
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("message %s, want %q", rec.Body, want)
		}
	}
	// No Reserve on the link from B to A.
	try(ends("b2b2b2b2", uidA, 5, 2), "panel Lobby has no Reserved tie line from")
	// Two sources through TIE 3 in one press.
	two := fmt.Sprintf(`{"routes":[{"src":{"router":%[1]q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1]},{"src":{"router":%[1]q,"n":7},"dst":{"router":"b2b2b2b2","n":3},"levels":[1]}]}`, uidA)
	try(two, "this press needs two sources through TIE 3")
	// Someone else holds a Protect on TIE 3.
	a.mu.Lock()
	a.first.router.protect[10] = map[int]protHold{1: {dev: 99, kind: swp08.KindProtect}}
	a.mu.Unlock()
	try(ends(uidA, "b2b2b2b2", 4, 2), "TIE 3 is protected by device 99")
}

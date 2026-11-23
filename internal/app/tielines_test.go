package app

import (
	"net/http"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// An Administrator saves tie lines; each end keeps its router name; the list survives a restart.
func TestSaveTieLines(t *testing.T) {
	a, _, _, u := twoRouterFiles(t)
	readUnitNames(a, a.first, []string{"A CAM 1"}, []string{"A MON 1", "TO STAGE 2"})
	readUnitNames(a, u, []string{"B CAM 1", "FROM STAGE 1"}, []string{"B MON 1"})
	ada := adminClient(t, a)
	ada.send = make(chan []byte, 16)
	a.mu.Lock()
	a.clients[ada] = struct{}{}
	a.mu.Unlock()
	tie := config.TieLine{Name: "TIE 3", From: config.Ref{Router: a.first.uid(), N: 1}, To: config.Ref{Router: u.uid(), N: 1}}
	if err := a.SaveTieLines(ada, []config.TieLine{tie}); err != nil {
		t.Fatal(err)
	}
	got := a.r().cfg.TieLines
	if len(got) != 1 || got[0].ID == "" || got[0].Name != "TIE 3" || got[0].From.Name != "TO STAGE 2" || got[0].To.Name != "FROM STAGE 1" {
		t.Fatalf("tie lines %+v", got)
	}
	if m := pageMsgs(t, ada)["tieLines"]; len(m) != 1 || len(m[0]["tieLines"].([]any)) != 1 {
		t.Errorf("tieLines broadcast %v", m)
	}
	back, _, err := config.Load(a.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.TieLines) != 1 || back.TieLines[0].ID != got[0].ID || back.TieLines[0].To.Router != u.uid() {
		t.Fatalf("after a restart %+v", back.TieLines)
	}
	// An edit keeps the id; a blank name takes the downstream source name.
	edit := got[0]
	edit.Name = ""
	if err := a.SaveTieLines(ada, []config.TieLine{edit}); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg.TieLines; len(got) != 1 || got[0].ID != edit.ID || got[0].Name != "FROM STAGE 1" {
		t.Errorf("after an edit %+v", got)
	}
	if err := a.SaveTieLines(ada, nil); err != nil || len(a.r().cfg.TieLines) != 0 {
		t.Errorf("remove: %v %+v", err, a.r().cfg.TieLines)
	}
}

// Only an Administrator can change tie lines.
func TestSaveTieLinesNeedsAnAdministrator(t *testing.T) {
	a := userApp(t, false)
	setFlags(t, a, "Ana", true, false)
	ana := watched(t, a, "10.0.0.1", "1234")  // Trusted
	bo := watched(t, a, "10.0.0.2", "567890") // Standard
	for _, c := range []*client{ana, bo} {
		if err := admitMsg(a, c, inMsg{T: "tieLines"}); !isRoleErr(err) {
			t.Errorf("%s admitted to tie lines: %v", c.who(), err)
		}
		if err := a.SaveTieLines(c, nil); !isRoleErr(err) {
			t.Errorf("%s saved tie lines: %v", c.who(), err)
		}
	}
	if err := admitMsg(a, adminClient(t, a), inMsg{T: "tieLines"}); err != nil {
		t.Errorf("an Administrator refused: %v", err)
	}
}

// The server refuses a bad tie line even when a stale or hand-made message sends one.
func TestSaveTieLinesRefusesBad(t *testing.T) {
	a, _, _, u := twoRouterFiles(t)
	ada := adminClient(t, a)
	for _, tc := range []struct {
		list []config.TieLine
		want string
	}{
		{[]config.TieLine{{Name: "X", From: config.Ref{Router: u.uid(), N: 1}, To: config.Ref{Router: u.uid(), N: 2}}}, "different router"},
		{[]config.TieLine{
			{Name: "TIE", From: config.Ref{Router: a.first.uid(), N: 1}, To: config.Ref{Router: u.uid(), N: 2}},
			{Name: "tie", From: config.Ref{Router: a.first.uid(), N: 2}, To: config.Ref{Router: u.uid(), N: 3}},
		}, "two tie lines are named"},
	} {
		if err := a.SaveTieLines(ada, tc.list); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: %v, want %q", tc.list, err, tc.want)
		}
	}
	if len(a.r().cfg.TieLines) != 0 {
		t.Errorf("a refused save wrote %+v", a.r().cfg.TieLines)
	}
}

// GET /tielines lists each tie line: id, Tie line name and the two router references, with no router names.
func TestAPITieLines(t *testing.T) {
	var b string
	a, _ := apiTestApp(t, func(c *config.Config) {
		r := moreRouter("B", "Second", "", 1)
		r.UID = "b2b2b2b2"
		c.MoreRouters = []config.Router{r}
		b = r.UID
		c.TieLines = []config.TieLine{{ID: "t1", Name: "TIE 3", From: config.Ref{Router: c.UID, N: 40, Name: "TO B"}, To: config.Ref{Router: r.UID, N: 2, Name: "FROM A"}}}
	})
	rec := apiDo(a, "GET", "/api/v1/tielines", botKey(t, a), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	list, _ := apiBody(t, rec)["tieLines"].([]any)
	if len(list) != 1 {
		t.Fatalf("tie lines %v", list)
	}
	got := list[0].(map[string]any)
	from, to := got["from"].(map[string]any), got["to"].(map[string]any)
	if got["id"] != "t1" || got["name"] != "TIE 3" || from["router"] != a.r().cfg.UID || from["n"] != 40.0 || to["router"] != b || to["n"] != 2.0 || len(from) != 2 {
		t.Errorf("tie line %v", got)
	}
}

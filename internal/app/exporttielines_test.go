package app

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// tieImportApp: two routers with names read, Ada signed in, and two tie lines A.2 → B.1 (TIE 3) and B.2 → A.1 (TIE 9).
func tieImportApp(t *testing.T) (*App, *client, string, *routerUnit) {
	t.Helper()
	a, _, _, u := twoRouterFiles(t)
	readUnitNames(a, a.first, []string{"A CAM 0", "FROM STAGE 2"}, []string{"A MON 0", "A MON 1", "TO STAGE 2"})
	readUnitNames(a, u, []string{"B CAM 0", "FROM STAGE 1"}, []string{"B MON 0", "B MON 1", "TO STAGE 1"})
	c := &client{ip: "10.0.0.9", send: make(chan []byte, 4096)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	if err := a.UserUnlockAs(c, "Ada", "2468"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSessionTab(c, true)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.TieLines = []config.TieLine{
			{ID: "aaaaaaaaaaaaaaa3", Name: "TIE 3", From: config.Ref{Router: cfg.Routers[0].UID, N: 2, Name: "TO STAGE 2"}, To: config.Ref{Router: u.uid(), N: 1, Name: "FROM STAGE 1"}},
			{ID: "aaaaaaaaaaaaaaa9", Name: "TIE 9", From: config.Ref{Router: u.uid(), N: 2, Name: "TO STAGE 1"}, To: config.Ref{Router: cfg.Routers[0].UID, N: 1, Name: "FROM STAGE 2"}},
		}
	})
	return a, c, tok, u
}

func tieEnds(list []config.TieLine) []string {
	var out []string
	for _, t := range list {
		out = append(out, strings.Join([]string{t.ID, t.Name, t.From.Router, t.From.Name, t.To.Router, t.To.Name}, "|"))
	}
	return out
}

// Tie lines are their own export category, picked by id, and come back the same.
func TestExportTieLinesRoundTrip(t *testing.T) {
	a, _, tok, _ := tieImportApp(t)
	want := slices.Clone(a.r().cfg.TieLines)
	file := exported(t, a, tok, exportReq{Categories: []string{catTieLines}})
	files := unzipAll(t, file)
	var man exportManifest
	if err := yaml.Unmarshal(files["manifest.yaml"], &man); err != nil || !slices.Equal(man.Categories, []string{catRouters, catTieLines}) {
		t.Fatalf("manifest categories: %v %v", err, man.Categories)
	}
	if !bytes.Contains(files["tielines.yaml"], []byte("router: "+a.r().cfg.Routers[0].Prefix+"\n")) {
		t.Errorf("an end on the first router does not keep its prefix:\n%s", files["tielines.yaml"])
	}
	b, done, err := a.stageImport(bytes.NewReader(file))
	defer done()
	if err != nil {
		t.Fatal(err)
	}
	if got := tieEnds(b.TieLines); !slices.Equal(got, tieEnds(want)) {
		t.Fatalf("read back %v, want %v", got, tieEnds(want))
	}

	list, err := a.exportList()
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(list.Categories, func(c exportListCat) bool { return c.Name == catTieLines })
	if i < 0 || len(list.Categories[i].Items) != 2 || list.Categories[i].Items[0].ID != want[0].ID || !list.Categories[i].Items[0].Router {
		t.Fatalf("export list: %+v", list.Categories)
	}
	one := unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catTieLines}, Pick: map[string][]string{catTieLines: {want[1].ID}}}))
	if bytes.Contains(one["tielines.yaml"], []byte("TIE 3")) || !bytes.Contains(one["tielines.yaml"], []byte("TIE 9")) {
		t.Fatalf("pick TIE 9 only:\n%s", one["tielines.yaml"])
	}

	// Add import into an install that lost them: both come back with their ids. A second import changes nothing.
	lifeCommit(t, a, func(cfg *config.Config) { cfg.TieLines = nil })
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if c := res.Plan.Categories[catTieLines]; c == nil || c.Refused != "" || len(c.Items) != 2 {
		t.Fatalf("plan: %+v", c)
	}
	if got := tieEnds(a.r().cfg.TieLines); !slices.Equal(got, tieEnds(want)) {
		t.Fatalf("after the import %v, want %v", got, tieEnds(want))
	}
	res = applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if it := planItem(t, res.Plan, catTieLines, "TIE 3"); !it.Same {
		t.Fatalf("equal tie line: %+v", it)
	}
}

// A tie line whose router the import maps to no router here is dropped with a note. The rest import, and the list
// still validates.
func TestImportDropsTieLineWithUnmappedRouter(t *testing.T) {
	a, _, tok, _ := tieImportApp(t)
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catTieLines}}))

	// One router here: the second router of the file is not on this server.
	one, _, otok := importApp(t)
	res := applied(t, postApply(t, one, otok, file, importReq{Mode: importAdd}))
	c := res.Plan.Categories[catTieLines]
	if c == nil || c.Refused != "" || len(c.Items) != 2 {
		t.Fatalf("plan: %+v", c)
	}
	for _, it := range c.Items {
		if !strings.Contains(it.Refused, ", which has no target here") || it.writes() {
			t.Errorf("%s: %+v", it.Name, it)
		}
	}
	if got := one.r().cfg.TieLines; len(got) != 0 {
		t.Fatalf("tie lines arrived: %+v", got)
	}

	// Two routers here, but the page maps the file's first router to none.
	lifeCommit(t, a, func(cfg *config.Config) { cfg.TieLines = nil })
	fileUID := a.r().cfg.Routers[0].UID
	res = applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Routers: map[string]string{fileUID: ""}}))
	if it := planItem(t, res.Plan, catTieLines, "TIE 9"); !strings.Contains(it.Refused, "which has no target here") {
		t.Fatalf("TIE 9: %+v", it)
	}
	if got := a.r().cfg.TieLines; len(got) != 0 {
		t.Fatalf("tie lines arrived: %+v", got)
	}
	if err := a.r().cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

// A foreign tie line with a live name but other ends arrives renamed. One with the same two ends as a live tie line
// has no Rename, and Use current keeps the live one.
func TestImportTieLineClashes(t *testing.T) {
	a, _, tok, u := tieImportApp(t)
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catTieLines}}))
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.TieLines = []config.TieLine{
			{Name: "TIE 3", From: config.Ref{Router: cfg.Routers[0].UID, N: 1}, To: config.Ref{Router: u.uid(), N: 0}},
			{Name: "B TO A", From: config.Ref{Router: u.uid(), N: 2}, To: config.Ref{Router: cfg.Routers[0].UID, N: 1}},
		}
	})
	req := importReq{Mode: importAdd, Choices: map[string]map[string]string{catTieLines: {"aaaaaaaaaaaaaaa9": useCurrent}}}
	res := applied(t, postApply(t, a, tok, file, req))
	if it := planItem(t, res.Plan, catTieLines, "TIE 3"); it.Choice != useRename || it.As != "TIE 3 2" {
		t.Errorf("TIE 3: %+v", it)
	}
	if it := planItem(t, res.Plan, catTieLines, "TIE 9"); it.Clash != "B TO A" || it.writes() {
		t.Errorf("TIE 9: %+v", it)
	}
	var names []string
	for _, x := range a.r().cfg.TieLines {
		names = append(names, x.Name)
	}
	if !slices.Equal(names, []string{"TIE 3", "B TO A", "TIE 3 2"}) {
		t.Fatalf("tie lines %v", names)
	}
}

// Replace import removes the live tie lines that the file does not hold.
func TestReplaceImportTieLines(t *testing.T) {
	a, c, tok, u := tieImportApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catTieLines}, Pick: map[string][]string{catTieLines: {"aaaaaaaaaaaaaaa3"}}})
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.TieLines = append(cfg.TieLines, config.TieLine{Name: "EXTRA", From: config.Ref{Router: cfg.Routers[0].UID, N: 0}, To: config.Ref{Router: u.uid(), N: 0}})
	})
	grantRecheck(a, c)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importReplace, Categories: []string{catTieLines}}))
	removed := 0
	for _, it := range res.Plan.Categories[catTieLines].Items {
		if it.Remove {
			removed++
		}
	}
	if got := a.r().cfg.TieLines; removed != 2 || len(got) != 1 || got[0].ID != "aaaaaaaaaaaaaaa3" {
		t.Fatalf("removed %d, tie lines %+v", removed, got)
	}
}

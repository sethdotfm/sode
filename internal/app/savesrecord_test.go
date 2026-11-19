package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// waitSaves waits for at least n Settings saves rows, newest first.
func waitSaves(t *testing.T, a *App, n int) []Sent {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rows := a.saves.Between(-1, 1<<62); len(rows) >= n {
			return rows
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("want %d Settings saves rows, have %d", n, len(a.saves.Between(-1, 1<<62)))
	return nil
}

func fieldsByKey(e Sent) map[string]SavedField {
	out := map[string]SavedField{}
	for _, f := range e.Save.Fields {
		out[f.Key] = f
	}
	return out
}

// adminSave: Ada (Administrator, rechecked) saves Branding, a user's PIN and Trusted proxies in one
// Save. It returns Ana's user id.
func adminSave(t *testing.T, a *App) string {
	t.Helper()
	ana := idOf(a, "Ana")
	p := dialPeer(t, a)
	p.signInRechecked("Ada", "2468")
	live := a.r().cfg
	fields := map[string]fieldEdit{}
	for k, v := range map[string]any{
		"brandText":            "Studio B",
		"user/" + ana + "/pin": "9753",
		"trustedProxies":       []string{"10.1.0.0/16"},
	} {
		fields[k] = fieldEdit{Value: jsonOf(v), Version: fieldVersion(live, k)}
	}
	r := p.ask("settingsSave", 1, map[string]any{"fields": fields})
	for _, k := range []string{"brandText", "user/" + ana + "/pin", "trustedProxies"} {
		wantSaved(t, r, k)
	}
	return ana
}

func TestSettingsSaveRowHasOldAndNewAndHidesSecrets(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	ana := adminSave(t, a)

	rows := waitSaves(t, a, 1)
	if len(rows) != 1 {
		t.Fatalf("one save wrote %d rows", len(rows))
	}
	e := rows[0]
	if e.Kind != kindSettings || e.By != "Ada" || !e.Save.Critical {
		t.Fatalf("row = %+v", e)
	}
	f := fieldsByKey(e)
	if b := f["brandText"]; string(b.New) != `"Studio B"` || b.Topic != "Look and feel" || b.Section != "Branding" || b.Label == "" {
		t.Fatalf("brandText = %+v", b)
	}
	if p := f["user/"+ana+"/pin"]; !p.Changed || p.Old != nil || p.New != nil || !strings.Contains(p.Label, "Ana") {
		t.Fatalf("PIN = %+v", p)
	}
	if tp := f["trustedProxies"]; len(tp.Added) != 1 || string(tp.Added[0]) != `"10.1.0.0/16"` || tp.New != nil {
		t.Fatalf("trustedProxies = %+v", tp)
	}
	b, _ := json.Marshal(e)
	u, _ := a.r().cfg.FindUserByID(ana)
	if strings.Contains(string(b), "9753") || strings.Contains(string(b), u.Pin) {
		t.Fatalf("row holds the PIN: %s", b)
	}
	// History gets nothing for a settings save.
	if page, _ := a.hist.Page(-1, 10); len(page) != 0 {
		t.Fatalf("History got %+v", page)
	}
}

// liveSave reads p's logAdd messages until one holds a Settings saves row.
func liveSave(t *testing.T, p *wsPeer) Sent {
	t.Helper()
	for {
		var m sentAddMsg
		if err := json.Unmarshal(jsonOf(p.until("logAdd")), &m); err != nil {
			t.Fatal(err)
		}
		for _, e := range m.Entries {
			if e.Kind == kindSettings {
				return e
			}
		}
	}
}

// The live feed sends each reader the form its Role can see.
func TestLiveSettingsSaveRowFollowsReaderRole(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	bo, ada := dialPeer(t, a), dialPeer(t, a)
	bo.signIn("Bo", "567890")
	ada.signIn("Ada", "2468")
	for _, p := range []*wsPeer{bo, ada} {
		p.send(map[string]any{"t": "log", "before": -1, "n": 10})
		p.until("logPage")
	}
	adminSave(t, a)
	waitSaves(t, a, 1)
	a.flush()
	if f := fieldsByKey(liveSave(t, bo))["trustedProxies"]; !f.Changed || f.Added != nil {
		t.Fatalf("Standard reader's live row: %+v", f)
	}
	if f := fieldsByKey(liveSave(t, ada))["trustedProxies"]; f.Changed || len(f.Added) != 1 {
		t.Fatalf("Administrator's live row: %+v", f)
	}
}

// Through the API a key reads the narrow form, and a panel client's widened Rights write a row.
func TestAPILogHidesAccessValues(t *testing.T) {
	a, _, cy, _ := keyedPanel(t)
	defer startLog(t, a)()
	if rec := apiSend(a, "POST", "/api/v1/panels", cy, apiPanel+`,"clientRights":{"sources":[{"n":1}],"dsts":[{"n":2}]}}`); rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	saved := waitSaves(t, a, 1)
	var add *SavedField
	for _, e := range saved {
		for _, f := range e.Save.Fields {
			if strings.HasSuffix(f.Key, "/add") {
				add = &f
			}
		}
	}
	if add == nil || add.New == nil || saved[0].Why != "api" {
		t.Fatalf("no panel client add row: %+v", saved)
	}
	// LogBot is the fixture's client with the log scope.
	rec := apiSend(a, "GET", "/api/v1/log", apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "LogBot"}, "Reader", time.Time{}), "")
	if rec.Code != 200 {
		t.Fatalf("log: %d %s", rec.Code, rec.Body.String())
	}
	var body struct{ Entries []Sent }
	json.Unmarshal(rec.Body.Bytes(), &body)
	n := 0
	for _, e := range body.Entries {
		if e.Kind != kindSettings {
			continue
		}
		for _, f := range e.Save.Fields {
			n++
			if !f.Changed || f.New != nil || f.Added != nil {
				t.Fatalf("API reader got a value of %s: %+v", f.Key, f)
			}
		}
	}
	if n == 0 {
		t.Fatalf("no Settings saves fields in %s", rec.Body.String())
	}
}

func TestEverySavedFieldHasLabel(t *testing.T) {
	for k, sec := range saveFields {
		if fieldLabels[k] == "" {
			t.Errorf("%s has no label", k)
		}
		if sectionNames[sec] == "" {
			t.Errorf("section %s has no name", sec)
		}
	}
}

// Pages of the Log carry each Settings saves row once, between the seqs it was written between.
func TestLogPagesMergeSavesByAnchor(t *testing.T) {
	a := userApp(t, false)
	for i := range 10 {
		a.sent.Append(&Sent{Kind: "keepalive", Result: "ack"})
		if i == 3 || i == 7 {
			e := saveRow("Ada")
			e.Seq, e.Time = a.sent.Next(), time.Now() // written after seq i
			a.saves.Append(&e)
		}
	}
	var order []string
	before := int64(-1)
	for {
		page, more, _ := a.sent.Page(before, 3)
		for _, e := range a.mergeSaves(page, before, more) {
			if e.Kind == kindSettings {
				order = append(order, fmt.Sprintf("s%d", e.ID))
			} else {
				order = append(order, fmt.Sprint(e.Seq))
			}
		}
		if !more {
			break
		}
		before = page[len(page)-1].Seq
	}
	if got := strings.Join(order, " "); got != "9 8 s2 7 6 5 4 s1 3 2 1 0" {
		t.Fatalf("order = %s", got)
	}
}

// A Communication Log that starts its seqs again does not put old saves at the top.
func TestSavesReanchorOnAFreshLog(t *testing.T) {
	a := userApp(t, false)
	saves, _ := OpenSaveLog("")
	old := saveRow("Ada")
	old.Seq, old.Time = 50_000, time.Now()
	saves.Append(&old)
	for range 3 {
		a.sent.Append(&Sent{Kind: "keepalive", Result: "ack"})
	}
	a.SetSettingsSaves(saves)
	page, more, _ := a.sent.Page(-1, 2)
	if got := a.mergeSaves(page, -1, more); len(got) != 2 || got[0].Kind == kindSettings {
		t.Fatalf("newest page = %+v", got)
	}
	page, more, _ = a.sent.Page(1, 2)
	if got := a.mergeSaves(page, 1, more); len(got) != 2 || got[1].Kind != kindSettings {
		t.Fatalf("last page = %+v", got)
	}
}

func TestAPIChangeWritesASettingsSavesRow(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	defer startLog(t, a)()
	if rec := putNetworks(a, cy, id, `["10.1.0.0/16"]`); rec.Code != 200 {
		t.Fatalf("networks: %d %s", rec.Code, rec.Body.String())
	}
	rows := waitSaves(t, a, 2) // the panel's client add, then the networks
	e := rows[0]
	if e.Why != "api" || len(e.Save.Fields) != 1 {
		t.Fatalf("row = %+v", e)
	}
	f := e.Save.Fields[0]
	if !strings.HasSuffix(f.Key, "/networks") || len(f.Added) != 1 || string(f.Added[0]) != `"10.1.0.0/16"` || !strings.Contains(f.Label, "Networks") {
		t.Fatalf("field = %+v", f)
	}
}

// A Standard user reading the Log sees labels, but values only for Shown sections.
func TestStandardReaderSeesNoAccessOrServerValues(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	ana := adminSave(t, a)
	waitSaves(t, a, 1)

	bo := dialPeer(t, a)
	bo.signIn("Bo", "567890")
	bo.send(map[string]any{"t": "log", "before": -1, "n": 100})
	m := bo.until("logPage")
	var page sentPageMsg
	if err := json.Unmarshal(jsonOf(m), &page); err != nil {
		t.Fatal(err)
	}
	var row *Sent
	for i := range page.Entries {
		if page.Entries[i].Kind == kindSettings {
			row = &page.Entries[i]
		}
	}
	if row == nil {
		t.Fatalf("no Settings saves row in %+v", page.Entries)
	}
	f := fieldsByKey(*row)
	if string(f["brandText"].New) != `"Studio B"` {
		t.Fatalf("Branding is Shown, so its value shows: %+v", f["brandText"])
	}
	for _, k := range []string{"trustedProxies", "user/" + ana + "/pin"} {
		if g := f[k]; !g.Changed || g.Old != nil || g.New != nil || g.Added != nil || g.Removed != nil || g.Label == "" {
			t.Fatalf("%s shows a value to a Standard user: %+v", k, g)
		}
	}
}

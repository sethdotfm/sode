package app

import (
	"os"
	"path/filepath"
	"testing"
)

// REST seam for "Ask users to set an instance name", `settings.askInstanceName`.

func askOf(p map[string]any) any {
	s, _ := p["settings"].(map[string]any)
	return s["askInstanceName"]
}

func TestAskInstanceNameRoundTripsAndDefaultsOn(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	off := makePanel(t, a, ana, `{"name":"Wall","settings":{"askInstanceName":false}}`)
	if got := askOf(readPanel(t, a, ana, off)); got != false {
		t.Fatalf("saved off, read %v", got)
	}
	if p, _ := loadPanels(a.cfgPath).get(off); p.Settings["askInstanceName"] != false {
		t.Fatalf("after reload %v", p.Settings)
	}
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+off, ana, `{"name":"Wall","settings":{"askInstanceName":true}}`); rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	if got := askOf(readPanel(t, a, ana, off)); got != true {
		t.Fatalf("saved on, read %v", got)
	}

	plain := makePanel(t, a, ana, `{"name":"Plain"}`)
	if got := askOf(readPanel(t, a, ana, plain)); got != true {
		t.Fatalf("a panel saved without it reads %v", got)
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panels", ana, `{"name":"Bad","settings":{"askInstanceName":"no"}}`), 400, "bad_request")
}

// A panel file written before the setting loads as on, and the file is not rewritten for it.
func TestAskInstanceNameOldPanelFileReadsOn(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	dir := filepath.Join(filepath.Dir(a.cfgPath), "panels")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := []byte("id: p-0000abcd\nname: Old wall\nowner: Ana\nacts: session\nrows: 1\ncols: 1\ncells: []\nsettings:\n    mode: instant\n")
	if err := os.WriteFile(filepath.Join(dir, "p-0000abcd.yaml"), old, 0o644); err != nil {
		t.Fatal(err)
	}
	a.panels = loadPanels(a.cfgPath)
	if got := askOf(readPanel(t, a, ana, "p-0000abcd")); got != true {
		t.Fatalf("old file reads %v", got)
	}
}

func TestAskInstanceNameTravelsWithPushAndTemplates(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := makePanel(t, a, ana, `{"name":"Wall 1","settings":{"askInstanceName":false}}`)
	dst := makePanel(t, a, ana, `{"name":"Wall 2"}`)
	if code, body := pushTo(a, ana, src, dst); code != 200 {
		t.Fatalf("push: %d %v", code, body)
	}
	if got := askOf(readPanel(t, a, ana, dst)); got != false {
		t.Fatalf("after the Push Wall 2 reads %v", got)
	}

	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Quiet","scope":"me","settings":{"askInstanceName":false},"cells":[]}`)
	if rec.Code != 201 {
		t.Fatalf("template: %d %s", rec.Code, rec.Body.String())
	}
	id, _ := apiBody(t, rec)["id"].(string)
	if got := askOf(apiBody(t, apiDo(a, "GET", "/api/v1/panel-templates/"+id, ana, nil))); got != false {
		t.Fatalf("template reads %v", got)
	}
}

// An unnamed instance (the setting off, no ?instance= and no stored name) still counts, under its address.
func TestUnnamedInstanceCountsUnderItsAddress(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","settings":{"askInstanceName":false}}`)
	if rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/instances", ana, `{"device":""}`); rec.Code != 200 {
		t.Fatalf("keepalive: %d %s", rec.Code, rec.Body.String())
	}
	p := readPanel(t, a, ana, id)
	if p["online"] != true {
		t.Fatalf("not online: %v", p)
	}
	insts, _ := p["instances"].([]any)
	if len(insts) != 1 {
		t.Fatalf("instances %v", p["instances"])
	}
	if dev := insts[0].(map[string]any)["device"]; dev != "192.0.2.7" {
		t.Fatalf("device %v, want the address %s", dev, "192.0.2.7")
	}
}

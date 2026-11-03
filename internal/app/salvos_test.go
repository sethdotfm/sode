package app

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestSaveSalvos(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	sv := []config.Salvo{{Name: " Morning ", Items: []config.SalvoItem{{Src: 3, Dsts: []int{1, 2}, Levels: []int{2, 1, 1}}}}}
	if err := a.SaveSalvos(c, sv); err != nil {
		t.Fatalf("open save: %v", err)
	}
	got := a.r().cfg.Salvos
	if len(got) != 1 || got[0].Name != "Morning" || len(got[0].Items[0].Levels) != 2 || got[0].Items[0].Levels[0] != 1 {
		t.Fatalf("normalized: %+v", got)
	}
	// The caller's slice isn't the live one.
	sv[0].Items[0].Dsts[0] = 99
	if a.r().cfg.Salvos[0].Items[0].Dsts[0] != 1 {
		t.Fatal("salvos share memory with the caller")
	}
	if err := a.SaveSalvos(c, []config.Salvo{{Name: "x"}}); err == nil {
		t.Fatal("salvo with no takes accepted")
	}
	// A settings save from a stale browser keeps the salvos.
	stale := a.r().cfg
	stale.Salvos = nil
	if err := a.SaveSettings(c, stale, "", false); err != nil || len(a.r().cfg.Salvos) != 1 {
		t.Fatalf("settings save dropped the salvos: %v %v", err, a.r().cfg.Salvos)
	}
	// PIN guarded like filter sets.
	if err := a.SaveSettings(c, a.r().cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSalvos(c, nil); !errors.Is(err, errLocked) {
		t.Fatalf("locked save: %v", err)
	}
	if a.Unlock(c, "2468") != nil {
		t.Fatal("unlock")
	}
	if err := a.SaveSalvos(c, nil); err != nil || len(a.r().cfg.Salvos) != 0 {
		t.Fatalf("unlocked save: %v %v", err, a.r().cfg.Salvos)
	}
}

func TestSalvoMessageShapes(t *testing.T) {
	b, _ := json.Marshal(salvosMsg{T: "salvos", Salvos: []config.Salvo{}})
	if string(b) != `{"t":"salvos","salvos":[]}` {
		t.Fatalf("salvos message %s", b)
	}
	var m struct {
		Salvos []config.Salvo `json:"salvos"`
	}
	if err := json.Unmarshal([]byte(`{"salvos":[{"name":"A","items":[{"src":1,"dsts":[2],"levels":[]}]}]}`), &m); err != nil || m.Salvos[0].Items[0].Dsts[0] != 2 {
		t.Fatalf("decode %v %+v", err, m)
	}
}

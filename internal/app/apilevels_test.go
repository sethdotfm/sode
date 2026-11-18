package app

import "testing"

func TestTallyCarriesLevelLook(t *testing.T) {
	a, key := viewApp(t, "")
	a.mu.Lock()
	a.r().cfg.LevelNumbering = "va"
	a.r().cfg.DefaultLevels = "V-3"
	a.mu.Unlock()
	res := viewGet(t, a, key, "/api/v1/tally")
	lv := res["levels"].([]any)
	if len(lv) == 0 {
		t.Fatal("no levels")
	}
	first := lv[0].(map[string]any)
	if first["swatch"] != "video" {
		t.Errorf("first level %v", first)
	}
	if c, _ := first["color"].(string); c == "" {
		t.Errorf("first level has no color: %v", first)
	}
	if res["levelNumbering"] != "va" || res["defaultLevels"] != "V-3" {
		t.Errorf("numbering %v, defaults %v", res["levelNumbering"], res["defaultLevels"])
	}
}

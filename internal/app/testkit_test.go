package app

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"
)

func jsonOf(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// saveKeys runs the one Save as c, each key at its live version; kv maps a key to its new value.
func (a *App) saveKeys(c *client, kv map[string]any) (map[string]fieldResult, error) {
	live := a.r().cfg
	edits := make(map[string]fieldEdit, len(kv))
	for k, v := range kv {
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		edits[k] = fieldEdit{Value: b, Version: fieldVersion(live, k)}
	}
	res, _, err := a.saveSettingsFields(c, edits, nil, "", "")
	return res, err
}

// firstRefused: the first refused key in key order, "" for none.
func firstRefused(res map[string]fieldResult) (string, fieldResult) {
	for _, k := range slices.Sorted(maps.Keys(res)) {
		if res[k].Status == "refused" {
			return k, res[k]
		}
	}
	return "", fieldResult{}
}

func mustSave(t *testing.T, a *App, c *client, kv map[string]any) {
	t.Helper()
	res, err := a.saveKeys(c, kv)
	if err != nil {
		t.Fatal(err)
	}
	if k, r := firstRefused(res); k != "" {
		t.Fatalf("%s refused: %s: %s", k, r.Reason, r.Err)
	}
}

// mustRefuse returns the first refusal; a save refused as a whole comes back as Err.
func mustRefuse(t *testing.T, a *App, c *client, kv map[string]any) fieldResult {
	t.Helper()
	res, err := a.saveKeys(c, kv)
	if err != nil {
		return fieldResult{Status: "refused", Err: err.Error()}
	}
	k, r := firstRefused(res)
	if k == "" {
		t.Fatalf("saved, want a refusal: %v", res)
	}
	return r
}

// ukey is the save key of attr for the user named name.
func ukey(a *App, name, attr string) string { return "user/" + idOf(a, name) + "/" + attr }

// saveLive is saveFields at the live versions, which a peer has for user and client keys only
// after usersList.
func (p *wsPeer) saveLive(a *App, id int, kv map[string]any) map[string]any {
	p.t.Helper()
	live := a.r().cfg
	fields := map[string]any{}
	for k, v := range kv {
		fields[k] = map[string]any{"value": v, "version": fieldVersion(live, k)}
	}
	return p.ask("settingsSave", id, map[string]any{"fields": fields})
}

// allSaved fails on a refused Save or any refused key.
func allSaved(t *testing.T, what string, r map[string]any) {
	t.Helper()
	res, _ := r["results"].(map[string]any)
	if r["ok"] != true || res == nil {
		t.Fatalf("%s: refused: %v", what, r)
	}
	for k, v := range res {
		if f, _ := v.(map[string]any); f["status"] == "refused" {
			t.Fatalf("%s: %s refused: %v", what, k, f)
		}
	}
}

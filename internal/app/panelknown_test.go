package app

import (
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

// A key a read adds that panelKnown lacks lands in Extra when an editor saves the read back, and is then
// stored and shown to non-editors (it happened with artDefaults, then keyless).
func TestPanelReadsAddOnlyPanelKnownKeys(t *testing.T) {
	a, _ := clientApp(t)
	cy, bo := userKey(t, a, "Cy"), userKey(t, a, "Bo")
	id := makePanel(t, a, cy, apiPanel+`,"slug":"wall","share":{"mode":"everyone"}}`)
	keylessHold(t, a, "wall", "?instance=Lobby")
	keylessRow(t, a, id, cy, "Lobby")

	reads := map[string]map[string]any{
		"editor read":     panelOf(t, a, id, cy),
		"non-editor read": panelOf(t, a, id, bo),
	}
	for who, key := range map[string]string{"editor": cy, "non-editor": bo} {
		rec := apiSend(a, "GET", "/api/v1/panels", key, "")
		for _, p := range apiBody(t, rec)["panels"].([]any) {
			if m := p.(map[string]any); m["id"] == id {
				reads[who+" list"] = m
			}
		}
	}
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/copy", bo, "")
	if rec.Code != 201 {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body.String())
	}
	reads["copy"] = apiBody(t, rec)

	if len(reads) != 5 {
		t.Fatalf("got %d reads, want 5: the list left the panel out", len(reads))
	}
	for _, k := range []string{"keyless", "instances", "hasKey", "clientRights", "artDefaults", "rights"} {
		if _, ok := reads["editor read"][k]; !ok {
			t.Errorf("editor read has no %s, so this test no longer checks it", k)
		}
	}
	for name, m := range reads {
		var unknown []string
		for k := range m {
			if !slices.Contains(panelKnown, k) {
				unknown = append(unknown, k)
			}
		}
		sort.Strings(unknown)
		if len(unknown) > 0 {
			t.Errorf("%s adds %v, missing from panelKnown (panelstore.go)", name, unknown)
		}
	}
}

func TestPanelKnownListsEveryStoredField(t *testing.T) {
	ty := reflect.TypeFor[Panel]()
	for i := range ty.NumField() {
		name, _, _ := strings.Cut(ty.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" && !slices.Contains(panelKnown, name) {
			t.Errorf("Panel.%s (%q) is missing from panelKnown", ty.Field(i).Name, name)
		}
	}
}

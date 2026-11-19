package app

import (
	"fmt"
	"strings"
	"testing"
)

func keylessRenameBody(id any, name string) string {
	return fmt.Sprintf(`{"keyless":%q,"name":%q}`, id, name)
}

func TestRenameAKeylessPage(t *testing.T) {
	a := panelsApp(t)
	defer startLog(t, a)()
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	id := makePanel(t, a, ana, `{"name":"Studio A","slug":"studio-a","share":{"mode":"everyone"}}`)
	page, _ := keylessHold(t, a, "studio-a", "?instance=Wall")
	row := keylessRow(t, a, id, cy, "Wall")

	// Trusted only, as for an Assign: the owner is not enough for a page with no key.
	wantErr(t, apiSend(a, "POST", renamePath(id), ana, keylessRenameBody(row["keyless"], "Porch")), 403, "trusted_required")
	silent(t, page)

	rec := apiSend(a, "POST", renamePath(id), cy, keylessRenameBody(row["keyless"], " Porch "))
	if rec.Code != 200 || apiBody(t, rec)["instance"] != "Porch" {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if msg := wantLine(t, page, "rename"); msg["instance"] != "Porch" {
		t.Fatalf("message %v", msg)
	}
	log := lastRow(t, a, "rename")
	if !strings.Contains(log.By, "Cy") || !strings.Contains(log.Note, "Wall") || !strings.Contains(log.Note, "Porch") || !strings.Contains(log.Note, "at 127.0.0.1") {
		t.Fatalf("row %+v", log)
	}
	keylessHold(t, a, "studio-a", "?instance=Porch")
	keylessRow(t, a, id, cy, "Porch")
	for _, r := range keylessRows(t, a, id, cy) {
		if r["device"] == "Wall" {
			t.Fatalf("the old row stays: %v", r)
		}
	}
	wantErr(t, apiSend(a, "POST", renamePath(id), cy, keylessRenameBody(row["keyless"], "Gate")), 409, "offline")
}

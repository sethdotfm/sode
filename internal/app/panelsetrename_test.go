package app

import (
	"path/filepath"
	"testing"
)

// A Me set's file is named for its owner, so a rename moves the set and a later user with the old name gets none.
func TestCheckpointSetsFollowAUserRename(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	s := loadPanelSets(cfg)
	set, err := s.create(PanelSet{Name: "Show", Scope: panelShareMe, Owner: "Cy"})
	if err != nil {
		t.Fatal(err)
	}
	s.followUsers(map[string]string{"cy": "Cyrus"})
	back := loadPanelSets(cfg)
	if got := back.visible("Cyrus"); len(got) != 1 || got[0].ID != set.ID || got[0].Owner != "Cyrus" {
		t.Fatalf("Cyrus sees %+v", got)
	}
	if got := back.visible("Cy"); len(got) != 0 {
		t.Fatalf("the old name still sees %+v", got)
	}
}

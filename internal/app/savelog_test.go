package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func saveRow(by string) Sent {
	e := event(kindSettings, "save", by, "ok")
	e.Save = &SettingsSave{Fields: []SavedField{{Key: "brandText", Old: []byte(`"a"`), New: []byte(`"b"`)}}}
	return e
}

func TestSaveLogKeepsRowsAcrossRestartAndDropsOld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings-saves.jsonl")
	c := &clock{time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	l, err := openSaveLog(path, c.now, keepBytes)
	if err != nil {
		t.Fatal(err)
	}
	for i, by := range []string{"Ada", "Bo"} {
		e := saveRow(by)
		e.Time, e.Seq = c.t, int64(10*(i+1))
		if err := l.Append(&e); err != nil {
			t.Fatal(err)
		}
		c.add(200 * 24 * time.Hour)
	}
	// Ada's row is now 400 days old, Bo's 200.
	l2, err := openSaveLog(path, c.now, keepBytes)
	if err != nil {
		t.Fatal(err)
	}
	rows := l2.Between(-1, 1<<62)
	if len(rows) != 1 || rows[0].By != "Bo" || rows[0].Save == nil || rows[0].ID != 2 {
		t.Fatalf("rows after 400 days = %+v", rows)
	}
	e := saveRow("Cy")
	if err := l2.Append(&e); err != nil || e.ID != 3 {
		t.Fatalf("next id = %d, %v", e.ID, err)
	}
}

func TestSaveLogDropsOldestOverSizeCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings-saves.jsonl")
	c := &clock{time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	l, err := openSaveLog(path, c.now, 2000)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 40 {
		e := saveRow("Ada")
		e.Time, e.Seq = c.t, int64(i)
		if err := l.Append(&e); err != nil {
			t.Fatal(err)
		}
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Size() > 2000 {
		t.Fatalf("file size %v over the cap: %v", fi.Size(), err)
	}
	rows := l.Between(-1, 1<<62)
	if len(rows) == 0 || rows[0].Seq != 39 || rows[len(rows)-1].Seq == 0 {
		t.Fatalf("kept %d rows, newest %d", len(rows), rows[0].Seq)
	}
	// Anchors in (lo, hi], newest first.
	got := l.Between(36, 38)
	if len(got) != 2 || got[0].Seq != 38 || got[1].Seq != 37 {
		t.Fatalf("Between(36, 38) = %+v", got)
	}
}

func TestUptimeDropsOldestOverSizeCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uptime.jsonl")
	c := &clock{time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
	u, err := openUptimeCap(path, c.now, 600)
	if err != nil {
		t.Fatal(err)
	}
	for range 40 {
		c.add(time.Minute)
		u.State(true)
		c.add(time.Minute)
		u.State(false)
	}
	u.flush()
	v, err := openUptimeCap(path, c.now, 600)
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	if fi.Size() > 600 {
		t.Fatalf("uptime file %d bytes, over the cap", fi.Size())
	}
	if n := len(v.trans); n < 3 || v.trans[n-1].S != upDown {
		t.Fatalf("kept %d changes", n)
	}
}

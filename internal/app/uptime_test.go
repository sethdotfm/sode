package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// clock is a settable time source.
type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }
func ms(t time.Time) int64           { return t.UnixMilli() }
func openAt(t *testing.T, path string, c *clock) *Uptime {
	t.Helper()
	u, err := openUptime(path, c.now)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestUptimeRangeSlotsAndCurrentMinute(t *testing.T) {
	c := &clock{time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
	u := openAt(t, "", c)
	start := c.t
	c.add(time.Minute)
	u.State(true)
	c.add(10 * time.Minute) // 12:11
	u.Note(Sent{Result: "nak", Tries: 3})
	u.Note(Sent{Result: "ack", Tries: 1, AckMs: slowAckMs + 1})
	u.Note(Sent{Result: "ack", Tries: 1, AckMs: 5}) // fine: not counted
	c.add(20 * time.Minute)                         // 12:31
	u.State(false)

	r := u.Range(ms(start), ms(start.Add(time.Hour)), 15*60_000)
	if r.Start != upDown {
		t.Fatalf("start = %q", r.Start)
	}
	if len(r.Trans) != 2 || r.Trans[0].S != upUp || r.Trans[1].S != upDown {
		t.Fatalf("trans = %+v", r.Trans)
	}
	// 12:11 is in the first 15 minute slot.
	if len(r.Counts) != 1 || r.Counts[0] != (UptimeSlot{I: 0, Bad: 1, Slow: 1}) {
		t.Fatalf("counts = %+v", r.Counts)
	}
	// Before anything was recorded there is no state.
	if r := u.Range(ms(start.Add(-time.Hour)), ms(start.Add(-time.Minute)), 60_000); r.Start != "" || len(r.Trans) != 0 {
		t.Fatalf("before the record: %+v", r)
	}
}

func TestUptimeReopenAfterCrashIsDownFromLastAlive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "uptime.jsonl")
	c := &clock{time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
	u := openAt(t, path, c)
	c.add(time.Minute)
	u.State(true)
	c.add(time.Minute)
	u.Note(Sent{Result: "noack", Tries: 3})
	c.add(2 * time.Minute) // 12:04
	u.flush()
	u.writeAlive()
	// Killed: no off line. Sode comes back an hour later.
	c.add(time.Hour)
	v := openAt(t, path, c)

	var states []string
	for _, tr := range v.trans {
		states = append(states, tr.S)
	}
	if got := strings.Join(states, ","); got != "down,up,off,down" {
		t.Fatalf("states = %s", got)
	}
	if off := v.trans[2]; off.T != ms(time.Date(2026, 10, 29, 12, 4, 0, 0, time.UTC)) {
		t.Fatalf("off at %v, want the alive stamp", time.UnixMilli(off.T).UTC())
	}
	if len(v.mins) != 1 || v.mins[0].Bad != 1 {
		t.Fatalf("minutes = %+v", v.mins)
	}
}

func TestUptimeCleanStopIsNotCutShort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "u.jsonl")
	c := &clock{time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
	u := openAt(t, path, c)
	c.add(time.Minute)
	u.setState(upOff, c.t)
	u.flush()
	c.add(time.Hour)
	v := openAt(t, path, c)
	if n := len(v.trans); n != 3 || v.trans[1].S != upOff || v.trans[1].T != ms(c.t.Add(-time.Hour)) {
		t.Fatalf("trans = %+v", v.trans)
	}
}

func TestUptimePrunesOldLinesKeepingTheStateBefore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "u.jsonl")
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	old := now.Add(-uptimeKeep - 48*time.Hour)
	body := `{"t":` + itoa(ms(old)) + `,"s":"up"}` + "\n" +
		`{"m":` + itoa(ms(old)/60_000) + `,"bad":2}` + "\n" +
		"garbage\n" +
		`{"t":` + itoa(ms(now.Add(-time.Hour))) + `,"s":"off"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	u := openAt(t, path, &clock{now})
	if len(u.mins) != 0 {
		t.Fatalf("old minute kept: %+v", u.mins)
	}
	// The up from before the cutoff stays: it is the state the oldest range starts in.
	if len(u.trans) != 3 || u.trans[0].S != upUp {
		t.Fatalf("trans = %+v", u.trans)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "garbage") {
		t.Fatal("damaged line was written back")
	}
}

func TestUptimeRangeClampedAndSlotsBounded(t *testing.T) {
	c := &clock{time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
	u := openAt(t, "", c)
	for i := 0; i < 5; i++ {
		u.Note(Sent{Result: "nak", Tries: 1})
		c.add(time.Minute)
	}
	// A ten year range with 1h slots is cut to what is kept, not 87,600 slots.
	r := u.Range(ms(c.t.AddDate(-10, 0, 0)), ms(c.t), 3600_000)
	if len(r.Counts) != 1 {
		t.Fatalf("counts = %+v", r.Counts)
	}
	if first := r.Counts[0].I; first < 0 || first > int(uptimeKeep/time.Hour) {
		t.Fatalf("slot index %d outside the kept range", first)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

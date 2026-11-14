package app

import (
	"errors"
	"slices"
	"sync"
	"testing"
)

// exits records the codes a Kill asked for, in place of os.Exit.
type exits struct {
	mu    sync.Mutex
	codes []int
	// rows: the log as it stood when exit ran, so the test sees the row came first.
	rows [][]Sent
	err  error // reading the log; exit may run off the test goroutine, so no t.Fatal there
}

func stubExit(t *testing.T, a *App) *exits {
	t.Helper()
	x := &exits{}
	a.exit = func(code int) {
		page, _, err := a.sent.Page(-1, 500)
		slices.Reverse(page)
		x.mu.Lock()
		defer x.mu.Unlock()
		x.codes = append(x.codes, code)
		x.rows = append(x.rows, page)
		x.err = errors.Join(x.err, err)
	}
	return x
}

func (x *exits) n() int {
	x.mu.Lock()
	defer x.mu.Unlock()
	return len(x.codes)
}

func killOn(t *testing.T, a *App, on bool) {
	t.Helper()
	cfg := a.r().cfg
	cfg.KillEnabled = on
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestKillRefusals(t *testing.T) {
	a := userApp(t, false)
	x := stubExit(t, a)
	c := &client{ip: "10.0.0.1"}

	if err := admitMsg(a, c, inMsg{T: "kill", Pin: "2468"}); !errors.Is(err, errLocked) {
		t.Fatalf("PIN locked on this connection: admit gave %v, want errLocked", err)
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.Kill(c, "2468"); !errors.Is(err, errKillOff) {
		t.Fatalf("setting off: %v", err)
	}

	killOn(t, a, true)
	before := a.pin.n
	if err := a.Kill(c, "0000"); err == nil || errors.Is(err, errKillOff) {
		t.Fatalf("wrong PIN: %v", err)
	}
	if a.pin.n != before+1 {
		t.Fatalf("a wrong PIN in a Kill did not count: %d tries, want %d", a.pin.n, before+1)
	}
	if x.n() != 0 {
		t.Fatalf("a refused Kill exited: %v", x.codes)
	}

	// No PIN on the install: admit lets needAdmin through, so Kill itself refuses.
	b := pinApp(t)
	xb := stubExit(t, b)
	killOn(t, b, true)
	cb := &client{ip: "10.0.0.2"}
	if err := admitMsg(b, cb, inMsg{T: "kill"}); err != nil {
		t.Fatalf("no PIN: admit %v", err)
	}
	if err := b.Kill(cb, ""); !errors.Is(err, errKillNoPin) {
		t.Fatalf("no PIN set: %v", err)
	}
	if xb.n() != 0 {
		t.Fatal("Kill with no PIN set exited")
	}
}

func TestKillLogsThenExitsZero(t *testing.T) {
	a := userApp(t, false)
	stop := startLog(t, a)
	defer stop()
	x := stubExit(t, a)
	killOn(t, a, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := admitMsg(a, c, inMsg{T: "kill", Pin: "2468"}); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if err := a.Kill(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if len(x.codes) != 1 || x.codes[0] != 0 || x.err != nil {
		t.Fatalf("exit codes %v, want [0] (log read: %v)", x.codes, x.err)
	}
	var row *Sent
	for i, e := range x.rows[0] {
		if e.Kind == kindCommand && e.Why == "kill" {
			row = &x.rows[0][i]
		}
	}
	if row == nil {
		t.Fatalf("no kill row in the log before exit: %+v", x.rows[0])
	}
	if row.By != "Ana" || row.Result != "ok" {
		t.Fatalf("kill row %+v, want By Ana, result ok", *row)
	}
}

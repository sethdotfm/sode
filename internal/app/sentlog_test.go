package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seqs(page []Sent) []int64 {
	out := make([]int64, len(page))
	for i, e := range page {
		out[i] = e.Seq
	}
	return out
}

// Rotation keeps the two files under the cap, pages run across them, and
// a reopened log carries on numbering where it stopped.
func TestSentLogRotateAndReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sent.jsonl")
	const capBytes = 4000
	l, err := OpenSentLog(path, capBytes)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if err := l.Append(&Sent{Kind: "connect", Dst: i, Src: -1, Wire: -1, Result: "ack"}); err != nil {
			t.Fatal(err)
		}
	}
	var total int64
	for _, p := range []string{path, filepath.Join(dir, "sent.1.jsonl")} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		total += st.Size()
	}
	if total > capBytes {
		t.Fatalf("files hold %d bytes, cap %d", total, capBytes)
	}

	// Page through everything that's left, newest first, without gaps.
	var all []Sent
	before := int64(-1)
	for {
		page, more, err := l.Page(before, 7)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, page...)
		if !more {
			break
		}
		before = page[len(page)-1].Seq
	}
	if len(all) == 0 || all[0].Seq != 99 || all[0].Dst != 99 {
		t.Fatalf("newest = %+v", all[:1])
	}
	for i := 1; i < len(all); i++ {
		if all[i].Seq != all[i-1].Seq-1 {
			t.Fatalf("gap in pages: %v", seqs(all))
		}
	}
	if all[len(all)-1].Seq == 0 {
		t.Fatal("nothing was rotated away")
	}
	// The total counts what's left on disk, not everything ever appended.
	if got := l.Total(); got != int64(len(all)) {
		t.Fatalf("Total = %d, want %d", got, len(all))
	}

	// A torn write is dropped on reopen and numbering carries on.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"seq":100,"kind":`)
	f.Close()
	l2, err := OpenSentLog(path, capBytes)
	if err != nil {
		t.Fatal(err)
	}
	e := &Sent{Kind: "connect"}
	if err := l2.Append(e); err != nil || e.Seq != 100 {
		t.Fatalf("after reopen seq %d err %v, want 100", e.Seq, err)
	}
	if got := l2.Total(); got != int64(len(all))+1 {
		t.Fatalf("Total after reopen = %d, want %d", got, len(all)+1)
	}
	page, _, err := l2.Page(-1, len(all)+1)
	if err != nil || len(page) != len(all)+1 || page[0].Seq != 100 || page[1].Seq != 99 {
		t.Fatalf("after reopen %v err %v", seqs(page), err)
	}
}

func TestSentLogMemoryCap(t *testing.T) {
	l, _ := OpenSentLog("", 0)
	for i := 0; i < 2*sentMem+1; i++ {
		l.Append(&Sent{})
	}
	page, more, _ := l.Page(-1, 3)
	if len(page) != 3 || page[0].Seq != 2*sentMem || !more {
		t.Fatalf("newest %v more %v", seqs(page), more)
	}
	page, more, _ = l.Page(sentMem+3, 10)
	if len(page) != 2 || page[1].Seq != sentMem+1 || more {
		t.Fatalf("oldest %v more %v, want %d, %d and no more", seqs(page), more, sentMem+2, sentMem+1)
	}
}

// A damaged file is set aside, not fatal; the log starts afresh.
func TestSentLogDamagedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sent.jsonl")
	l, _ := OpenSentLog(path, 1<<20)
	for i := 0; i < 3; i++ {
		l.Append(&Sent{Kind: "connect"})
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("\x00\x00garbage\n")
	f.Close()
	l2, err := OpenSentLog(path, 1<<20)
	if err != nil {
		t.Fatalf("damaged file stopped the open: %v", err)
	}
	bad, _ := filepath.Glob(path + ".bad-*")
	if len(bad) != 1 {
		t.Fatalf("quarantined files %v, want one", bad)
	}
	if err := l2.Append(&Sent{Kind: "connect"}); err != nil {
		t.Fatal(err)
	}
	if page, _, _ := l2.Page(-1, 10); len(page) != 1 {
		t.Fatalf("after quarantine %v, want just the new entry", seqs(page))
	}
}

// One unreadable line in the middle doesn't hide the others.
func TestSentLogBadMiddleLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sent.jsonl")
	l, _ := OpenSentLog(path, 1<<20)
	for i := 0; i < 3; i++ {
		l.Append(&Sent{Kind: "connect"})
	}
	// Garble the middle line in place, keeping its length.
	l.mu.Lock()
	off, end := l.cur.offs[1], l.cur.offs[2]
	l.mu.Unlock()
	f, _ := os.OpenFile(path, os.O_WRONLY, 0)
	f.WriteAt([]byte(strings.Repeat("#", int(end-off)-1)), off)
	f.Close()
	page, _, err := l.Page(-1, 10)
	if err != nil || len(page) != 2 || page[0].Seq != 2 || page[1].Seq != 0 {
		t.Fatalf("page %v err %v, want 2 and 0", seqs(page), err)
	}
}

// A failed write doesn't give out its seq: the next entry gets it, so
// seqs stay unique and the file stays gapless.
func TestSentLogWriteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sent.jsonl")
	l, _ := OpenSentLog(path, 1<<20)
	l.Append(&Sent{Kind: "connect"})
	l.mu.Lock()
	l.cur.f.Close() // every write fails from here
	l.mu.Unlock()
	if err := l.Append(&Sent{Kind: "connect"}); err == nil {
		t.Fatal("write to a closed file succeeded")
	}
	l2, err := OpenSentLog(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	e := &Sent{Kind: "connect"}
	if err := l2.Append(e); err != nil || e.Seq != 1 {
		t.Fatalf("seq %d err %v, want 1", e.Seq, err)
	}
}

func TestSentLogTotalMemory(t *testing.T) {
	l, _ := OpenSentLog("", 0)
	for i := 0; i < 5; i++ {
		l.Append(&Sent{Kind: "connect"})
	}
	if got := l.Total(); got != 5 {
		t.Fatalf("Total = %d, want 5", got)
	}
}

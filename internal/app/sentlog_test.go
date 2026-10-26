package app

import (
	"os"
	"path/filepath"
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
	page, _, err := l2.Page(-1, len(all)+1)
	if err != nil || len(page) != len(all)+1 || page[0].Seq != 100 || page[1].Seq != 99 {
		t.Fatalf("after reopen %v err %v", seqs(page), err)
	}
}

func TestSentLogMemoryCap(t *testing.T) {
	l, _ := OpenSentLog("", 0)
	for i := 0; i < sentMem+5; i++ {
		l.Append(&Sent{})
	}
	page, more, _ := l.Page(-1, 3)
	if len(page) != 3 || page[0].Seq != sentMem+4 || !more {
		t.Fatalf("newest %v more %v", seqs(page), more)
	}
	page, more, _ = l.Page(7, 10)
	if len(page) != 2 || page[1].Seq != 5 || more {
		t.Fatalf("oldest %v more %v, want 6, 5 and no more", seqs(page), more)
	}
}

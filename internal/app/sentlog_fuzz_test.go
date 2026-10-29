package app

import (
	"path/filepath"
	"testing"
)

// Every Page(before, n) must return exactly the n entries just below
// before (or what's left), whatever the rotation state or reopen.
func TestSentLogPageAllStates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sent.jsonl")
	const capBytes = 3000
	l, err := OpenSentLog(path, capBytes)
	if err != nil {
		t.Fatal(err)
	}
	check := func(l *SentLog) {
		t.Helper()
		oldest := l.next - l.Total()
		for before := int64(-1); before <= l.next+1; before++ {
			for _, n := range []int{1, 3, 7, 100} {
				page, more, err := l.Page(before, n)
				if err != nil {
					t.Fatalf("before %d n %d: %v", before, n, err)
				}
				top := before
				if top < 0 || top > l.next {
					top = l.next
				}
				want := min(int64(n), top-oldest)
				if want < 0 {
					want = 0
				}
				if int64(len(page)) != want {
					t.Fatalf("before %d n %d: got %d entries %v, want %d (oldest %d next %d)", before, n, len(page), seqs(page), want, oldest, l.next)
				}
				for i, e := range page {
					if e.Seq != top-1-int64(i) {
						t.Fatalf("before %d n %d: seqs %v", before, n, seqs(page))
					}
				}
				if wantMore := top-want > oldest; more != wantMore {
					t.Fatalf("before %d n %d: more=%v want %v (%v)", before, n, more, wantMore, seqs(page))
				}
			}
		}
	}
	for i := 0; i < 120; i++ {
		if err := l.Append(&Sent{Kind: "connect", Dst: i, Src: -1, Wire: -1, Result: "ack"}); err != nil {
			t.Fatal(err)
		}
		if i%5 == 0 {
			check(l)
		}
		if i%37 == 36 {
			l.cur.f.Close()
			if l.old != nil {
				l.old.f.Close()
			}
			if l, err = OpenSentLog(path, capBytes); err != nil {
				t.Fatal(err)
			}
			check(l)
		}
	}
}

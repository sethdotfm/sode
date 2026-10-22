package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryPagingAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.jsonl")
	h, err := OpenHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		if err := h.Append(&Entry{Src: i, OK: true}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := h.Page(-1, 10)
	if err != nil || len(page) != 10 || page[0].ID != 24 || page[9].ID != 15 || page[0].Src != 24 {
		t.Fatalf("first page %+v err %v", page, err)
	}
	page, _ = h.Page(5, 10)
	if len(page) != 5 || page[4].ID != 0 {
		t.Fatalf("last page %+v", page)
	}

	// Simulate a torn write, then reopen.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"id":25,"src":`)
	f.Close()
	h2, err := OpenHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if h2.Len() != 25 {
		t.Fatalf("reopened len %d", h2.Len())
	}
	h2.Append(&Entry{Src: 99})
	page, _ = h2.Page(-1, 1)
	if page[0].ID != 25 || page[0].Src != 99 {
		t.Fatalf("after reopen %+v", page)
	}
}

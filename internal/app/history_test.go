package app

import (
	"os"
	"path/filepath"
	"slices"
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

func TestHistoryClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.jsonl")
	h, _ := OpenHistory(path)
	for i := 0; i < 3; i++ {
		h.Append(&Entry{Src: i})
	}
	if err := h.Clear(); err != nil {
		t.Fatal(err)
	}
	if h.Len() != 0 {
		t.Fatalf("len after clear %d", h.Len())
	}
	e := &Entry{Src: 7}
	h.Append(e)
	h2, _ := OpenHistory(path)
	page, _ := h2.Page(-1, 10)
	if e.ID != 0 || len(page) != 1 || page[0].Src != 7 {
		t.Fatalf("after clear+append: id %d, page %+v", e.ID, page)
	}
}

func TestHistoryPageKeepsGroups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.jsonl")
	h, err := OpenHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	// IDs 0-1 ungrouped, 2-5 one group, 6 ungrouped.
	for i := 0; i < 7; i++ {
		e := &Entry{Dst: i}
		if i >= 2 && i <= 5 {
			e.Group, e.GroupN = "g", 4
		}
		h.Append(e)
	}
	// A page of 2 from the top ends inside the group, so it runs on to ID 2.
	page, _ := h.Page(-1, 2)
	if len(page) != 5 || page[4].ID != 2 {
		t.Fatalf("page %+v", page)
	}
	page, _ = h.Page(2, 2)
	if len(page) != 2 || page[1].ID != 0 {
		t.Fatalf("next page %+v", page)
	}
	h2, err := OpenHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	page, _ = h2.Page(4, 1)
	if page[0].Group != "g" || page[0].GroupN != 4 {
		t.Fatalf("after reopen %+v", page[0])
	}
}

func TestHistoryUpdate(t *testing.T) {
	for _, name := range []string{"", "h.jsonl"} {
		path := name
		if path != "" {
			path = filepath.Join(t.TempDir(), name)
		}
		h, err := OpenHistory(path)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 5; i++ {
			h.Append(&Entry{Src: i, Failed: []int{1, 2}, Err: "short"})
		}
		long := "a much longer error than the one it replaces"
		e, ok := h.Update(2, func(e *Entry) { e.Failed, e.Late, e.Err, e.ID = []int{2}, []int{1}, long, 99 })
		if !ok || e.ID != 2 || e.Err != long {
			t.Fatalf("%q: update %+v %v", name, e, ok)
		}
		if _, ok := h.Update(5, func(*Entry) {}); ok {
			t.Fatal("updated an entry that isn't there")
		}
		if _, ok := h.Update(1, func(*Entry) {}); !ok {
			t.Fatal("no-op update failed")
		}
		check := func(h *History) {
			t.Helper()
			page, err := h.Page(-1, 10)
			if err != nil || len(page) != 5 {
				t.Fatalf("%q: page %+v err %v", name, page, err)
			}
			for i, e := range page {
				id := 4 - i
				if e.ID != id || e.Src != id {
					t.Fatalf("%q: entry %d is %+v", name, id, e)
				}
				if (id == 2) != (e.Err == long) || (id == 2) != (len(e.Late) == 1) {
					t.Fatalf("%q: entry %d amended wrongly: %+v", name, id, e)
				}
			}
		}
		check(h)
		h.Append(&Entry{Src: 5})
		if h.Len() != 6 {
			t.Fatalf("%q: len %d", name, h.Len())
		}
		h.Update(5, func(e *Entry) { e.Err = long })
		if path == "" {
			continue
		}
		h2, err := OpenHistory(path)
		if err != nil || h2.Len() != 6 {
			t.Fatalf("reopen: %v, len %d", err, h2.Len())
		}
		page, _ := h2.Page(-1, 10)
		if page[0].ID != 5 || page[0].Err != long || page[3].Err != long || page[3].ID != 2 {
			t.Fatalf("reopened %+v", page)
		}
		if m, _ := filepath.Glob(path + ".tmp-*"); len(m) != 0 {
			t.Fatalf("temp files left: %v", m)
		}
	}
}

// An entry handed out before an Update keeps its own slices: the update
// must not change them under a reader (a broadcast being marshalled).
func TestHistoryUpdateMemoryDoesNotAlias(t *testing.T) {
	h, _ := OpenHistory("")
	h.Append(&Entry{Failed: []int{1, 2, 3}, Now: [][2]int{{1, 5}}})
	before, _ := h.Page(-1, 1)
	h.Update(0, func(e *Entry) {
		e.Failed = slices.DeleteFunc(e.Failed, func(x int) bool { return x == 2 })
		e.Late = append(e.Late, 2)
	})
	if !slices.Equal(before[0].Failed, []int{1, 2, 3}) || len(before[0].Late) != 0 {
		t.Fatalf("earlier copy changed: %+v", before[0])
	}
	after, _ := h.Page(-1, 1)
	if !slices.Equal(after[0].Failed, []int{1, 3}) {
		t.Fatalf("update lost: %+v", after[0])
	}
}

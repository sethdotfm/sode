package app

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Retention of the Settings saves record and the uptime graph: whichever limit comes first.
const (
	keepAge   = 365 * 24 * time.Hour
	keepBytes = 5 << 20
)

const kindSettings = "settings" // a Settings save; Seq is its anchor, ID its own number

// Preset: the Access preset staged into the save. Interruptions: the review's lines at save time.
type SettingsSave struct {
	Fields        []SavedField `json:"fields"`
	Approver      string       `json:"approver,omitempty"`
	Preset        string       `json:"preset,omitempty"`
	Critical      bool         `json:"critical,omitempty"`
	Interruptions []string     `json:"interruptions,omitempty"`
	// HiddenInterruptions: how many lines a reader below Administrator does not see. They name
	// users, Public panels and sessions from Access (forReader).
	HiddenInterruptions int         `json:"hiddenInterruptions,omitempty"`
	Import              *ImportSave `json:"import,omitempty"` // an import: counts only, no values
}

// SavedField: one written field. A list gives Added and Removed in place of Old and New. Changed
// with no values: a secret, or a value the reader's Role cannot see.
type SavedField struct {
	Topic     string            `json:"topic"`
	Section   string            `json:"section"`
	SectionID string            `json:"sectionId"`
	Key       string            `json:"key"`
	Label     string            `json:"label"`
	Old       json.RawMessage   `json:"old,omitempty"`
	New       json.RawMessage   `json:"new,omitempty"`
	Added     []json.RawMessage `json:"added,omitempty"`
	Removed   []json.RawMessage `json:"removed,omitempty"`
	Changed   bool              `json:"changed,omitempty"`
	Note      string            `json:"note,omitempty"`
}

// Its own JSONL file, so keepAge applies apart from the Communication Log's size cap. Seq is the next Communication Log
// seq at write time; the Log merges rows by it (docs/agents/comm-log.md).
type SaveLog struct {
	mu       sync.Mutex
	path     string
	now      func() time.Time
	capBytes int
	rows     []Sent // oldest first
	sizes    []int  // line length of each row
	size     int
	next     int64
}

func OpenSaveLog(path string) (*SaveLog, error) { return openSaveLog(path, time.Now, keepBytes) }

func openSaveLog(path string, now func() time.Time, capBytes int) (*SaveLog, error) {
	l := &SaveLog{path: path, now: now, capBytes: capBytes, next: 1}
	if path == "" {
		return l, nil
	}
	f, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	bad := 0
	if err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1<<16), 1<<22)
		for sc.Scan() {
			var e Sent
			if json.Unmarshal(sc.Bytes(), &e) != nil || e.Kind != kindSettings {
				bad++
				continue
			}
			l.push(e, len(sc.Bytes())+1)
		}
		f.Close()
		if bad > 0 {
			log.Printf("settings saves: skipped %d damaged lines in %s", bad, path)
		}
	}
	if l.prune() || bad > 0 {
		if err := l.rewrite(); err != nil {
			return nil, err
		}
	}
	return l, nil
}

func (l *SaveLog) push(e Sent, n int) {
	l.rows = append(l.rows, e)
	l.sizes = append(l.sizes, n)
	l.size += n
	l.next = max(l.next, e.ID+1)
}

// lowWater: over the cap, the oldest go until this much is left, so the file is not written
// again on every append.
func lowWater(capBytes int) int { return capBytes * 4 / 5 }

// Caller holds mu or owns l.
func (l *SaveLog) prune() bool {
	cut := l.now().Add(-keepAge)
	i := sort.Search(len(l.rows), func(i int) bool { return l.rows[i].Time.After(cut) })
	size := l.size
	for _, n := range l.sizes[:i] {
		size -= n
	}
	if size > l.capBytes {
		for ; i < len(l.rows) && size > lowWater(l.capBytes); i++ {
			size -= l.sizes[i]
		}
	}
	if i == 0 {
		return false
	}
	for _, n := range l.sizes[:i] {
		l.size -= n
	}
	l.rows = append([]Sent(nil), l.rows[i:]...)
	l.sizes = append([]int(nil), l.sizes[i:]...)
	return true
}

func (l *SaveLog) rewrite() error {
	var b []byte
	for _, e := range l.rows {
		line, _ := json.Marshal(e)
		b = append(append(b, line...), '\n')
	}
	return config.AtomicWrite(l.path, b)
}

// Over the cap or past keepAge, the file is written again without the oldest rows.
func (l *SaveLog) Append(e *Sent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.ID = l.next
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	l.push(*e, len(line))
	if l.path == "" {
		l.prune()
		return nil
	}
	if l.prune() {
		err = l.rewrite()
	} else {
		err = appendFile(l.path, line)
	}
	if err != nil { // not saved, so not shown: the id goes to the next row
		n := len(l.rows) - 1
		l.size -= l.sizes[n]
		l.rows, l.sizes = l.rows[:n], l.sizes[:n]
	}
	return err
}

func appendFile(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// Reanchor moves rows anchored past next to oldest, the first seq. Then a Communication Log that
// started its seqs again does not show them as the newest rows.
func (l *SaveLog) Reanchor(next, oldest int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	moved := false
	for i := range l.rows {
		if l.rows[i].Seq > next {
			l.rows[i].Seq, moved = oldest, true
		}
	}
	if moved && l.path != "" {
		if err := l.rewrite(); err != nil {
			log.Printf("settings saves: %v", err)
		}
	}
}

// Between: rows with lo < Seq <= hi, newest first.
func (l *SaveLog) Between(lo, hi int64) []Sent {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Sent
	for i := len(l.rows) - 1; i >= 0; i-- {
		if s := l.rows[i].Seq; s > lo && s <= hi {
			out = append(out, l.rows[i])
		}
	}
	return out
}

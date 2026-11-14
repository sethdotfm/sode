package app

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// defaultDrainLimit: a take's check (1s), route (3s) and grace (1s), plus margin. main waits
// a little longer than this for Run.
const defaultDrainLimit = 6 * time.Second

//lint:ignore ST1005 the UI shows this text as is
var errShuttingDown = errors.New("Sode is shutting down; try again once it is back")

// inflight counts the writes started from browser messages (takes and saves), so shutdown
// can wait for them. Adding and closing share mu, so no Add can race the Wait.
type inflight struct {
	mu    sync.Mutex
	wg    sync.WaitGroup
	shut  bool
	kinds map[string]int
}

// start counts one write of kind; false once shutdown began. done may be called more than once.
func (f *inflight) start(kind string) (done func(), ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.shut {
		return nil, false
	}
	if f.kinds == nil {
		f.kinds = map[string]int{}
	}
	f.wg.Add(1)
	f.kinds[kind]++
	var once sync.Once
	return func() {
		once.Do(func() {
			f.mu.Lock()
			if f.kinds[kind]--; f.kinds[kind] == 0 {
				delete(f.kinds, kind)
			}
			f.mu.Unlock()
			f.wg.Done()
		})
	}, true
}

// close: Run selects on the channel; shut is set under mu, so no start can slip past the Wait.
func (f *inflight) close() <-chan struct{} {
	f.mu.Lock()
	f.shut = true
	f.mu.Unlock()
	ch := make(chan struct{})
	go func() { f.wg.Wait(); close(ch) }()
	return ch
}

// left: for the log line when the limit cuts the wait short ("batch 2, config 1").
func (f *inflight) left() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k, n := range f.kinds {
		out = append(out, fmt.Sprintf("%s %d", k, n))
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// finish: nil-safe, since tests call handlers directly without the read loop counting them.
func (m *inMsg) finish() {
	if m.done != nil {
		m.done()
	}
}

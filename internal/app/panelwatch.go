package app

import (
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// Watches live in panelwatch.yaml, never in a panel file, so a panel save cannot drop another user's watch.

// Caller holds mu.
func (m *panelMon) saveLocked() error {
	if m.path == "" {
		return nil
	}
	by := map[string][]string{}
	for u, set := range m.watch {
		if len(set) > 0 {
			by[u] = slices.Sorted(maps.Keys(set))
		}
	}
	b, err := yaml.Marshal(by)
	if err != nil {
		return err
	}
	return config.AtomicWrite(m.path, b)
}

// watchKey names one watch: a panel id alone for the whole panel, or "<id>/<device lowercased>" for
// one instance. A panel id has no slash, so the first one splits them. Stored as is in panelwatch.yaml,
// which is why an older file of plain ids still loads.
func watchKey(id, inst string) string {
	if inst = strings.ToLower(strings.TrimSpace(inst)); inst != "" {
		return id + "/" + inst
	}
	return id
}

func splitWatchKey(k string) (id, inst string) {
	id, inst, _ = strings.Cut(k, "/")
	return id, inst
}

func (m *panelMon) watching(user, id string) bool { return m.watchingInst(user, id, "") }

func (m *panelMon) watchingInst(user, id, inst string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watch[strings.ToLower(user)][watchKey(id, inst)]
}

func (m *panelMon) watchedInstances(user, id string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for k := range m.watch[strings.ToLower(user)] {
		if wid, inst := splitWatchKey(k); wid == id && inst != "" {
			out = append(out, inst)
		}
	}
	slices.Sort(out)
	return out
}

func (m *panelMon) setWatchInst(user, id, inst string, on bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, k := strings.ToLower(user), watchKey(id, inst)
	if on {
		if m.watch[u] == nil {
			m.watch[u] = map[string]bool{}
		}
		m.watch[u][k] = true
	} else {
		delete(m.watch[u], k)
	}
	return m.saveLocked()
}

// watchersOf maps, by lowercased name, who is told about a device of panel id. The value is true for
// a whole-panel watch and false for one that names only this device.
func (m *panelMon) watchersOf(id, device string) map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]bool{}
	for u, set := range m.watch {
		if set[id] {
			out[u] = true
		} else if set[watchKey(id, device)] {
			out[u] = false
		}
	}
	return out
}

// clearAlarm hides one watch's current outage from the user's banner and badge until the next
// notice about that panel.
func (m *panelMon) clearAlarm(user, id, inst string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := strings.ToLower(user)
	if m.clear[k] == nil {
		m.clear[k] = map[string]bool{}
	}
	m.clear[k][watchKey(id, inst)] = true
}

// unclear makes a new notice show again: the panel's own outage, and the device's when it has one.
func (m *panelMon) unclear(id, device string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unclearLocked(id, device)
}

// unclearLocked with no device drops every clear of the panel.
func (m *panelMon) unclearLocked(id, device string) {
	for _, set := range m.clear {
		if device == "" {
			for k := range set {
				if wid, _ := splitWatchKey(k); wid == id {
					delete(set, k)
				}
			}
			continue
		}
		delete(set, id)
		delete(set, watchKey(id, device))
	}
}

// Watches are keyed by user id, so a rename keeps them.
func (m *panelMon) dropUsers(removed map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	for id := range m.watch {
		if removed[id] {
			delete(m.watch, id)
			changed = true
		}
	}
	if changed {
		if err := m.saveLocked(); err != nil {
			log.Printf("panels: watches not saved: %v", err)
		}
	}
}

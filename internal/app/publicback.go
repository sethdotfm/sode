package app

import "slices"

// A Public panel page comes back by itself (docs/agents/panels.md, Public keys). When Public panels allowed turns
// on, or a panel's mode changes back to Public, every keyless stream whose address resolves to that panel hears a
// public line. The page waits a random 0 to 5 s and asks once; nothing polls, since publicAskMax counts per address.

var publicLine = []byte("{\"t\":\"public\"}\n")

// keylessRefs lists the address segments that hold a keyless stream now.
func (m *panelMon) keylessRefs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	var out []string
	for _, in := range m.keyless {
		if len(in.conns) > 0 && !seen[in.ref] {
			seen[in.ref] = true
			out = append(out, in.ref)
		}
	}
	return out
}

// tellPublic queues a public line on every keyless stream of refs. A line already queued stands for this one.
func (m *panelMon) tellPublic(refs map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, in := range m.keyless {
		if !refs[in.ref] {
			continue
		}
		for _, c := range in.conns {
			select {
			case c.pub <- struct{}{}:
			default:
			}
		}
	}
}

// publicAgain tells the keyless pages of the Public panels in ids, or of every Public panel when ids is empty.
// Call it after the change is in place, so an ask the line causes reads it.
func (a *App) publicAgain(ids ...string) {
	if !a.r().cfg.PublicPanels {
		return
	}
	refs := map[string]bool{}
	// Resolved with the monitor unlocked: the panel store has its own lock.
	for _, ref := range a.mon.keylessRefs() {
		p, ok := a.panels.get(a.panelIDOf(ref))
		if !ok || p.Acts != panelActsPublic || (len(ids) > 0 && !slices.Contains(ids, p.ID)) {
			continue
		}
		refs[ref] = true
	}
	if len(refs) > 0 {
		a.mon.tellPublic(refs)
	}
}

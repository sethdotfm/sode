package app

import "slices"

// docs/agents/panels.md, Public keys. The page waits a random 0 to 5 s and asks one time; nothing polls, since
// publicAskMax counts per address.

var publicLine = []byte("{\"t\":\"public\"}\n")

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

// A line already queued stands for this one.
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

// No ids means every Public panel. Call it after the change is in place, so the ask that the line causes reads it.
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

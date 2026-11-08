package app

import "github.com/sethdotfm/sode/internal/config"

// apiView is the one place a caller's visibility applies to reads. Ticket F makes hidden
// rights drop out-of-rights sources, destinations and their History entries here (ADR 0004);
// every read handler already asks it, so F changes only this file.
type apiView struct{ rights config.Rights }

func (c *apiCtx) view() apiView { return apiView{c.Rights} }

func (v apiView) Source(n int) bool       { return true }
func (v apiView) Destination(n int) bool  { return true }
func (v apiView) Preset(name string) bool { return true }

// Route: src < 0 means nothing routed, which is always visible.
func (v apiView) Route(dst, level, src int) bool {
	return v.Destination(dst) && (src < 0 || v.Source(src))
}

// Entry returns the History entry as this caller may see it, or false to drop it.
func (v apiView) Entry(e Entry) (Entry, bool) { return e, true }

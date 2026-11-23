package app

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Tie lines in the export file (docs/agents/profiles.md, docs/tielines-design.md, Other stores): their own category,
// tielines.yaml, picked by tie line id.

type exportTieLines struct {
	TieLines []config.TieLine `yaml:"tieLines"`
}

// exportTieLinesYAML writes each end's router as its prefix, also with one router, since a tie line names two. An end
// on a router that the file does not hold keeps the raw UID (toPrefixes).
func exportTieLinesYAML(cfg config.Config, pick map[string][]string, prefix map[string]string, used map[string]bool) ([]byte, error) {
	var out exportTieLines
	for _, t := range cfg.TieLines {
		if picked(pick, catTieLines, t.ID) {
			out.TieLines = append(out.TieLines, t)
		}
	}
	return exportYAML(&out, prefix, false, used)
}

// tieRouterHere gives the router here for the router UID of a file tie line end. It gives false when the import maps
// that router to no router. It is the one seam for the router mapping. A file router in the manifest goes where the
// remap step sends it (routerTo). A raw UID names a router here only when this server has it. When the import maps
// every file router (several routers), that mapping replaces this body.
func (p *importPlanner) tieRouterHere(uid string) (string, bool) {
	if to, inFile := p.routerTo[uid]; inFile {
		return to, to != ""
	}
	_, ok := p.live.RouterByUID(uid)
	return uid, ok
}

// tieRouterTitle names a file router for a note: its manifest title, else its UID.
func (p *importPlanner) tieRouterTitle(uid string) string {
	for _, r := range p.b.Manifest.Routers {
		if r.UID == uid {
			return cmp.Or(r.Title, r.Prefix, uid)
		}
	}
	return uid
}

// tieUnmapped is the note on a tie line that the import drops, since a router of an end is not mapped here.
func tieUnmapped(router string) string {
	return fmt.Sprintf("router %s is not mapped to a router here, so the tie line is not imported", router)
}

func sameTieLine(a, b config.TieLine) bool {
	return a.Name == b.Name && a.From.Router == b.From.Router && a.From.N == b.From.N && a.To.Router == b.To.Router && a.To.N == b.To.N &&
		a.From.Name == b.From.Name && a.To.Name == b.To.Name
}

func sameTieEnds(a, b config.TieLine) bool {
	return a.From.Router == b.From.Router && a.From.N == b.From.N && a.To.Router == b.To.Router && a.To.N == b.To.N
}

// matchTieLine: a file from this install by hidden id, then by name, then by the two ends. Two tie lines with the same
// two ends are the same cable, and config.Validate refuses a second one.
func matchTieLine(same bool, list []config.TieLine, t config.TieLine) int {
	if i := matchIdx(same, list, t.ID, t.Name, func(x config.TieLine) string { return x.ID }, func(x config.TieLine) string { return x.Name }); i >= 0 {
		return i
	}
	return slices.IndexFunc(list, func(x config.TieLine) bool { return sameTieEnds(x, t) })
}

// planTieLines maps each end through tieRouterHere first. A tie line with an end that maps to no router, or that
// config.TieLineProblem refuses after the mapping, gets Refused and the others import. So does one that would share its
// two ends or its name with another tie line here. Thus the import never leaves a tie line that Validate refuses.
func (p *importPlanner) planTieLines(cand *config.Config, cat *importCat) error {
	for _, t := range p.b.TieLines {
		if !p.picked(catTieLines, t.ID) {
			continue
		}
		fileID := t.ID
		it := importItem{Key: t.ID, Name: t.Name}
		var unmapped []string
		for _, end := range []*config.Ref{&t.From, &t.To} {
			to, ok := p.tieRouterHere(end.Router)
			if !ok {
				unmapped = append(unmapped, p.tieRouterTitle(end.Router))
			}
			end.Router = to
		}
		if len(unmapped) > 0 {
			it.Refused = tieUnmapped(strings.Join(slices.Compact(unmapped), ", "))
			cat.Items = append(cat.Items, it)
			continue
		}
		if why := cand.TieLineProblem(t); why != "" {
			it.Refused = why
			cat.Items = append(cat.Items, it)
			continue
		}
		out, twin := addItem, ""
		if i := matchTieLine(p.same, cand.TieLines, t); i >= 0 {
			lt := cand.TieLines[i]
			t.ID, t.Extra, twin = lt.ID, lt.Extra, lt.ID
			// A renamed copy of a tie line with the same two ends is still the same cable: no Rename then.
			var taken func(string) bool
			if it.NoRename = sameTieEnds(lt, t); !it.NoRename {
				taken = namesIn(cand.TieLines, func(x config.TieLine) string { return x.Name })
			}
			out = p.settle(&it, catTieLines, lt.Name, sameTieLine(t, lt), taken)
		}
		if out == addItem {
			t.Name = cmp.Or(it.As, t.Name)
			if t.ID == "" || it.Choice == useRename || slices.ContainsFunc(cand.TieLines, func(x config.TieLine) bool { return x.ID == t.ID }) {
				t.ID = config.RandomHexID(8)
			}
			t.Extra = nil
		}
		if out != keepLive && slices.ContainsFunc(cand.TieLines, func(x config.TieLine) bool {
			return (out != overwrite || x.ID != twin) && (sameTieEnds(x, t) || sameName(x.Name, t.Name))
		}) {
			it.Refused = "another tie line here has the same two ends or the same name"
			cat.Items = append(cat.Items, it)
			continue
		}
		switch out {
		case keepLive:
			p.ties[fileID] = it.liveID(twin)
		case overwrite:
			cand.TieLines[slices.IndexFunc(cand.TieLines, func(x config.TieLine) bool { return x.ID == twin })] = t
			p.ties[fileID] = twin
		case addItem:
			cand.TieLines = append(cand.TieLines, t)
			p.ties[fileID] = t.ID
		}
		cat.Items = append(cat.Items, it)
	}
	return nil
}

package app

import "github.com/sethdotfm/sode/internal/config"

// presetItem: a blank router in a reference is the one router (config Normalize fills it).
func presetItem(src int, levels []int, dsts ...int) config.PresetItem {
	it := config.PresetItem{Src: config.Ref{N: src}, Dsts: []config.Ref{}, Levels: levels}
	for _, d := range dsts {
		it.Dsts = append(it.Dsts, config.Ref{N: d})
	}
	return it
}

func withItemExtra(it config.PresetItem, extra map[string]any) config.PresetItem {
	it.Extra = extra
	return it
}

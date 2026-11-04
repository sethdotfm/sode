package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"net/http/httptest"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
)

// maxOwned is a config with every owned list as big as it is allowed to be
// (worst case: every character escaped to six bytes in JSON).
func maxOwned(t *testing.T) config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.Normalize()
	// grow adds to cfg while the result still validates.
	grow := func(try func(*config.Config) bool) {
		for i := 0; ; i++ {
			next := cfg.Clone()
			if !try(&next) {
				return
			}
			next.Normalize()
			if err := next.Validate(); err != nil {
				t.Logf("stop: %v", err)
				return
			}
			cfg = next
		}
	}
	// Filter sets: the longest ones until the cap refuses the next.
	f := 0
	grow(func(c *config.Config) bool {
		c.FilterSets = append(c.FilterSets, config.FilterSet{
			Name: fmt.Sprintf("%02d", f) + strings.Repeat("<", 30),
			Src:  strings.Repeat("<", 500), Dst: strings.Repeat("<", 500), Sode: true, Ext: true, Hidden: true,
		})
		f++
		return len(c.FilterSets) <= 30
	})
	n := 0
	grow(func(c *config.Config) bool {
		n++
		si := (n - 1) / config.MaxPresetItems
		if si >= config.MaxPresets {
			return false
		}
		if len(c.Presets) <= si {
			c.Presets = append(c.Presets, config.Preset{Name: fmt.Sprintf("%02d", si) + strings.Repeat("<", 30)})
		}
		lv := make([]int, 0, 64)
		for l := 1; l <= 64; l++ {
			lv = append(lv, l)
		}
		c.Presets[si].Items = append(c.Presets[si].Items, config.PresetItem{Src: 60000 + n%5000, Dsts: []int{n}, Levels: lv})
		return true
	})
	if len(cfg.FilterSets) == 0 || len(cfg.Presets) == 0 {
		t.Fatal("test config not filled")
	}
	return cfg
}

// The messages that carry the big lists stay under the 64 KiB read limit; the
// whole config with all of them does not, which is why a Settings save must
// leave the owned lists out (and why the server ignores them if it doesn't).
func TestMessageSizes(t *testing.T) {
	cfg := maxOwned(t)
	size := func(v any) int { b, _ := json.Marshal(v); return len(b) }
	const envelope = 1024
	for name, n := range map[string]int{
		"presets":    size(map[string]any{"t": "presets", "presets": cfg.Presets, "id": 1<<31 - 1}),
		"filterSets": size(map[string]any{"t": "filterSets", "sets": cfg.FilterSets, "id": 1<<31 - 1}),
	} {
		t.Logf("%s: %d bytes", name, n)
		if n >= wsReadLimit {
			t.Errorf("%s message is %d bytes, over the %d read limit", name, n, wsReadLimit)
		}
	}
	if n := size(cfg.Presets); n > config.MaxPresetsBytes || n < config.MaxPresetsBytes/2 {
		t.Errorf("max presets are %d bytes, cap %d", n, config.MaxPresetsBytes)
	}
	// hello carries the config (no limit on what the server sends). It and
	// what a Settings save would echo back are over the limit with the owned
	// lists in; without them (what the form owns) they are tiny.
	full := size(map[string]any{"t": "config", "config": cfg})
	t.Logf("config message with every owned list full: %d bytes", full)
	if full < wsReadLimit {
		t.Errorf("a full config is %d bytes: the owned lists were meant to be able to exceed the %d limit", full, wsReadLimit)
	}
	form := cfg
	form.FilterSets, form.Presets, form.Users = nil, nil, nil
	if n := size(map[string]any{"t": "config", "config": form, "pin": strings.Repeat("9", 64)}); n+envelope >= wsReadLimit/4 {
		t.Errorf("a Settings message without owned lists is %d bytes", n)
	}
	var hello struct{ Cfg config.Config }
	b, _ := json.Marshal(struct {
		Cfg config.Config `json:"cfg"`
	}{cfg})
	if err := json.Unmarshal(b, &hello); err != nil {
		t.Fatal(err)
	}
	t.Logf("hello cfg: %d bytes", len(b))
}

// A settings save that carries every owned field (as a browser sending S.cfg
// whole would) changes none of them, nor fails because of them.
func TestSettingsIgnoresOwned(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	if err := a.SaveFilterSets(c, []config.FilterSet{{Name: "keep"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.SavePresets(c, []config.Preset{{Name: "keep", Items: []config.PresetItem{{Src: 1, Dsts: []int{2}}}}}); err != nil {
		t.Fatal(err)
	}
	live := a.r().cfg

	in := maxOwned(t)
	in.Router = "9.9.9.9:9"
	in.SettingsPin = "not a hash"
	in.Users = []config.User{{Name: "intruder"}}
	in.Presets[0].Name = "" // would fail validation if it were applied
	if err := a.SaveSettings(c, in, "", false); err != nil {
		t.Fatalf("owned fields were validated or applied: %v", err)
	}
	got := a.r().cfg
	if got.Router != "9.9.9.9:9" {
		t.Error("form field not applied")
	}
	if len(got.FilterSets) != 1 || got.FilterSets[0].Name != "keep" || len(got.Presets) != 1 ||
		got.SettingsPin != live.SettingsPin || len(got.Users) != 0 ||
		got.Locks.Any() || got.AutoLockMinutes != live.AutoLockMinutes {
		t.Fatalf("owned fields changed: %+v", got)
	}
}

// The same over the socket: a config message just under the read limit with
// big owned lists is answered ok and changes none of them.
func TestSocketConfig(t *testing.T) {
	a := pinApp(t)
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	send := func(v any) {
		b, _ := json.Marshal(v)
		if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
	}
	await := func(typ string) map[string]any {
		for {
			_, b, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("waiting for %s: %v", typ, err)
			}
			var m map[string]any
			json.Unmarshal(b, &m)
			if m["t"] == typ {
				return m
			}
		}
	}
	await("hello")

	// A config message with big owned lists, under the read limit.
	big := maxOwned(t)
	cfg := a.r().cfg
	cfg.Presets = big.Presets[:1]
	cfg.FilterSets = big.FilterSets[:2]
	b, _ := json.Marshal(map[string]any{"t": "config", "config": cfg})
	if len(b) >= wsReadLimit {
		t.Fatalf("test message is %d bytes", len(b))
	}
	send(map[string]any{"t": "config", "config": cfg})
	if m := await("configResult"); m["ok"] != true {
		t.Fatalf("%v", m)
	}
	got := a.r().cfg
	if len(got.Presets) != 0 || len(got.FilterSets) != 0 {
		t.Fatalf("owned fields applied from a config message: %+v", got)
	}
}

package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The Import tab: a same-install file preselects Use new on its clash, and Review… puts the review in the
// dialog body with the PIN row. Use current removes the PIN row and imports without a recheck; Use new
// imports after the PIN typed in the review.
func TestChromeImportReview(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if src, _ := a.liveNames(); len(src) > simSrc {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the router names were not read")
		}
	}
	src, dst := a.liveNames()
	lifeCommit(t, a, func(cfg *config.Config) {
		item := []config.PresetItem{{Src: cfg.SourceRef(simSrc, src[simSrc]), Dsts: []config.Ref{cfg.DestinationRef(simDst, dst[simDst])}}}
		cfg.Presets = []config.Preset{{Name: "Show", Items: item}, {Name: "Open", Items: item}}
	})
	zip, err := a.buildExport(exportReq{Categories: []string{catPresets}})
	if err != nil {
		t.Fatal(err)
	}
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets = cfg.Presets[:1]
		cfg.Presets[0].Name = "Show live"
	})
	names := func() string {
		out := ""
		for _, p := range a.r().cfg.Presets {
			out += p.Name + ";"
		}
		return out
	}

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Export & Import button shown", `!!document.getElementById('xi-open')?.offsetParent`)
	b.eval(`document.getElementById('xi-open').click(); document.getElementById('tab-xi-import-tab').click(); true`)
	load := fmt.Sprintf(`(() => {
		const bin = atob('%s');
		const u8 = Uint8Array.from(bin, (c) => c.charCodeAt(0));
		const dt = new DataTransfer();
		dt.items.add(new File([u8], 'show.zip', { type: 'application/zip' }));
		const inp = document.getElementById('xi-file');
		inp.files = dt.files;
		inp.dispatchEvent(new Event('change'));
		return true;
	})()`, base64.StdEncoding.EncodeToString(zip))
	showChoice := `[...document.querySelectorAll('.sode__xi-import .sode__xi-head[data-cat="presets"] + .sode__xi-items .sode__xi-choice label')].filter((b) => b.closest('.sode__xi-item').textContent.includes('Show'))`
	open := func() {
		b.eval(load)
		b.waitFor("presets listed", `!!document.querySelector('.sode__xi-import .sode__xi-head[data-cat="presets"]')`)
		b.waitFor("Show asks Use new", showChoice+`.find((b) => document.getElementById(b.htmlFor).checked)?.textContent === 'Use new'`)
	}
	open()
	b.shot("list")
	if got := b.eval(showChoice + `.map((b) => b.textContent).join(',')`); got != "Use new,Use current,Skip,Rename" {
		t.Errorf("clash choices %v", got)
	}

	b.eval(`document.getElementById('xi-review').click(); true`)
	b.waitFor("review with the PIN row", `document.getElementById('xi-title').textContent === 'Import from file?' && !!document.getElementById('xi-pin')?.offsetParent && !document.querySelector('.sode__xi [role=tab]')?.offsetParent`)
	b.shot("review")
	if got := b.eval(`[...document.querySelectorAll('.sode__xi-crit button')].map((x) => x.textContent).join()`); got != "Back up everything first" {
		t.Errorf("Critical row %v", got)
	}
	b.eval(`document.getElementById('xi-back').click(); true`)
	b.waitFor("back to the list", `!!document.querySelector('.sode__xi-import .sode__xi-head[data-cat="presets"]') && !!document.querySelector('.sode__xi [role=tab]')?.offsetParent`)

	b.eval(showChoice + `.find((b) => b.textContent === 'Use current').click(); true`)
	b.eval(`document.getElementById('xi-review').click(); true`)
	b.waitFor("review without the PIN row", `document.getElementById('xi-title').textContent === 'Import from file?' && !document.getElementById('xi-pin')?.offsetParent`)
	b.eval(`document.getElementById('xi-import').click(); true`)
	b.waitFor("imported", `/^Imported/.test(document.querySelector('.sode__xi .pb-alert')?.textContent || '')`)
	if got := names(); got != "Show live;Open;" {
		t.Fatalf("after Use current: %s", got)
	}

	b.eval(`[...document.querySelectorAll('.sode__xi .pb-modal__footer button')].find((x) => x.textContent === 'Import another').click(); true`)
	open()
	b.eval(`document.getElementById('xi-review').click(); true`)
	b.waitFor("PIN row", `!!document.getElementById('xi-pin')?.offsetParent`)
	b.eval(`document.getElementById('xi-pin').value = '1234'; document.getElementById('xi-import').click(); true`)
	b.waitFor("imported after the recheck", `/^Imported/.test(document.querySelector('.sode__xi .pb-alert')?.textContent || '')`)
	if got := names(); got != "Show;Open;" {
		t.Fatalf("after Use new: %s", got)
	}
	b.check()
}

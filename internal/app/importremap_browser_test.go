package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The remap step on a file from another router. A name at another number moves automatically. A name the target
// lacks preselects Skip, and Other… picks a number. The review counts the router pair.
func TestChromeImportRemap(t *testing.T) {
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
		if src, _ := a.liveNames(); len(src) > simSrc+3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the router names were not read")
		}
	}
	src, dst := a.liveNames()
	// A name that is on the target one time only, filed at simSrc.
	moved := -1
	for n := simSrc + 1; n < len(src); n++ {
		count := 0
		for _, s := range src {
			if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(src[n])) {
				count++
			}
		}
		if count == 1 && strings.TrimSpace(src[n]) != "" && n != simSrc+2 && n != simSrc+3 {
			moved = n
			break
		}
	}
	if moved < 0 {
		t.Fatal("no unique source name on the simulator")
	}
	lifeCommit(t, a, func(cfg *config.Config) {
		d := []config.Ref{cfg.DestinationRef(simDst, dst[simDst])}
		cfg.Presets = []config.Preset{
			{Name: "Show", Items: []config.PresetItem{{Src: cfg.SourceRef(simSrc, src[moved]), Dsts: d}}},
			{Name: "Open", Items: []config.PresetItem{{Src: cfg.SourceRef(simSrc+2, "Nowhere 9"), Dsts: d}}},
		}
	})
	zip, err := a.buildExport(exportReq{Categories: []string{catPresets}})
	if err != nil {
		t.Fatal(err)
	}
	file := otherRouter(t, a, zip)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = nil })
	off := a.r().cfg.NumberOffset

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Export & Import button shown", `!!document.getElementById('xi-open')?.offsetParent`)
	b.eval(`document.getElementById('xi-open').click(); document.getElementById('tab-xi-import-tab').click(); true`)
	b.eval(fmt.Sprintf(`(() => {
		const u8 = Uint8Array.from(atob('%s'), (c) => c.charCodeAt(0));
		const dt = new DataTransfer();
		dt.items.add(new File([u8], 'truck.zip', { type: 'application/zip' }));
		const inp = document.getElementById('xi-file');
		inp.files = dt.files;
		inp.dispatchEvent(new Event('change'));
		return true;
	})()`, base64.StdEncoding.EncodeToString(file)))
	row := fmt.Sprintf(`document.querySelector('[data-row="%s:source:%d"]')`, otherRouterUID, simSrc+2)
	b.waitFor("the unmatched name preselects Skip", row+`?.querySelector('select').value === 'skip'`)
	if got := b.eval(`document.querySelector('.sode__xi-moved')?.textContent || ''`); !strings.Contains(fmt.Sprint(got), "Moved automatically (1)") {
		t.Errorf("moved group: %v", got)
	}
	b.eval(`document.querySelector(".sode__xi-names").scrollIntoView(); true`)
	b.shot("rows")
	b.eval(row + `.querySelector('select').value = 'other'; ` + row + `.querySelector('select').dispatchEvent(new Event('change')); true`)
	b.eval(fmt.Sprintf(`(() => { const i = %s.querySelector('input'); i.value = '%d'; i.dispatchEvent(new Event('change')); return true; })()`, row, simSrc+3+off))
	b.waitFor("the choice holds", row+fmt.Sprintf(`?.querySelector('select').value === 'n:%d'`, simSrc+3))

	b.eval(`document.getElementById('xi-review').click(); true`)
	b.waitFor("review counts the pair", `/1 moved automatically, 1 chosen by hand, 0 skipped/.test(document.querySelector('.sode__xi-pairs')?.textContent || '')`)
	b.shot("review")
	b.eval(`document.getElementById('xi-import').click(); true`)
	b.waitFor("imported", `/^Imported/.test(document.querySelector('.sode__xi .pb-alert')?.textContent || '')`)
	show, _ := presetNamed(t, a, "Show")
	open, _ := presetNamed(t, a, "Open")
	if len(show.Items) == 0 || show.Items[0].Src.N != moved || len(open.Items) == 0 || open.Items[0].Src.N != simSrc+3 || len(show.Broken)+len(open.Broken) != 0 {
		t.Fatalf("Show %+v, Open %+v", show, open)
	}
	b.check()
}

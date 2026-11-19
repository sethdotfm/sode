package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// mouse sends a real mouse event at the middle of the element expr names.
func mouse(b *tab, kind, expr string, modifiers int) {
	got := b.eval(`(() => { const r = ` + expr + `.getBoundingClientRect(); return [r.x + 8, r.y + r.height / 2]; })()`)
	at, _ := got.([]any)
	if len(at) != 2 {
		return
	}
	buttons := 1
	if kind == "mouseReleased" {
		buttons = 0
	}
	b.call("Input.dispatchMouseEvent", map[string]any{"type": kind, "x": at[0], "y": at[1], "button": "left", "buttons": buttons, "clickCount": 1, "modifiers": modifiers})
}

func press(b *tab, key, code string, vk, modifiers int) {
	for _, kind := range []string{"keyDown", "keyUp"} {
		b.call("Input.dispatchKeyEvent", map[string]any{"type": kind, "key": key, "code": code, "windowsVirtualKeyCode": vk, "modifiers": modifiers})
	}
}

// The remap rows as one list. Shift+arrow and a drag select rows, and Esc clears them without closing the dialog. A
// choice on a selected row goes to every selected row, and Reset to Skip clears them. A range term whose names differ asks
// with Convert to names preselected, and the import writes the names.
func TestChromeImportRemapSelect(t *testing.T) {
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
		if src, _ := a.liveNames(); len(src) > simSrc+8 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the router names were not read")
		}
	}
	_, dst := a.liveNames()
	uid := a.r().cfg.UID
	off := a.r().cfg.NumberOffset
	lifeCommit(t, a, func(cfg *config.Config) {
		d := []config.Ref{cfg.DestinationRef(simDst, dst[simDst])}
		for i, n := range []int{2, 4, 5} {
			cfg.Presets = append(cfg.Presets, config.Preset{Name: fmt.Sprintf("P%d", i), Items: []config.PresetItem{{Src: cfg.SourceRef(simSrc+n, fmt.Sprintf("Nowhere %d", i)), Dsts: d}}})
		}
		cfg.FilterSets = []config.FilterSet{{Name: "Cams", Router: uid, Src: fmt.Sprintf("%d-%d", simSrc+6+off, simSrc+7+off), Sode: true}}
	})
	zip, err := a.buildExport(exportReq{Categories: []string{catPresets, catFilterSets}})
	if err != nil {
		t.Fatal(err)
	}
	// The file's names at the range differ from the router's.
	files := map[string]string{}
	for name, body := range unzipAll(t, otherRouter(t, a, zip)) {
		s := string(body)
		if strings.HasPrefix(name, "names/") {
			for i, n := range []int{simSrc + 6, simSrc + 7} {
				s = regexp.MustCompile(fmt.Sprintf(`(?m)(- "n": %d\n\s+name: ).*$`, n)).ReplaceAllString(s, fmt.Sprintf("${1}FILE CAM %d", i))
			}
		}
		files[name] = s
	}
	file := zipOf(t, files)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets, cfg.FilterSets = nil, nil })

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
	row := func(n int) string {
		return fmt.Sprintf(`document.querySelector('[data-row="%s:source:%d"]')`, otherRouterUID, simSrc+n)
	}
	label := func(n int) string { return row(n) + `.querySelector('strong')` }
	selected := `document.querySelectorAll('.sode__xi-remap [aria-selected="true"]').length`
	b.waitFor("three unmatched rows", row(5)+`?.querySelector('select').value === 'skip'`)

	b.eval(row(2) + `.scrollIntoView({ block: 'center' }); true`)
	mouse(b, "mousePressed", label(2), 0)
	mouse(b, "mouseReleased", label(2), 0)
	press(b, "ArrowDown", "ArrowDown", 40, 8)
	b.waitFor("Shift+Down selects two rows", selected+` === 2`)
	press(b, "Escape", "Escape", 27, 0)
	b.waitFor("Esc clears the selection", selected+` === 0`)
	if open := b.eval(`!!document.getElementById('xi-review')?.offsetParent`); open != true {
		t.Fatal("Esc with rows selected closed the dialog")
	}

	mouse(b, "mousePressed", label(2), 0)
	mouse(b, "mouseMoved", label(4), 0)
	mouse(b, "mouseMoved", label(5), 0)
	mouse(b, "mouseReleased", label(5), 0)
	b.waitFor("a drag selects three rows", selected+` === 3`)
	b.shot("selected")

	sel := row(2) + `.querySelector('select')`
	b.eval(sel + `.value = 'other'; ` + sel + `.dispatchEvent(new Event('change')); true`)
	b.eval(fmt.Sprintf(`(() => { const i = %s.querySelector('input'); i.value = '%d'; i.dispatchEvent(new Event('change')); return true; })()`, row(2), simSrc+3+off))
	for _, n := range []int{2, 4, 5} {
		b.waitFor(fmt.Sprintf("row %d takes the choice", n), row(n)+fmt.Sprintf(`?.querySelector('select').value === 'n:%d'`, simSrc+3))
	}
	b.eval(`document.getElementById('xi-bulk-skip').click(); true`)
	for _, n := range []int{2, 4, 5} {
		b.waitFor(fmt.Sprintf("Reset to Skip clears row %d", n), row(n)+`?.querySelector('select').value === 'skip'`)
	}

	term := fmt.Sprintf(`document.querySelector('[data-term="%s:src:%d-%d"] select')`, otherRouterUID, simSrc+6+off, simSrc+7+off)
	b.waitFor("the range term preselects Convert to names", term+`?.value === 'names'`)
	b.eval(`document.getElementById('xi-review').click(); true`)
	b.waitFor("the review names the changed term", `/Convert to names/.test(document.querySelector('.sode__xi-pairs')?.textContent || '')`)
	b.shot("review")
	b.eval(`document.getElementById('xi-import').click(); true`)
	b.waitFor("imported", `/^Imported/.test(document.querySelector('.sode__xi .pb-alert')?.textContent || '')`)
	fs := a.r().cfg.FilterSets
	if len(fs) != 1 || fs[0].Src != `"FILE CAM 0", "FILE CAM 1"` || fs[0].Router != uid {
		t.Fatalf("filter sets: %+v", fs)
	}
	b.check()
}

package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func TestChromeExportDialog(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Export & Import button shown", `!!document.getElementById('xi-open')?.offsetParent`)
	b.eval(`document.getElementById('xi-open').click(); true`)
	b.waitFor("nine categories listed", `document.querySelectorAll('.sode__xi-head').length === 9`)

	var got struct {
		Width, Rem     float64
		Tabs, Footer   []string
		Pins           bool
		Cats, Sizes    []string
		Old            bool
		UsersCount     string
		ImportHidden   bool
		ExportDisabled bool
	}
	read := func() {
		raw := b.eval(`JSON.stringify((() => {
			const d = document.querySelector('.sode__xi');
			return {
				width: d.getBoundingClientRect().width,
				rem: parseFloat(getComputedStyle(document.documentElement).fontSize),
				tabs: [...d.querySelectorAll('[role=tab]')].map((t) => t.textContent),
				footer: [...d.querySelectorAll('.pb-modal__footer button')].filter((x) => x.offsetParent).map((x) => x.textContent),
				pins: document.getElementById('xi-pins').checked,
				cats: [...d.querySelectorAll('.sode__xi-head')].map((h) => h.dataset.cat),
				sizes: [...d.querySelectorAll('.sode__xi-head .sode__xi-size')].map((s) => s.textContent),
				old: !!document.getElementById('config-file') || !!document.getElementById('transfer-open'),
				importHidden: document.getElementById('panel-xi-import-tab').hidden,
				exportDisabled: document.getElementById('xi-export').disabled,
			};
		})())`).(string)
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if got.Width != 37.5*got.Rem {
		t.Errorf("dialog %v px wide, want 37.5rem (%v px)", got.Width, 37.5*got.Rem)
	}
	if strings.Join(got.Tabs, ",") != "Export,Import" {
		t.Errorf("tabs %v", got.Tabs)
	}
	if !got.Pins {
		t.Error("Include PINs is off by default")
	}
	if strings.Join(got.Cats, ",") != strings.Join(exportCategoryNames(), ",") {
		t.Errorf("categories %v", got.Cats)
	}
	if got.Sizes[2] == "0 B" || got.Sizes[0] == "0 B" {
		t.Errorf("Settings and Users have no size: %v", got.Sizes)
	}
	if got.Old {
		t.Error("the old Server backup or profile transfer is still in the section")
	}
	b.waitFor("zip size on Export", `/^Export \d/.test(document.getElementById('xi-export').textContent)`)
	read()
	if len(got.Footer) != 3 || got.Footer[0] != "Close" || got.Footer[1] != "Back up everything" || !strings.HasPrefix(got.Footer[2], "Export ") {
		t.Errorf("footer %v", got.Footer)
	}

	b.eval(`(() => {
		window.sentBodies = [];
		// No real download: the runner's Downloads folder stays empty.
		const click = HTMLAnchorElement.prototype.click;
		HTMLAnchorElement.prototype.click = function () { if (this.download) window.downloaded = this.download; else click.call(this); };
		const f = window.fetch;
		window.fetch = (u, i) => { if (String(u).endsWith('api/export')) window.sentBodies.push(i.body); return f(u, i); };
		// Each click draws the list again, so find the box anew.
		for (const cat of ['settings', 'routers', 'browser']) {
			const box = document.querySelector('.sode__xi-head[data-cat="' + cat + '"] input');
			if (box.checked && !box.disabled) box.click();
		}
		return true;
	})()`)
	b.eval(`document.getElementById('xi-export').click(); true`)
	b.waitFor("export sent", `window.sentBodies.length === 1`)
	b.waitFor("saved note", `/^Saved .*\.zip/.test(document.getElementById('xi-note').textContent) && /-Sode-Export\.zip$/.test(window.downloaded)`)
	var body exportReq
	json.Unmarshal([]byte(b.eval(`window.sentBodies[0]`).(string)), &body)
	ana, _ := a.r().cfg.FindUserByName("Ana")
	if strings.Join(body.Categories, ",") != "users" || len(body.Pick[catUsers]) != 1 || body.Pick[catUsers][0] != ana.ID || !body.IncludePins {
		t.Errorf("export body %+v", body)
	}

	b.eval(`document.getElementById('tab-xi-import-tab').click(); true`)
	read()
	if got.ImportHidden || strings.Join(got.Footer, ",") != "Close,Choose a file…" {
		t.Errorf("Import tab: hidden %v, footer %v", got.ImportHidden, got.Footer)
	}
	b.eval(`document.getElementById('xi-close').click(); true`)
	b.waitFor("dialog closed", `!document.querySelector('.sode__xi')?.offsetParent`)

	b.eval(`document.documentElement.dataset.layout = 'phone'; document.getElementById('xi-open').click(); true`)
	b.waitFor("desktop-only message", `!!document.querySelector('.sode__xi .sode__desktop-only')?.offsetParent && !document.querySelector('.sode__xi [role=tab]')?.offsetParent`)
	b.check()
}

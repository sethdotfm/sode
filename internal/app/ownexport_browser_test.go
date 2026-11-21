package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// My user > Export & Import for a Standard user: the own categories and the greyed Administrators only rows. The
// export sends only own categories, and the file imports again through the own Import tab.
func TestChromeOwnExportImport(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		withAdministrator(c)
		c.Users[0].Profile = config.Profile{
			Prefs:   map[string]string{"theme": "dark"},
			Presets: []config.Preset{{Name: "Mine", Items: []config.PresetItem{{Src: config.Ref{N: 1}, Dsts: []config.Ref{{N: 2}}}}}},
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Export & Import shown", `!!document.getElementById('account-transfer')?.offsetParent`)
	b.eval(`document.getElementById('account-transfer').click(); true`)
	b.waitFor("own rows listed", `document.querySelectorAll('.sode__xi-own .sode__xi-head').length === 13`)

	var got struct {
		Cats          []string
		AdminDisabled bool
		AdminHints    []string
	}
	raw := b.eval(`JSON.stringify((() => {
		const heads = [...document.querySelectorAll('.sode__xi-own .sode__xi-head')];
		const admin = heads.filter((h) => h.dataset.cat.startsWith('admin:'));
		return {
			cats: heads.map((h) => h.dataset.cat),
			adminDisabled: admin.every((h) => h.querySelector('input').disabled && !h.querySelector('input').checked),
			adminHints: [...new Set(admin.map((h) => h.querySelector('.sode__hint').textContent))],
		};
	})())`).(string)
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Cats[:3], ",") != "profile,presets,filterSets" || !slices.Contains(got.Cats, "admin:settings") {
		t.Errorf("categories %v", got.Cats)
	}
	if !got.AdminDisabled || strings.Join(got.AdminHints, ",") != "Administrators only" {
		t.Errorf("global rows: disabled %v, hints %v", got.AdminDisabled, got.AdminHints)
	}

	b.eval(`(() => {
		window.sentBodies = [];
		const click = HTMLAnchorElement.prototype.click;
		HTMLAnchorElement.prototype.click = function () { if (this.download) window.downloaded = this.download; else click.call(this); };
		const f = window.fetch;
		window.fetch = async (u, i) => {
			const res = await f(u, i);
			if (String(u).endsWith('api/own/export')) {
				window.sentBodies.push(i.body);
				window.zip = await res.clone().blob();
			}
			return res;
		};
		return true;
	})()`)
	b.eval(`document.getElementById('own-xi-export').click(); true`)
	b.waitFor("saved note", `/^Saved .*\.zip/.test(document.getElementById('own-xi-note').textContent) && !!window.zip`)
	var body exportReq
	json.Unmarshal([]byte(b.eval(`window.sentBodies[0]`).(string)), &body)
	if strings.Join(body.Categories, ",") != "profile,presets" || len(body.Pick[catPresets]) != 1 {
		t.Errorf("export body %+v", body)
	}

	lifeCommit(t, a, func(c *config.Config) { c.Users[0].Profile.Presets = nil })
	b.eval(`document.getElementById('tab-own-xi-import-tab').click(); true`)
	b.eval(`(() => {
		const dt = new DataTransfer();
		dt.items.add(new File([window.zip], 'mine.zip', { type: 'application/zip' }));
		const input = document.getElementById('own-xi-file');
		input.files = dt.files;
		input.dispatchEvent(new Event('change'));
		return true;
	})()`)
	b.waitFor("own import list", `!!document.getElementById('own-xi-review')?.offsetParent`)
	b.eval(`document.getElementById('own-xi-review').click(); true`)
	b.waitFor("review", `!!document.getElementById('own-xi-import')?.offsetParent`)
	b.eval(`document.getElementById('own-xi-import').click(); true`)
	b.waitFor("imported", `/Imported/.test(document.querySelector('[aria-labelledby="own-xi-title"] .pb-alert')?.textContent || '')`)
	if ps := a.r().cfg.Users[0].Profile.Presets; len(ps) != 1 || ps[0].Name != "Mine" {
		t.Errorf("presets after the own import: %+v", ps)
	}
	b.check()
}

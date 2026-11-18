package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// A preset Item link opens the preset popup on that preset on its own install and leaves the address. A link
// from another install, for a preset or a panel, opens nothing and says why.
func TestChromeItemLinkWorksOnlyOnItsInstall(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim, func(c *config.Config) {
		c.Presets = []config.Preset{
			{ID: "pfirst", Name: "First", Items: []config.PresetItem{{Src: config.Ref{N: simSrc}, Dsts: []config.Ref{{N: simDst}}}}},
			{ID: "pshow", Name: "Show", Items: []config.PresetItem{{Src: config.Ref{N: simSrc}, Dsts: []config.Ref{{N: simDst + 1}}}}},
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	here := a.r().cfg.InstallUID

	b.open(srv.URL + "/#preset=pshow&install=" + here)
	b.waitFor("the popup opens on the linked preset", `!document.querySelector('.sode__preset-pop').closest('.pb-modal-backdrop').hidden && document.querySelector('.sode__preset-pop .sode__preset-item.is-hi strong')?.textContent === 'Show'`)
	b.waitFor("the link leaves the address", `location.hash === ''`)
	b.check()

	// The link the preset editor makes, opened as is, does the same.
	b.eval(`[...document.querySelectorAll('.sode__preset-pop button')].find((x) => x.textContent === 'Preset Editor').click(); true`)
	pick := `[...document.querySelectorAll('select')].find((s) => s.offsetParent && [...s.options].some((o) => o.value === 'global:Show'))`
	b.waitFor("the preset editor", "!!"+pick)
	b.eval(`(() => { const s = ` + pick + `; s.value = 'global:Show'; s.dispatchEvent(new Event('change')); return true })()`)
	b.eval(`[...document.querySelectorAll('button')].find((x) => x.offsetParent && x.textContent === 'Item link').click(); true`)
	b.waitFor("the Item link and its QR code", `document.querySelector('.sode__itemlink input')?.value === location.origin + '/#preset=pshow&install=`+here+`' && !!document.querySelector('.sode__itemlink .sode__qr')`)
	link, _ := b.eval(`document.querySelector('.sode__itemlink input').value`).(string)
	b.open(strings.Replace(link, "/#", "/?again#", 1)) // a new search, so a full load and not a hashchange
	b.waitFor("the made link opens the popup on its preset", `!document.querySelector('.sode__preset-pop').closest('.pb-modal-backdrop').hidden && document.querySelector('.sode__preset-pop .sode__preset-item.is-hi strong')?.textContent === 'Show'`)
	b.check()

	b.open(srv.URL + "/?other#preset=pshow&install=0000000000000000")
	b.waitFor("another install's link says so", `document.body.innerText.includes('made on another Sode install')`)
	b.waitFor("and opens no popup", `document.querySelector('.sode__preset-pop').closest('.pb-modal-backdrop').hidden`)
	b.check()

	b.open(srv.URL + "/panels/p-00000000#install=0000000000000000")
	b.waitFor("a panel link from another install says so", `document.getElementById('panels-note')?.textContent.includes('made on another Sode install')`)
	b.check()
}

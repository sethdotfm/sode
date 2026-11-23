package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Custom page border color: the swatch is a color input under its label, with a paintbrush. Choosing Custom does not
// call showPicker, which opened at the select and not at the swatch.
func TestChromeBrandBorderSwatchPicker(t *testing.T) {
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
	b.waitFor("Branding shown", `!!document.getElementById('cfg-brandborder')?.offsetParent`)
	b.eval(`(() => {
		window.pickerCalls = 0;
		HTMLInputElement.prototype.showPicker = function () { window.pickerCalls++; };
		const s = document.getElementById('cfg-brandborder');
		s.value = 'custom';
		s.dispatchEvent(new Event('change', { bubbles: true }));
		return true;
	})()`)
	b.waitFor("the swatch shows with its brush", `(() => {
		const w = document.getElementById('cfg-brandcustom-wrap');
		const i = document.getElementById('cfg-brandcustom');
		if (!w?.offsetParent || !w.querySelector('svg.pb-icon')) return false;
		w.scrollIntoView({ block: 'center' });
		const r = i.getBoundingClientRect(), q = w.getBoundingClientRect();
		const c = document.elementFromPoint(q.left + q.width / 2, q.top + q.height / 2);
		return r.width >= q.width - 2 && r.height >= q.height - 2 && c === i && w.tagName === 'LABEL' && i.type === 'color';
	})()`)
	if b.eval(`window.pickerCalls`) != float64(0) {
		t.Error("choosing Custom called showPicker")
	}
	b.eval(`(() => { const s = document.getElementById('cfg-brandborder'); s.value = ''; s.dispatchEvent(new Event('change', { bubbles: true })); return true; })()`)
	b.waitFor("the swatch hides for Off", `!document.getElementById('cfg-brandcustom-wrap').offsetParent`)
	b.check()
}

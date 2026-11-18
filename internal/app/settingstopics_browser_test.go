package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Settings in five topics. Each band has its accent line on a raised surface. The router block is under Routers,
// with the title of the router. The sidebar tree opens the topic in view and marks the section. Search names the topic.
func TestChromeSettingsTopics(t *testing.T) {
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
	// Access and Server show only to an Administrator.
	signInSettings(b)
	b.waitFor("Users shown", `!!document.getElementById('users-manage')?.offsetParent`)

	b.waitFor("five banded topics", `(() => {
		const bands = [...document.querySelectorAll('#settings-view > .sode__topic')];
		const names = bands.map((h) => h.textContent.trim()).join('|');
		return names === 'Preferences|Look and feel|Routers|Access|Server' && bands.every((h) => {
			const cs = getComputedStyle(h);
			return h.offsetParent && parseFloat(cs.borderTopWidth) >= 4 && cs.backgroundColor !== 'rgba(0, 0, 0, 0)';
		});
	})()`)
	b.waitFor("router block under Routers, titled by the router", `(() => {
		const r = document.querySelector('#settings-view > .sode__router');
		const routers = document.getElementById('topic-routers'), access = document.getElementById('topic-access');
		const thin = parseFloat(getComputedStyle(r).borderTopWidth) < parseFloat(getComputedStyle(routers).borderTopWidth);
		return r.offsetParent && thin && r.textContent.trim() === (document.getElementById('cfg-routername').value.trim() || 'Router')
			&& (routers.compareDocumentPosition(r) & Node.DOCUMENT_POSITION_FOLLOWING)
			&& (r.compareDocumentPosition(access) & Node.DOCUMENT_POSITION_FOLLOWING)
			&& document.querySelector('#routers-list li')?.textContent.includes(r.textContent.trim());
	})()`)

	// The sidebar follows the scroll: Access opens, Users is marked, other topics' entries close.
	tree := `(() => {
		const toc = document.getElementById('settings-toc');
		const open = [...toc.querySelectorAll('.sode__toc-topic[aria-expanded="true"]')].map((b) => b.textContent);
		const cur = toc.querySelector('[aria-current="location"]')?.textContent;
		const shown = [...toc.querySelectorAll('.sode__toc-sec')].filter((b) => !b.hidden).map((b) => b.textContent);
		return JSON.stringify({ open, cur, shown });
	})()`
	b.eval(`document.getElementById('users-group').scrollIntoView({ block: 'start', behavior: 'instant' }); true`)
	b.waitFor("Users marked under Access", tree+` === JSON.stringify({ open: ['Access'], cur: 'Users', shown: ['Locks', 'Users', 'API', 'Panels'] })`)
	b.eval(`document.querySelector('[data-section="takes"]').scrollIntoView({ block: 'start', behavior: 'instant' }); true`)
	b.waitFor("Takes marked under Routers", `(() => { const s = JSON.parse(`+tree+`); return s.open[0] === 'Routers' && s.cur === 'Takes' && s.shown.join() === 'Routers,Router & matrix,Router names,Takes'; })()`)

	// A topic entry scrolls to its band.
	b.eval(`document.querySelector('.sode__toc-topic[data-topic="server"]').click(); true`)
	b.waitFor("Server band at the top", `Math.abs(document.getElementById('topic-server').getBoundingClientRect().top - parseFloat(getComputedStyle(document.getElementById('topic-server')).scrollMarginTop)) < 4 || innerHeight + scrollY >= document.documentElement.scrollHeight - 2`)
	b.waitFor("Server open", `document.querySelector('.sode__toc-topic[data-topic="server"]').getAttribute('aria-expanded') === 'true'`)

	// Search names each result's topic.
	b.waitFor("search names the topic", `(() => {
		const f = document.getElementById('settings-find');
		f.value = 'confirm lockout'; f.dispatchEvent(new Event('input'));
		const li = document.querySelector('#settings-found li');
		const ok = li?.firstChild.textContent === 'Confirm lockout (ms)' && li.lastChild.textContent === 'in Look and feel › Confirm and results';
		f.value = ''; f.dispatchEvent(new Event('input'));
		return ok;
	})()`)
	b.check()
}

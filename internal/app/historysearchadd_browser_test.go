package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// In Filter by Source, + and Ctrl+Enter add the highlighted match, as in the Destination box. The
// list stays open on the same text, without the added ones. A row's + button does the same.
// Enter still picks one and closes the list.
func TestChromeHistorySearchAddsSeveral(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))

	b.open(srv.URL + "/")
	box := `document.querySelector('#history-search [data-search-src]')`
	menu := box + `.parentElement.querySelector('.sode__menu')`
	chips := `[...document.querySelectorAll('#history-search .sode__search-chip')].map((c) => c.textContent.replace('×', '').trim())`
	key := func(k, code string, ctrl bool) {
		c := "false"
		if ctrl {
			c = "true"
		}
		b.eval(box + `.dispatchEvent(new KeyboardEvent('keydown', { key: '` + k + `', code: '` + code + `', ctrlKey: ` + c + `, bubbles: true, cancelable: true })); true`)
	}

	b.waitFor("CAM matches", `(() => { const f = `+box+`; f.focus(); f.value = 'CAM'; f.dispatchEvent(new Event('input', { bubbles: true })); return `+menu+`.children.length > 2; })()`)
	first := b.eval(menu + `.children[0].textContent`)
	key("+", "Equal", false)
	b.waitFor("+ adds a chip", chips+`.length === 1`)
	key("Enter", "Enter", true)
	b.waitFor("Ctrl+Enter adds a second chip", chips+`.length === 2`)
	if got := b.eval(box + `.value`); got != "CAM" {
		t.Fatalf("the box holds %q after the adds, want CAM", got)
	}
	b.waitFor("the list stays open", `!`+menu+`.hidden`)
	if got := b.eval(`[...` + menu + `.children].map((li) => li.textContent)`); got != nil {
		for _, row := range got.([]any) {
			if row == first {
				t.Fatalf("the list still offers %v after it was added", first)
			}
		}
	}
	b.eval(menu + `.querySelector('.sode__menu-add').click(); true`)
	b.waitFor("a row's + adds a third chip", chips+`.length === 3`)

	key("Enter", "Enter", false)
	b.waitFor("Enter picks a fourth and closes the list", chips+`.length === 4 && `+menu+`.hidden && `+box+`.value === ''`)
	b.check()
}

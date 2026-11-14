package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// A paste past the edge of the grid asks Clip, Wrap or Cancel. Escape and Cancel change nothing and give focus
// back to the grid; Clip and Wrap are one undo step each; Wrap follows the fill order picked in Find.
func TestChromeEditorPasteEdge(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Edge one","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"rs":1,"cs":1,"kind":"take","caption":"A"},
		{"r":1,"c":2,"rs":1,"cs":1,"kind":"clear","caption":"B"},
		{"r":2,"c":1,"rs":1,"cs":2,"kind":"take","caption":"C"}]}`)
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Edge one'`)

	b.eval(`window.copied = '';
		navigator.clipboard.writeText = async (t) => { window.copied = t };
		navigator.clipboard.readText = async () => window.copied;
		window.press = (key, code, init = {}, at = document.getElementById('ed-grid')) => { at.focus(); const e = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key, code, ...init }); at.dispatchEvent(e); return e.defaultPrevented; };
		window.goTo = (r, c) => { press('Escape', 'Escape'); press('ArrowDown', 'ArrowDown'); for (let i = 1; i < r; i++) press('ArrowDown', 'ArrowDown'); for (let i = 1; i < c; i++) press('ArrowRight', 'ArrowRight'); };
		window.stat = () => document.getElementById('ed-status').textContent;
		window.state = () => document.getElementById('ed-state').textContent;
		window.edge = () => document.getElementById('ed-edge');
		window.pasteAt = (r, c) => { goTo(r, c); press('v', 'KeyV', { ctrlKey: true }); };
		true`)

	b.eval(`goTo(1, 1); press('ArrowDown', 'ArrowDown', { shiftKey: true }); press('ArrowRight', 'ArrowRight', { shiftKey: true }); press('c', 'KeyC', { metaKey: true })`)
	b.waitFor("copied", `stat() === 'Copied 3 cells.'`)

	// Escape: nothing changes, the selection stays and focus is back on the grid.
	b.eval(`pasteAt(3, 8)`)
	b.waitFor("dialog with the counts and the fill order", `edge()?.textContent.includes('Clip drops 1 cell that falls outside the grid and shrinks 1 merged cell.') && edge().textContent.includes('Wrap moves 2 cells along Across, then down.') && document.activeElement.id === 'ed-edge-clip'`)
	b.shot("paste-edge")
	b.eval(`press('Escape', 'Escape', {}, document.activeElement)`)
	b.waitFor("Escape changes nothing", `!edge() && stat() === 'Nothing pasted.' && document.activeElement.id === 'ed-grid' && !!document.querySelector('.is-sel[data-r="3"][data-c="8"]')`)
	b.waitSaved("Escape left it saved")

	// Clip, reached with the arrow keys; then undo.
	b.eval(`pasteAt(3, 8)`)
	b.waitFor("dialog again", `document.activeElement?.id === 'ed-edge-clip'`)
	b.eval(`press('ArrowRight', 'ArrowRight', {}, document.activeElement)`)
	b.waitFor("arrow moves to Wrap", `document.activeElement.id === 'ed-edge-wrap'`)
	b.eval(`press('ArrowLeft', 'ArrowLeft', {}, document.activeElement)`)
	b.waitFor("and back to Clip", `document.activeElement.id === 'ed-edge-clip'`)
	b.eval(`document.activeElement.click()`)
	b.waitFor("clipped", `!edge() && stat() === 'Pasted 2 cells. Clip dropped 1 cell and shrank 1 merged cell.' && state() === 'Unsaved changes' && document.activeElement.id === 'ed-grid'`)
	b.eval(`document.getElementById('ed-save').click()`)
	b.waitSaved("Clip saved")
	base := []string{"1,1 1x1 take art= artB= A", "1,2 1x1 clear art= artB= B", "2,1 1x2 take art= artB= C"}
	want := append(append([]string{}, base...), "3,8 1x1 take art= artB= A", "4,8 1x1 take art= artB= C")
	if got := storedCells(t, a, ana, id); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("after Clip:\n got %q\nwant %q", got, want)
	}
	b.eval(`press('z', 'KeyZ', { metaKey: true })`)
	b.waitFor("Clip undone", `stat() === 'Undid paste.' && state() === 'Unsaved changes' && !document.querySelector('.sode-ed__sq[data-r="4"][data-c="8"]')?.textContent.includes('C')`)

	// Wrap across, the order Find starts on.
	b.eval(`pasteAt(3, 8)`)
	b.waitFor("dialog for Wrap", `!!edge()`)
	b.eval(`document.getElementById('ed-edge-wrap').click()`)
	b.waitFor("wrapped", `!edge() && stat() === 'Pasted 3 cells. Wrap moved 2 cells along Across, then down.'`)
	b.eval(`document.getElementById('ed-save').click()`)
	b.waitSaved("Wrap saved")
	want = append(append([]string{}, base...), "3,8 1x1 take art= artB= A", "4,1 1x1 clear art= artB= B", "4,2 1x2 take art= artB= C")
	if got := storedCells(t, a, ana, id); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("after Wrap:\n got %q\nwant %q", got, want)
	}
	b.eval(`press('z', 'KeyZ', { metaKey: true })`)
	b.waitFor("Wrap undone", `stat() === 'Undid paste.' && state() === 'Unsaved changes'`)

	// Find's fill order is the one Wrap names and walks.
	b.eval(`goTo(3, 1); press('z', 'KeyZ')`)
	b.waitFor("Find open", `document.querySelector('.sode-find input') === document.activeElement`)
	b.eval(`document.querySelector('.sode-find__orders [data-order="down"]').click()`)
	b.eval(`pasteAt(3, 8)`)
	b.waitFor("Wrap follows Find's order", `edge()?.textContent.includes('Wrap moves 1 cell along Down, then across, and drops 1 that finds no square.')`)
	b.eval(`document.getElementById('ed-edge-cancel').click()`)
	b.waitFor("Cancel changes nothing", `!edge() && stat() === 'Nothing pasted.' && document.activeElement.id === 'ed-grid'`)
	b.check()
}

package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// Cmd+Z undoes a paste and Ctrl+Shift+Z redoes it; the redone paste is what Save stores, and Save leaves the steps
// in place. The 100-step cap is a node test (gridundo.test.mjs).
func TestChromeEditorUndoRedo(t *testing.T) {
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
	id := makePanel(t, a, ana, `{"name":"Undo one","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"rs":1,"cs":1,"kind":"clear","caption":"Wipe"},
		{"r":1,"c":2,"rs":1,"cs":1,"kind":"source","n":1}]}`)
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Undo one'`)

	b.eval(`window.copied = '';
		navigator.clipboard.writeText = async (t) => { window.copied = t };
		navigator.clipboard.readText = async () => window.copied;
		window.press = (key, code, init = {}) => { const g = document.getElementById('ed-grid'); g.focus(); const e = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key, code, ...init }); g.dispatchEvent(e); return e.defaultPrevented; };
		window.goTo = (r, c) => { press('Escape', 'Escape'); press('ArrowDown', 'ArrowDown'); for (let i = 1; i < r; i++) press('ArrowDown', 'ArrowDown'); for (let i = 1; i < c; i++) press('ArrowRight', 'ArrowRight'); };
		window.stat = () => document.getElementById('ed-status').textContent;
		window.state = () => document.getElementById('ed-state').textContent;
		true`)

	b.eval(`goTo(1, 1); press('ArrowRight', 'ArrowRight', { shiftKey: true }); press('c', 'KeyC', { metaKey: true })`)
	b.waitFor("copied", `stat() === 'Copied 2 cells.'`)
	b.eval(`goTo(3, 1); press('v', 'KeyV', { ctrlKey: true })`)
	b.waitFor("pasted", `stat() === 'Pasted 2 cells.' && state() === 'Unsaved changes'`)

	if b.eval(`press('z', 'KeyZ', { metaKey: true })`) != true {
		t.Fatal("Cmd+Z on the grid was not taken by the editor")
	}
	b.waitFor("undone", `stat() === 'Undid paste.' && state() === 'Saved'`)
	b.eval(`press('z', 'KeyZ', { metaKey: true })`)
	b.waitFor("nothing more to undo", `stat() === 'Nothing to undo.'`)
	if b.eval(`press('Z', 'KeyZ', { ctrlKey: true, shiftKey: true })`) != true {
		t.Fatal("Ctrl+Shift+Z on the grid was not taken by the editor")
	}
	b.waitFor("redone", `stat() === 'Redid paste.' && state() === 'Unsaved changes'`)

	b.eval(`document.getElementById('ed-save').click()`)
	b.waitFor("saved", `state() === 'Saved'`)
	got := storedCells(t, a, ana, id)
	want := []string{"1,1 1x1 clear art= artB= Wipe", "1,2 1x1 source art= artB=", "3,1 1x1 clear art= artB= Wipe", "3,2 1x1 source art= artB="}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("stored cells:\n got %q\nwant %q", got, want)
	}

	// Save keeps the steps; a selection move is not one, and Delete is.
	b.eval(`goTo(1, 1); press('Delete', 'Delete'); goTo(2, 2)`)
	b.waitFor("deleted", `state() === 'Unsaved changes'`)
	b.eval(`press('z', 'KeyZ', { ctrlKey: true })`)
	b.waitFor("Delete undone", `stat() === 'Undid Delete.' && state() === 'Saved'`)
	b.eval(`press('z', 'KeyZ', { ctrlKey: true })`)
	b.waitFor("paste undone after Save", `stat() === 'Undid paste.' && state() === 'Unsaved changes'`)

	// A focused text field keeps its own Cmd+Z.
	b.eval(`{ const n = document.getElementById('ed-name'); n.focus(); const e = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'z', code: 'KeyZ', metaKey: true }); n.dispatchEvent(e); window.fieldUndo = e.defaultPrevented }`)
	if b.eval(`window.fieldUndo`) != false || b.eval(`stat()`) != "Undid paste." {
		t.Fatal("Cmd+Z in the name field was taken by the grid")
	}
	b.check()
}

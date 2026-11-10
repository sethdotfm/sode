package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// Cmd or Ctrl+X cuts them. Cmd or Ctrl+C copies cells, Cmd or Ctrl+V pastes them elsewhere on the same panel (through readText, as a synthetic
// key fires no paste event) and into a second panel (through the browser's paste event), where the uploaded image goes.
func TestChromeEditorCopyPaste(t *testing.T) {
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
	id := makePanel(t, a, ana, `{"name":"Clip one"}`)
	art := apiBody(t, apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, b64(t, tinyPNG)))["id"].(string)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Clip one","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"rs":1,"cs":1,"kind":"take","art":"`+art+`","artB":"2"},
		{"r":1,"c":2,"rs":1,"cs":1,"kind":"clear","art":"icon:camera","caption":"Wipe"},
		{"r":1,"c":3,"rs":2,"cs":1,"kind":"source","n":1,"levels":[1]}]}`); rec.Code != 200 {
		t.Fatalf("PUT %d %s", rec.Code, rec.Body)
	}
	other := makePanel(t, a, ana, `{"name":"Clip two","rows":4,"cols":8,"cells":[]}`)
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Clip one'`)

	b.eval(`window.copied = '';
		navigator.clipboard.writeText = async (t) => { window.copied = t };
		navigator.clipboard.readText = async () => window.copied;
		window.press = (key, code, init = {}) => { const g = document.getElementById('ed-grid'); g.focus(); const e = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key, code, ...init }); g.dispatchEvent(e); return e.defaultPrevented; };
		window.goTo = (r, c) => { press('Escape', 'Escape'); press('ArrowDown', 'ArrowDown'); for (let i = 1; i < r; i++) press('ArrowDown', 'ArrowDown'); for (let i = 1; i < c; i++) press('ArrowRight', 'ArrowRight'); };
		window.stat = () => document.getElementById('ed-status').textContent;
		window.pasteEvent = (text) => { const dt = new DataTransfer(); dt.setData('text/plain', text); document.getElementById('ed-grid').focus(); document.getElementById('ed-grid').dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true })); };
		true`)

	b.eval(`goTo(1, 1); press('ArrowRight', 'ArrowRight', { shiftKey: true }); press('ArrowRight', 'ArrowRight', { shiftKey: true })`)
	if b.eval(`press('c', 'KeyC', { metaKey: true })`) != true {
		t.Fatal("Cmd+C on the grid was not taken by the editor")
	}
	b.waitFor("copied", `window.copied.includes('sode-panel-cells') && stat() === 'Copied 3 cells.'`)

	// Text that is not a Sode block, and a paste past the edge, leave the grid as it is.
	b.eval(`goTo(3, 2); pasteEvent('Cam 1')`)
	b.waitFor("plain text is refused", `stat().includes('no Sode cells') && document.getElementById('ed-state').textContent === 'Saved'`)
	b.eval(`goTo(4, 8)`)
	if b.eval(`press('v', 'KeyV', { ctrlKey: true })`) != false {
		t.Fatal("Ctrl+V was not left to the browser")
	}
	b.waitFor("a paste past the edge asks first", `!!document.getElementById('ed-edge-clip')`)
	b.eval(`document.getElementById('ed-edge-cancel').click()`)
	b.waitFor("Cancel changes nothing", `!document.getElementById('ed-edge') && stat() === 'Nothing pasted.' && document.getElementById('ed-state').textContent === 'Saved'`)

	b.eval(`goTo(3, 2); press('v', 'KeyV', { ctrlKey: true })`)
	b.waitFor("pasted on the same panel", `stat() === 'Pasted 3 cells.'`)
	b.eval(`document.getElementById('ed-save').click()`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved'`)
	got := storedCells(t, a, ana, id)
	want := []string{
		"1,1 1x1 take art=" + art + " artB=2", "1,2 1x1 clear art=icon:camera artB= Wipe", "1,3 2x1 source art= artB=",
		"3,2 1x1 take art=" + art + " artB=2", "3,3 1x1 clear art=icon:camera artB= Wipe", "3,4 2x1 source art= artB=",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("same panel cells:\n got %q\nwant %q", got, want)
	}

	// The second panel, through the picker, takes the same text through the browser's paste event.
	b.eval(fmt.Sprintf(`{ const p = document.getElementById("ed-pick"); p.value = %q; p.dispatchEvent(new Event("change")) }`, other))
	b.waitFor("second panel loaded", `document.getElementById('ed-name').value === 'Clip two'`)
	b.eval(`goTo(2, 5); pasteEvent(window.copied)`)
	b.waitFor("pasted into the second panel without the uploaded image", `stat() === 'Pasted 3 cells. 1 cell lost its uploaded image, since images stay with the panel they were added to.'`)
	b.eval(`document.getElementById('ed-save').click()`)
	b.waitFor("second panel saved", `document.getElementById('ed-state').textContent === 'Saved'`)
	got = storedCells(t, a, ana, other)
	want = []string{"2,5 1x1 take art= artB=2", "2,6 1x1 clear art=icon:camera artB= Wipe", "2,7 2x1 source art= artB="}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("second panel cells:\n got %q\nwant %q", got, want)
	}

	// Cut copies like Cmd+C and then removes the selection as one change; the clipboard still pastes it back.
	b.eval(`goTo(2, 5); press('ArrowRight', 'ArrowRight', { shiftKey: true }); window.copied = ''`)
	if b.eval(`press('x', 'KeyX', { metaKey: true })`) != true {
		t.Fatal("Cmd+X on the grid was not taken by the editor")
	}
	b.waitFor("cut", `window.copied.includes('sode-panel-cells') && stat() === 'Cut 2 cells.' && document.getElementById('ed-state').textContent !== 'Saved'`)
	b.eval(`document.getElementById('ed-save').click()`)
	b.waitFor("cut saved", `document.getElementById('ed-state').textContent === 'Saved'`)
	got = storedCells(t, a, ana, other)
	want = []string{"2,7 2x1 source art= artB="}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("after the cut:\n got %q\nwant %q", got, want)
	}
	b.eval(`goTo(4, 1); press('v', 'KeyV', { ctrlKey: true })`)
	b.waitFor("cut cells paste back", `stat().startsWith('Pasted 2 cells.')`)
	// A focused text field keeps its own Cmd+X: the grid neither copies nor removes anything.
	b.eval(`window.copied = ''; { const n = document.getElementById('ed-name'); n.focus(); const e = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'x', code: 'KeyX', metaKey: true }); n.dispatchEvent(e); window.fieldCut = e.defaultPrevented }`)
	if b.eval(`window.fieldCut`) != false || b.eval(`window.copied`) != "" {
		t.Fatal("Cmd+X in the name field was taken by the grid")
	}
	b.check()
}

func storedCells(t *testing.T, a *App, key, id string) []string {
	t.Helper()
	var p struct {
		Pages []struct {
			Cells []struct {
				R, C, Rs, Cs    int
				Kind, Art, ArtB string
				Caption         string
				Levels          []int
			} `json:"cells"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(apiSend(a, "GET", "/api/v1/panels/"+id, key, "").Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range p.Pages[0].Cells {
		s := fmt.Sprintf("%d,%d %dx%d %s art=%s artB=%s", c.R, c.C, c.Rs, c.Cs, c.Kind, c.Art, c.ArtB)
		if c.Caption != "" {
			s += " " + c.Caption
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

package app

import "testing"

// The toolbar shows the panel name once, in the panel picker, with the Rename icon beside it. It opens the Panel
// tab with the Name field focused, and a name typed there reaches the hidden toolbar field.
func TestChromeEditorRenameButton(t *testing.T) {
	b, _, _, _ := editorTab(t, `{"name":"Wall","rows":2,"cols":2,"cells":[]}`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Wall'`)
	b.waitFor("the Rename icon shown", `!!document.getElementById('ed-name-btn')?.querySelector('svg') && document.getElementById('ed-name-btn').getAttribute('aria-label') === 'Rename'`)
	if tip := b.eval(`document.getElementById('ed-name-btn').dataset.pbTip`); tip != "Rename this panel" {
		t.Errorf("the Rename icon's tip is %v", tip)
	}
	// The picker's option is the one place: nothing else in the toolbar holds the name.
	if n := b.eval(`(() => { const c = document.querySelector('.sode-ed__bar').cloneNode(true); c.querySelector('#ed-pick').remove(); return c.textContent.split('Wall').length - 1 + (document.getElementById('ed-pick').selectedOptions[0]?.text === 'Wall' ? 1 : 0); })()`); n != float64(1) {
		t.Errorf("the toolbar shows the name %v times, want once", n)
	}
	if v := b.eval(`document.getElementById('ed-name').closest('label').hidden`); v != true {
		t.Errorf("the toolbar's name field shows, hidden = %v", v)
	}

	// A real Tab (a script KeyboardEvent moves no focus) goes from the panel picker to the Rename icon.
	b.eval(`document.getElementById('ed-pick').focus(); true`)
	for _, typ := range []string{"keyDown", "keyUp"} {
		b.call("Input.dispatchKeyEvent", map[string]any{"type": typ, "key": "Tab", "code": "Tab", "windowsVirtualKeyCode": 9})
	}
	b.waitFor("Tab reaches the Rename icon", `document.activeElement === document.getElementById('ed-name-btn')`)

	b.eval(`document.getElementById('tab-ed-cell').click(); document.getElementById('ed-name-btn').click(); true`)
	b.waitFor("the Panel tab opens with Name focused", `document.getElementById('tab-ed-panel').getAttribute('aria-selected') === 'true' && document.activeElement === document.getElementById('ed-side-name')`)
	b.eval(`{ const n = document.getElementById('ed-side-name'); n.value = 'Wall two'; n.dispatchEvent(new Event('input', { bubbles: true })); } true`)
	b.waitFor("the name reaches the editor", `document.getElementById('ed-name').value === 'Wall two' && document.getElementById('ed-state').textContent === 'Unsaved changes'`)
	b.check()
}

package app

import "testing"

// The toolbar shows the panel name once, beside a Rename button that opens the name field; Enter keeps the edit and
// Escape ends it.
func TestChromeEditorRenameButton(t *testing.T) {
	b, _, _, _ := editorTab(t, `{"name":"Wall","rows":2,"cols":2,"cells":[]}`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Wall'`)
	b.waitFor("Rename button shown", `document.getElementById('ed-name-btn')?.textContent === 'Rename' && !document.getElementById('ed-name-btn').querySelector('svg')`)
	if n := b.eval(`document.querySelector('.sode-ed__bar').innerText.split('Wall').length - 1`); n != float64(1) {
		t.Errorf("the toolbar's text holds the name %v times, want once", n)
	}
	if v := b.eval(`document.getElementById('ed-name').closest('label').hidden`); v != true {
		t.Errorf("the name field is open before Rename, hidden = %v", v)
	}
	b.eval(`document.getElementById('ed-name-btn').click(); true`)
	b.waitFor("field open and focused", `!document.getElementById('ed-name').closest('label').hidden && document.activeElement === document.getElementById('ed-name')`)
	b.eval(`document.getElementById('ed-name').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); true`)
	b.waitFor("Escape closed the field and focus returned", `document.getElementById('ed-name').closest('label').hidden && document.activeElement === document.getElementById('ed-name-btn')`)
	b.eval(`document.getElementById('ed-name-btn').click(); const n = document.getElementById('ed-name'); n.value = 'Wall two'; n.dispatchEvent(new Event('input', { bubbles: true })); n.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })); true`)
	b.waitFor("Enter shows the new name once", `document.querySelector('.sode-ed__name-text').textContent === 'Wall two' && document.querySelector('.sode-ed__bar').innerText.split('Wall two').length === 2`)
	b.check()
}

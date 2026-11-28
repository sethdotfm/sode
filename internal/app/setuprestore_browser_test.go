package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// Restore from a backup on the Welcome step. A file with no PINs asks for the first Administrator's PIN. The
// page reloads out of the wizard after the restore is written.
func TestChromeSetupRestore(t *testing.T) {
	b := startBrowser(t)
	_, file := backupOf(t, false)
	a := setupApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	type_ := func(id, text string) {
		b.eval(`(() => { const i = document.getElementById('` + id + `'); i.focus(); i.value = '` + text + `'; i.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	}
	b.open(srv.URL + "/")
	b.waitFor("Welcome", `document.getElementById('setup-title').textContent === 'Set up Sode'`)
	b.waitFor("Restore from a backup", `!document.getElementById('setup-restore').hidden && !document.getElementById('setup-restore').disabled`)
	upload := func() {
		b.eval(fmt.Sprintf(`(() => {
		window.beforeRestore = true;
		const u8 = Uint8Array.from(atob('%s'), (c) => c.charCodeAt(0));
		const dt = new DataTransfer();
		dt.items.add(new File([u8], 'backup.zip', { type: 'application/zip' }));
		const inp = document.getElementById('setup-restore-file');
		inp.files = dt.files;
		inp.dispatchEvent(new Event('change'));
		return true;
	})()`, base64.StdEncoding.EncodeToString(file)))
	}
	upload()
	b.waitFor("the PIN for Ada", `document.getElementById('setup-title').textContent === 'Restore from a backup' && document.getElementById('setup-body').textContent.includes('Ada') && !!document.getElementById('setup-restore-pin')`)
	b.shot("pin")
	// The page uses the wizard's footer: Back on the left, Restore as the main action on the right, and no buttons
	// in the body. The step chips go nowhere, since a chip would drop the picked file.
	b.waitFor("the footer holds Back and Restore", `(() => {
		const shown = (id) => { const n = document.getElementById(id); return !n.hidden && !!n.offsetParent; };
		const back = document.getElementById('setup-back').getBoundingClientRect();
		const next = document.getElementById('setup-next').getBoundingClientRect();
		return shown('setup-back') && shown('setup-next') && !shown('setup-restore') && !document.querySelector('#setup-body button.pb-btn:not(.sode__pinshow-btn)')
			&& document.getElementById('setup-back').textContent === 'Back' && document.getElementById('setup-next').textContent === 'Restore'
			&& back.right < next.left && !document.querySelector('#setup-steps [data-go]');
	})()`)
	b.sameMiddle("the two PIN fields in one row", "#setup-restore-pin", "#setup-restore-pin2")
	b.eval(`document.getElementById('setup-back').click(); true`)
	b.waitFor("Back goes to Welcome", `document.getElementById('setup-title').textContent === 'Set up Sode' && !document.getElementById('setup-restore').hidden
		&& document.getElementById('setup-next').textContent === 'Next'`)
	upload()
	b.waitFor("the PIN page again", `!!document.getElementById('setup-restore-pin')`)
	if !a.setupPending() {
		t.Fatal("the file was restored before the PIN")
	}
	type_("setup-restore-pin", "97531")
	type_("setup-restore-pin2", "97530")
	go_ := `document.getElementById('setup-next')`
	b.waitFor("PINs that differ stop Restore", go_+`.disabled`)
	type_("setup-restore-pin2", "97531")
	b.waitFor("Restore", `!`+go_+`.disabled`)
	b.eval(go_ + `.click(); true`)
	b.waitFor("the page reloads out of the wizard", `!window.beforeRestore && document.getElementById('setup').hidden && !document.documentElement.hasAttribute('data-sode-setup')`)
	if a.setupPending() {
		t.Fatal("setup still pending")
	}
	signInToken(t, a, "Ada", "97531")
}

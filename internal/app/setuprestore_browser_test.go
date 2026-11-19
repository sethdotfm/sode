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
// page reloads out of the wizard once the restore is written.
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
	b.waitFor("Welcome", `document.getElementById('setup-title').textContent === 'Welcome to Sode'`)
	b.waitFor("Restore from a backup", `!document.getElementById('setup-restore').hidden && !document.getElementById('setup-restore').disabled`)
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
	b.waitFor("the PIN for Ada", `document.getElementById('setup-title').textContent === 'Restore from a backup' && document.getElementById('setup-body').textContent.includes('Ada') && !!document.getElementById('setup-restore-pin')`)
	b.shot("pin")
	if !a.setupPending() {
		t.Fatal("the file was restored before the PIN")
	}
	type_("setup-restore-pin", "97531")
	type_("setup-restore-pin2", "97530")
	go_ := `document.querySelector('.sode-setup__restore .pb-btn--ok')`
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

// Package web embeds the browser UI.
package web

import "embed"

//go:embed index.html log.html panels.html paneleditor.html panel.html *.js base.css desktop.css mobile.css history.css log.css panels.css panelmon.css lock.css touch.css vendor
var FS embed.FS

// Package web embeds the browser UI.
package web

import "embed"

//go:embed index.html log.html favicon.svg *.js base.css desktop.css mobile.css history.css log.css touch.css vendor
var FS embed.FS

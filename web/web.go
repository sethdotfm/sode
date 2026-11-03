// Package web embeds the browser UI.
package web

import "embed"

//go:embed index.html log.html grid.html settings-grids.html *.js base.css desktop.css mobile.css history.css log.css grid.css lock.css touch.css vendor
var FS embed.FS

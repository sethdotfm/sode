// Package web embeds the browser UI.
package web

import "embed"

//go:embed index.html log.html favicon.svg icon.svg icon-192.png icon-512.png apple-touch-icon.png manifest.webmanifest *.js base.css desktop.css mobile.css history.css log.css touch.css vendor
var FS embed.FS

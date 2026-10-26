// Package web embeds the browser UI.
package web

import "embed"

//go:embed index.html log.html *.js sode.css vendor
var FS embed.FS

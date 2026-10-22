// Package web embeds the browser UI.
package web

import "embed"

//go:embed index.html *.js sode.css vendor
var FS embed.FS

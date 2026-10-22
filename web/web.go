// Package web embeds the browser UI.
package web

import "embed"

//go:embed index.html app.js sode.css vendor
var FS embed.FS

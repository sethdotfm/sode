package app

import (
	"strings"
	"testing"
)

// A panel shows artwork fetched with its key as a blob URL in an <img>; scripts stay same-origin only.
func TestCSPAllowsBlobImagesOnly(t *testing.T) {
	var img, script string
	for _, d := range strings.Split(csp, ";") {
		d = strings.TrimSpace(d)
		switch {
		case strings.HasPrefix(d, "img-src"):
			img = d
		case strings.HasPrefix(d, "script-src"):
			script = d
		}
	}
	if !strings.Contains(img, "blob:") {
		t.Errorf("img-src %q lacks blob:", img)
	}
	if strings.Contains(script, "blob:") {
		t.Errorf("blob: must not reach scripts: %q", script)
	}
}

package app

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

// staticFiles serves the embedded web files with an ETag and a gzip copy. Each is made once, on
// the file's first request, not in Handler, which tests call many times. The files cannot
// change while Sode runs. With "no-cache" and the ETag, a new build shows at once, and an
// unchanged file costs one 304.
type staticFiles struct {
	fsys  fs.FS
	files sync.Map // name -> *staticFile
}

type staticFile struct {
	once  sync.Once
	ok    bool
	body  []byte
	gz    []byte // nil when gzip does not make the file smaller or the type is compressed
	etag  string
	ctype string
}

func (s *staticFiles) get(name string) *staticFile {
	v, _ := s.files.LoadOrStore(name, &staticFile{})
	f := v.(*staticFile)
	f.once.Do(func() {
		b, err := fs.ReadFile(s.fsys, name)
		if err != nil {
			return
		}
		sum := sha256.Sum256(b)
		f.ok, f.body = true, b
		f.etag = `"` + hex.EncodeToString(sum[:12]) + `"`
		f.ctype = mime.TypeByExtension(path.Ext(name))
		if f.ctype == "" {
			f.ctype = http.DetectContentType(b)
		}
		if compressible(f.ctype) {
			var buf bytes.Buffer
			zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			zw.Write(b)
			zw.Close()
			// Under 10% saved is not worth the client's decode.
			if buf.Len() < len(b)*9/10 {
				f.gz = buf.Bytes()
			}
		}
	})
	if !f.ok {
		return nil
	}
	return f
}

func compressible(ctype string) bool {
	t, _, _ := strings.Cut(ctype, ";")
	switch t {
	case "application/javascript", "text/javascript", "application/json", "image/svg+xml", "application/manifest+json":
		return true
	}
	return strings.HasPrefix(t, "text/")
}

// serve writes the file or a 304; false when name is not a file, so the caller can fall back.
func (s *staticFiles) serve(w http.ResponseWriter, r *http.Request, name string) bool {
	f := s.get(name)
	if f == nil {
		return false
	}
	h := w.Header()
	h.Set("Cache-Control", "no-cache")
	h.Set("Content-Type", f.ctype)
	body, etag := f.body, f.etag
	if f.gz != nil {
		h.Add("Vary", "Accept-Encoding")
		if acceptsGzip(r) {
			body = f.gz
			// A distinct tag per encoding, so a cache never pairs a 304 with the other body.
			etag = strings.TrimSuffix(etag, `"`) + `-gz"`
			h.Set("Content-Encoding", "gzip")
			// A range of the gzip bytes means nothing to a client that asked for the plain file.
			r.Header.Del("Range")
		}
	}
	h.Set("ETag", etag)
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
	return true
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		enc, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(enc), "gzip") {
			continue
		}
		q := strings.ReplaceAll(params, " ", "")
		return q != "q=0" && q != "q=0.0" && q != "q=0.00" && q != "q=0.000"
	}
	return false
}

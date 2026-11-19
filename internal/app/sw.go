package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"
)

// shellWorker serves web/sw.js with this build's name and the app shell's files filled in. The build name is the
// version label and a hash of every web file. So any change to them makes a new worker and a new cache. It is made
// once, on the first request, since the files cannot change while Sode runs.
type shellWorker struct {
	fsys fs.FS
	once sync.Once
	body []byte
	etag string
}

// shellExts: the files a page loads. HTML is kept only for the pages a device opens (sw.js).
var shellExts = map[string]bool{".js": true, ".css": true, ".svg": true, ".png": true, ".woff2": true}

func (s *shellWorker) make() {
	src, err := fs.ReadFile(s.fsys, "sw.js")
	if err != nil {
		return
	}
	sum := sha256.New()
	var shell []string
	err = fs.WalkDir(s.fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(s.fsys, name)
		if err != nil {
			return err
		}
		sum.Write([]byte(name + "\x00"))
		sum.Write(b)
		if shellExts[path.Ext(name)] && name != "sw.js" {
			shell = append(shell, name)
		}
		return nil
	})
	if err != nil {
		log.Printf("service worker: %v", err)
		return
	}
	build := BuildLabel() + " " + hex.EncodeToString(sum.Sum(nil)[:6])
	list, _ := json.Marshal(shell)
	body := bytes.Replace(src, []byte(`'__SODE_BUILD__'`), []byte(strconv.Quote(build)), 1)
	body = bytes.Replace(body, []byte(`['__SODE_SHELL__']`), list, 1)
	tag := sha256.Sum256(body)
	s.body, s.etag = body, `"`+hex.EncodeToString(tag[:12])+`"`
}

func (s *shellWorker) serve(w http.ResponseWriter, r *http.Request) {
	s.once.Do(s.make)
	if s.body == nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/javascript; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("ETag", s.etag)
	http.ServeContent(w, r, "sw.js", time.Time{}, bytes.NewReader(s.body))
}

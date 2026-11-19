package app

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

func artBytes(p Panel) int {
	n := 0
	for _, a := range p.Art {
		n += a.Size
	}
	return n
}

func (s *panelStore) artPath(id, art string) string {
	return filepath.Join(s.dir, id+".art", art)
}

// data is already type-checked.
func (s *panelStore) addArt(id string, data []byte, typ string) (PanelArt, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.by[id]
	if !ok {
		return PanelArt{}, false, nil
	}
	if artBytes(p)+len(data) > panelArtTotal {
		return PanelArt{}, true, errArtCap
	}
	b := make([]byte, 4)
	rand.Read(b)
	art := PanelArt{ID: "a-" + hex.EncodeToString(b), Type: typ, Size: len(data)}
	if s.dir != "" {
		if err := os.MkdirAll(filepath.Join(s.dir, id+".art"), 0o700); err != nil {
			return PanelArt{}, true, err
		}
		if err := config.AtomicWrite(s.artPath(id, art.ID), data); err != nil {
			return PanelArt{}, true, err
		}
	} else {
		s.art[id+"/"+art.ID] = data
	}
	p.Art = append(slices.Clone(p.Art), art)
	if err := s.writeLocked(&p); err != nil {
		return PanelArt{}, true, err
	}
	s.by[id] = p
	return art, true, nil
}

func (s *panelStore) artData(id, art string) ([]byte, PanelArt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.by[id]
	if !ok {
		return nil, PanelArt{}, false
	}
	i := slices.IndexFunc(p.Art, func(a PanelArt) bool { return a.ID == art })
	if i < 0 {
		return nil, PanelArt{}, false
	}
	if s.dir == "" {
		b, ok := s.art[id+"/"+art]
		return b, p.Art[i], ok
	}
	b, err := os.ReadFile(s.artPath(id, art))
	return b, p.Art[i], err == nil
}

func (s *panelStore) delArt(id, art string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.by[id]
	if !ok {
		return false, nil
	}
	i := slices.IndexFunc(p.Art, func(a PanelArt) bool { return a.ID == art })
	if i < 0 {
		return false, nil
	}
	p.Art = slices.Delete(slices.Clone(p.Art), i, i+1)
	if err := s.writeLocked(&p); err != nil {
		return true, err
	}
	s.by[id] = p
	if s.dir != "" {
		os.Remove(s.artPath(id, art))
	}
	delete(s.art, id+"/"+art)
	return true, nil
}

// copyArt gives dst a copy of src's artwork under the same ids, so cells that name an artwork
// id keep working in the copy. It copies what fits under panelArtTotal, in order, and returns
// the ids that did not fit (or could not be read or written). Caller holds no lock.
func (s *panelStore) copyArt(src Panel, dst string) []string {
	t, ok := s.get(dst)
	if !ok {
		return nil
	}
	have := map[string]bool{}
	for _, a := range t.Art {
		have[a.ID] = true
	}
	var left []string
	for _, a := range src.Art {
		if have[a.ID] {
			continue
		}
		b, _, ok := s.artData(src.ID, a.ID)
		if !ok || !s.putArtAs(dst, a, b) {
			left = append(left, a.ID)
		}
	}
	return left
}

// putArtAs checks the total under the lock, since it is possible that an upload or another copy lands between
// copyArt's reads. Only the total: older artwork past 2 MB or 1024 by 1024 still copies.
func (s *panelStore) putArtAs(id string, art PanelArt, data []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.by[id]
	if !ok {
		return false
	}
	art.Size = len(data)
	if artBytes(p)+art.Size > panelArtTotal {
		return false
	}
	if s.dir != "" {
		if os.MkdirAll(filepath.Join(s.dir, id+".art"), 0o700) != nil || config.AtomicWrite(s.artPath(id, art.ID), data) != nil {
			return false
		}
	} else {
		s.art[id+"/"+art.ID] = data
	}
	p.Art = append(slices.Clone(p.Art), art)
	if s.writeLocked(&p) != nil {
		return false
	}
	s.by[id] = p
	return true
}

const svgNS = "http://www.w3.org/2000/svg"

// sniffArt decides the type from the bytes, never from a name or header. "" = not allowed.
func sniffArt(data []byte) string {
	switch t := http.DetectContentType(data); t {
	case "image/png", "image/jpeg", "image/gif":
		return t
	}
	if isSVG(data) {
		return "image/svg+xml"
	}
	return ""
}

// artTypeOK is a stored type sniffArt could have given. A hand-edited index must not choose the Content-Type.
func artTypeOK(t string) bool {
	switch t {
	case "image/png", "image/jpeg", "image/gif", "image/svg+xml":
		return true
	}
	return false
}

// isSVG: the first element is svg, after only a declaration, comments, a doctype and blanks.
func isSVG(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			return t.Name.Local == "svg" && (t.Name.Space == "" || t.Name.Space == svgNS)
		case xml.CharData:
			if len(bytes.TrimSpace(t)) > 0 {
				return false
			}
		}
	}
}

// The image is the raw body. Its content decides its type: Sode ignores the request's Content-Type and any file name.
func (a *App) apiArtUpload(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, panelArtBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			apiFail(w, http.StatusRequestEntityTooLarge, "too_large", "artwork is at most 2 MB")
		} else {
			apiFail(w, http.StatusBadRequest, "bad_request", "the body could not be read")
		}
		return
	}
	typ, f := checkArtBytes(data)
	if f != nil {
		failWith(w, f)
		return
	}
	art, _, err := a.panels.addArt(p.ID, data, typ)
	if err != nil {
		failStore(w, err, panelSaveFail)
		return
	}
	apiCreated(w, art)
}

// Panel uploads and Default artwork both pass through it.
func checkArtBytes(data []byte) (string, error) {
	if len(data) > panelArtBytes {
		return "", &panelClientFail{http.StatusRequestEntityTooLarge, "too_large", "artwork is at most 2 MB"}
	}
	typ := sniffArt(data)
	if typ == "" {
		return "", &panelClientFail{http.StatusUnsupportedMediaType, "unsupported_type", "artwork is a JPEG, PNG, GIF or SVG image"}
	}
	if typ != "image/svg+xml" {
		// DecodeConfig reads the header only, so a huge image costs no pixel memory here.
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return "", &panelClientFail{http.StatusBadRequest, "bad_request", "the image header could not be read"}
		}
		if cfg.Width > panelArtSide || cfg.Height > panelArtSide {
			msg := fmt.Sprintf("artwork is at most %d by %d pixels, this one is %d by %d", panelArtSide, panelArtSide, cfg.Width, cfg.Height)
			return "", &panelClientFail{http.StatusBadRequest, "bad_request", msg}
		}
	}
	return typ, nil
}

// The headers keep a script in an SVG from running even when someone opens the URL: sandbox without allow-scripts,
// nothing loadable, no sniffing. Pages must show it as an image, never inline.
func serveArtBytes(w http.ResponseWriter, typ string, data []byte) {
	h := w.Header()
	h.Set("Content-Type", typ)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Cache-Control", "private, no-cache")
	w.Write(data)
}

// Serves the stored type, never one the uploader named.
func (a *App) apiArtGet(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	data, art, found := a.panels.artData(p.ID, r.PathValue("art"))
	if !found || !artIDRe.MatchString(art.ID) {
		apiFail(w, http.StatusNotFound, "not_found", "no such artwork on this panel")
		return
	}
	serveArtBytes(w, art.Type, data)
}

func (a *App) apiArtDelete(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, true)
	if !ok {
		return
	}
	found, err := a.panels.delArt(p.ID, r.PathValue("art"))
	switch {
	case err != nil:
		failStore(w, err, panelSaveFail)
	case !found:
		apiFail(w, http.StatusNotFound, "not_found", "no such artwork on this panel")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

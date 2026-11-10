package app

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The Icon library's drop-in directory, icons/drop-in: PNG and SVG files put there are added at start and on a Rescan,
// then removed from the directory. A skipped file stays and is reported each time. Docs: docs/reference.md.

const iconDropDir = "drop-in"

type iconSkip struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

type iconRescanResult struct {
	Added   []LibIcon  `json:"added"`
	Skipped []iconSkip `json:"skipped"`
}

// checkLibBytes is checkArtBytes narrowed to the library's PNG and SVG, so upload and drop-in take the same files.
func checkLibBytes(data []byte) (string, error) {
	typ, err := checkArtBytes(data)
	if err != nil {
		return "", err
	}
	if typ != "image/png" && typ != "image/svg+xml" {
		return "", &panelClientFail{http.StatusUnsupportedMediaType, "unsupported_type", "a library icon is a PNG or SVG image"}
	}
	return typ, nil
}

// dropName is the file name without its extension, cut to iconNameMax. Keep in step with libNameOf in
// web/iconlibrules.js.
func dropName(file string) string {
	return cleanText(strings.TrimSuffix(file, filepath.Ext(file)), iconNameMax)
}

// rescan adds the drop-in directory's files. Links are never followed: a linked directory is refused and a linked file is
// skipped, and reads go through an os.Root so nothing outside the directory opens.
func (s *iconLib) rescan() iconRescanResult {
	out := iconRescanResult{Added: []LibIcon{}, Skipped: []iconSkip{}}
	if s.path == "" {
		return out
	}
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	dir := filepath.Join(s.dir(), iconDropDir)
	fi, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			log.Printf("icon library: %v", err)
		}
		return out
	}
	if err != nil || !fi.IsDir() {
		out.Skipped = append(out.Skipped, iconSkip{iconDropDir, "the drop-in directory is not a plain directory, so it was not read (a link is never followed)"})
		return out
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		out.Skipped = append(out.Skipped, iconSkip{iconDropDir, "the drop-in directory could not be read: " + err.Error()})
		return out
	}
	defer root.Close()
	if rfi, err := root.Stat("."); err != nil || !os.SameFile(fi, rfi) {
		out.Skipped = append(out.Skipped, iconSkip{iconDropDir, "the drop-in directory changed while it was read"})
		return out
	}
	d, err := root.Open(".")
	if err != nil {
		out.Skipped = append(out.Skipped, iconSkip{iconDropDir, "the drop-in directory could not be read: " + err.Error()})
		return out
	}
	entries, err := d.ReadDir(-1)
	d.Close()
	if err != nil {
		out.Skipped = append(out.Skipped, iconSkip{iconDropDir, "the drop-in directory could not be read: " + err.Error()})
		return out
	}
	// Sorted, so clashes inside one scan number the same way each time.
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") { // .DS_Store and the like
			continue
		}
		ic, err := s.dropOne(root, e)
		if err != nil {
			out.Skipped = append(out.Skipped, iconSkip{name, err.Error()})
			continue
		}
		out.Added = append(out.Added, ic)
		if err := root.Remove(name); err != nil {
			log.Printf("icon library: added %s but could not remove it from the drop-in directory: %v", name, err)
		}
	}
	return out
}

func (s *iconLib) dropOne(root *os.Root, e os.DirEntry) (LibIcon, error) {
	name := e.Name()
	if !e.Type().IsRegular() {
		return LibIcon{}, errors.New("not a plain file; links and directories are not read")
	}
	if ext := strings.ToLower(filepath.Ext(name)); ext != ".png" && ext != ".svg" {
		return LibIcon{}, errors.New("a library icon is a PNG or SVG image")
	}
	f, err := root.Open(name)
	if err != nil {
		return LibIcon{}, fmt.Errorf("could not be read: %w", err)
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return LibIcon{}, errors.New("not a plain file; links and directories are not read")
	}
	// One byte past the limit is enough for checkArtBytes to call it too large.
	data, err := io.ReadAll(io.LimitReader(f, panelArtBytes+1))
	if err != nil {
		return LibIcon{}, fmt.Errorf("could not be read: %w", err)
	}
	typ, err := checkLibBytes(data)
	if err != nil {
		return LibIcon{}, err
	}
	ic, err := s.add(dropName(name), data, typ)
	if errors.Is(err, errIconBadName) {
		return LibIcon{}, errors.New("the file name gives no icon name")
	}
	return ic, err
}

// skippedText is the report of a rescan's skipped files, or "".
func (r iconRescanResult) skippedText() string {
	var parts []string
	for _, s := range r.Skipped {
		parts = append(parts, s.File+": "+s.Reason)
	}
	return strings.Join(parts, "; ")
}

// rescanIcons runs a rescan and logs it as an `iconRescan` row when it added or skipped anything.
func (a *App) rescanIcons(act Actor) iconRescanResult {
	res := a.icons.rescan()
	if len(res.Added) == 0 && len(res.Skipped) == 0 {
		return res
	}
	var names []string
	for _, ic := range res.Added {
		names = append(names, ic.Name)
	}
	note := fmt.Sprintf("Icon library drop-in: added %d", len(res.Added))
	if len(names) > 0 {
		note += " (" + strings.Join(names, ", ") + ")"
	}
	var err error
	if len(res.Skipped) > 0 {
		note += fmt.Sprintf(", skipped %d", len(res.Skipped))
		err = errors.New(res.skippedText())
		log.Printf("icon library: drop-in skipped %s", res.skippedText())
	}
	a.apiEvent(act, "iconRescan", "ok", note, err)
	return res
}

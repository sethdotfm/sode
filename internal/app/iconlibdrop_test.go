package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func dropDir(a *App) string {
	return filepath.Join(filepath.Dir(a.cfgPath), iconLibDir, iconDropDir)
}

func dropFile(t *testing.T, a *App, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dropDir(a), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dropDir(a), name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func iconNames(a *App) []string {
	var out []string
	for _, ic := range a.icons.list() {
		out = append(out, ic.Name)
	}
	slices.Sort(out)
	return out
}

func rescanBody(t *testing.T, a *App, key, body string) (added []LibIcon, skipped []iconSkip) {
	t.Helper()
	rec := apiSend(a, "POST", "/api/v1/icon-library/rescan", key, body)
	if rec.Code != 200 {
		t.Fatalf("rescan: %d %s", rec.Code, rec.Body.String())
	}
	var got iconRescanResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got.Added, got.Skipped
}

// Files in the drop-in directory are taken at start: the file name is the icon name, a clash is "Name 2", a file over
// the artwork limits is reported and stays, and a taken file leaves the directory.
func TestIconLibraryDropInAtStart(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	png := b64(t, tinyPNG)
	if rec := apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload("Logo", png, "")); rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	dropFile(t, a, "Logo.png", png)
	dropFile(t, a, "Camera.SVG", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"/>`))
	dropFile(t, a, "Wide.png", imgBytes("png", 1025, 4))
	dropFile(t, a, "Huge.svg", append(bigSVG(), bytes.Repeat([]byte(" "), 200)...))
	dropFile(t, a, "Photo.jpg", jpegBytes())
	dropFile(t, a, ".DS_Store", []byte("junk"))

	b, err := New(a.r().cfg, a.cfgPath, a.hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(iconNames(b), ","); got != "Camera,Logo,Logo 2" {
		t.Fatalf("icons %s", got)
	}
	for _, f := range []string{"Logo.png", "Camera.SVG"} {
		if _, err := os.Stat(filepath.Join(dropDir(a), f)); !os.IsNotExist(err) {
			t.Errorf("%s still in the drop-in directory: %v", f, err)
		}
	}
	for _, f := range []string{"Wide.png", "Huge.svg", "Photo.jpg", ".DS_Store"} {
		if _, err := os.Stat(filepath.Join(dropDir(a), f)); err != nil {
			t.Errorf("skipped %s was moved: %v", f, err)
		}
	}
	// Nothing drains the queue yet, so the start's rows are still in it.
	var note string
	for len(b.evq) > 0 {
		if e := <-b.evq; e.Kind == kindAPI && e.Why == "iconRescan" {
			note = e.Note + " " + e.Err
		}
	}
	for _, want := range []string{"Wide.png", "1024", "Huge.svg", "2 MB", "Photo.jpg"} {
		if !strings.Contains(note, want) {
			t.Errorf("start report %q lacks %q", note, want)
		}
	}
	if strings.Contains(note, ".DS_Store") {
		t.Errorf("a hidden file was reported: %q", note)
	}
}

func TestIconLibraryRescan(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	png := b64(t, tinyPNG)

	// Nothing in the directory, or no directory at all, adds nothing.
	if added, skipped := rescanBody(t, a, cy, ""); len(added)+len(skipped) != 0 {
		t.Fatalf("empty rescan %v %v", added, skipped)
	}

	dropFile(t, a, "Desk.png", png)
	dropFile(t, a, "desk.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	dropFile(t, a, "Wide.png", imgBytes("png", 1025, 4))
	if err := os.Mkdir(filepath.Join(dropDir(a), "nested.png"), 0o700); err != nil {
		t.Fatal(err)
	}
	// A link out of the directory is never read.
	outside := filepath.Join(t.TempDir(), "secret.png")
	os.WriteFile(outside, png, 0o600)
	if err := os.Symlink(outside, filepath.Join(dropDir(a), "Link.png")); err != nil {
		t.Fatal(err)
	}

	wantErr(t, apiSend(a, "POST", "/api/v1/icon-library/rescan", ana, ""), 403, "trusted_required")
	if len(a.icons.list()) != 0 {
		t.Fatal("a refused rescan added icons")
	}
	setSettingsPin(t, a, "9999")
	wantErr(t, apiSend(a, "POST", "/api/v1/icon-library/rescan", ana, `{"pin":"1111"}`), 403, "trusted_required")

	added, skipped := rescanBody(t, a, ana, `{"pin":"9999"}`)
	var names []string
	for _, ic := range added {
		names = append(names, ic.Name)
	}
	if strings.Join(names, ",") != "Desk,desk 2" {
		t.Fatalf("added %v", names)
	}
	var files []string
	for _, s := range skipped {
		if s.Reason == "" {
			t.Errorf("%s skipped with no reason", s.File)
		}
		files = append(files, s.File)
	}
	if strings.Join(files, ",") != "Link.png,Wide.png,nested.png" {
		t.Fatalf("skipped %v", skipped)
	}
	if strings.Join(iconNames(a), ",") != "Desk,desk 2" {
		t.Fatalf("library %v", iconNames(a))
	}
	if _, err := os.Lstat(filepath.Join(dropDir(a), "Link.png")); err != nil {
		t.Errorf("the link was removed: %v", err)
	}

	// A second rescan finds only what was skipped, so nothing is added twice.
	if added, _ := rescanBody(t, a, cy, ""); len(added) != 0 {
		t.Fatalf("second rescan added %v", added)
	}
	dropFile(t, a, "Desk.png", png)
	if added, _ := rescanBody(t, a, cy, ""); len(added) != 1 || added[0].Name != "Desk 3" {
		t.Fatalf("clash %v", added)
	}
}

// A drop-in directory that is itself a link is not read, since it would lead out of the library.
func TestIconLibraryDropInDirectoryLink(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	elsewhere := t.TempDir()
	os.WriteFile(filepath.Join(elsewhere, "Away.png"), b64(t, tinyPNG), 0o600)
	os.MkdirAll(filepath.Dir(dropDir(a)), 0o700)
	os.RemoveAll(dropDir(a))
	if err := os.Symlink(elsewhere, dropDir(a)); err != nil {
		t.Fatal(err)
	}
	added, skipped := rescanBody(t, a, cy, "")
	if len(added) != 0 || len(skipped) != 1 {
		t.Fatalf("added %v skipped %v", added, skipped)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "Away.png")); err != nil {
		t.Errorf("a file behind the link was touched: %v", err)
	}
}

func TestIconLibraryRescanNeedsUserKey(t *testing.T) {
	a := panelsApp(t)
	cfg := a.r().cfg
	cfg.APIClients[0].Scopes = append(cfg.APIClients[0].Scopes, "panels", "panelsmanage")
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/icon-library/rescan", botKey(t, a), `{"pin":"2468"}`), 403, "user_required")
}

// The drop-in name rule: the file name without its extension, cut to the name limit. Keep in step with libNameOf.
func TestIconDropName(t *testing.T) {
	for in, want := range map[string]string{
		"Logo.png":                       "Logo",
		"  Cam 1 .svg":                   "Cam 1",
		"a.b.png":                        "a.b",
		strings.Repeat("x", 50) + ".png": strings.Repeat("x", iconNameMax),
		".png":                           "",
	} {
		if got := dropName(in); got != want {
			t.Errorf("dropName(%q) = %q, want %q", in, got, want)
		}
	}
}

// A skipped file's name reaches the API answer, the log and the Communication Log with no control characters.
func TestIconLibraryDropInSkippedNameIsCleaned(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	dropFile(t, a, "bad\x1b[2Jname\n.txt", []byte("junk"))
	for len(a.evq) > 0 {
		<-a.evq
	}
	_, skipped := rescanBody(t, a, cy, "")
	if len(skipped) != 1 || skipped[0].File != "bad[2Jname.txt" {
		t.Fatalf("skipped %q", skipped)
	}
	var row string
	for len(a.evq) > 0 {
		if e := <-a.evq; e.Kind == kindAPI && e.Why == "iconRescan" {
			row = e.Note + " " + e.Err
		}
	}
	if !strings.Contains(row, "bad[2Jname.txt") || strings.ContainsAny(row, "\x1b\n") {
		t.Fatalf("Communication Log row %q", row)
	}
}

// An open error carries the raw name inside its text, so the reason is cleaned too.
func TestIconLibraryDropInSkippedReasonIsCleaned(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens an unreadable file")
	}
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	dropFile(t, a, "bad\x1bname\n.png", b64(t, tinyPNG))
	if err := os.Chmod(filepath.Join(dropDir(a), "bad\x1bname\n.png"), 0); err != nil {
		t.Fatal(err)
	}
	for len(a.evq) > 0 {
		<-a.evq
	}
	_, skipped := rescanBody(t, a, cy, "")
	if len(skipped) != 1 || strings.ContainsAny(skipped[0].Reason, "\x1b\n") {
		t.Fatalf("skipped %q", skipped)
	}
	for len(a.evq) > 0 {
		if e := <-a.evq; e.Kind == kindAPI && strings.ContainsAny(e.Note+e.Err, "\x1b\n") {
			t.Fatalf("Communication Log row %q %q", e.Note, e.Err)
		}
	}
}

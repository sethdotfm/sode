package app

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
)

// bigSVG is just under the 2 MB file limit, so 32 of them fit the 64 MB total and a 33rd does not.
func bigSVG() []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat(" ", panelArtBytes-100) + `</svg>`)
}

func TestPanelHoldsMoreThan40ImagesUnderTheTotal(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Many"}`)
	up := "/api/v1/panels/" + id + "/artwork"
	var last string
	for i := 0; i < 45; i++ {
		rec := apiSendRaw(a, "POST", up, ana, b64(t, tinyPNG))
		if rec.Code != 201 {
			t.Fatalf("image %d: %d %s", i, rec.Code, rec.Body.String())
		}
		last = apiBody(t, rec)["id"].(string)
	}
	if n := len(apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))["art"].([]any)); n != 45 {
		t.Errorf("panel lists %d artwork files", n)
	}
	if apiDo(a, "GET", up+"/"+last, ana, nil).Code != 200 {
		t.Error("the 45th image is not served")
	}
}

func TestPanelArtworkTotalRefusesTheUploadThatPassesIt(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Full"}`)
	up := "/api/v1/panels/" + id + "/artwork"
	for i := 0; i < 32; i++ {
		if rec := apiSendRaw(a, "POST", up, ana, bigSVG()); rec.Code != 201 {
			t.Fatalf("file %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := apiSendRaw(a, "POST", up, ana, bigSVG())
	wantErr(t, rec, 409, "conflict")
	if !strings.Contains(rec.Body.String(), "64 MB") {
		t.Errorf("error does not state the total: %s", rec.Body.String())
	}
	// Room is by bytes: a small image still fits.
	if rec := apiSendRaw(a, "POST", up, ana, b64(t, tinyPNG)); rec.Code != 201 {
		t.Errorf("small image after the big ones: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPushAndCopyTakeArtworkUpToTheTotalAndReportTheRest(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := makePanel(t, a, ana, `{"name":"Src"}`)
	dst := makePanel(t, a, ana, `{"name":"Dst"}`)
	add := func(id string, n int) (ids []string) {
		for i := 0; i < n; i++ {
			rec := apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, bigSVG())
			if rec.Code != 201 {
				t.Fatalf("art: %d %s", rec.Code, rec.Body.String())
			}
			ids = append(ids, apiBody(t, rec)["id"].(string))
		}
		return
	}
	mine := add(src, 20)
	add(dst, 16)
	code, body := pushTo(a, ana, src, dst)
	if code != 200 {
		t.Fatalf("push: %d %v", code, body)
	}
	skipped, _ := body["pushed"].([]any)[0].(map[string]any)["artSkipped"].([]any)
	if len(skipped) != 4 {
		t.Fatalf("skipped %v, want the last 4", skipped)
	}
	for i, id := range mine {
		got := apiDo(a, "GET", "/api/v1/panels/"+dst+"/artwork/"+id, ana, nil).Code
		want := 404
		if i < 16 {
			want = 200
		}
		if got != want {
			t.Errorf("artwork %d on the target: %d, want %d", i, got, want)
		}
	}
	// A copy goes to an empty panel, so all 20 fit and nothing is reported.
	cp := apiBody(t, apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", ana, ""))
	if cp["artSkipped"] != nil || len(cp["art"].([]any)) != 20 {
		t.Errorf("copy: %v", cp["artSkipped"])
	}
}

// fakeFull lists one artwork record on id that claims all but room bytes of the total, so the cap is reached
// without writing 64 MB.
func fakeFull(t *testing.T, a *App, id string, room int) {
	t.Helper()
	if !a.panels.putArtAs(id, PanelArt{ID: "a-ffffffff", Type: "image/png"}, []byte("x")) {
		t.Fatal("seed")
	}
	s := a.panels
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.by[id]
	p.Art = slices.Clone(p.Art)
	p.Art[len(p.Art)-1].Size = panelArtTotal - room
	s.by[id] = p
}

func TestPanelArtPutAsKeepsTheTotal(t *testing.T) {
	a := panelsApp(t)
	id := makePanel(t, a, userKey(t, a, "Ana"), `{"name":"Full"}`)
	fakeFull(t, a, id, 4)
	if a.panels.putArtAs(id, PanelArt{ID: "a-00000001", Type: "image/png", Size: 1}, []byte("12345678")) {
		t.Fatal("putArtAs passed the 64 MB total")
	}
	// The size is the bytes stored, never the record's claim.
	if !a.panels.putArtAs(id, PanelArt{ID: "a-00000002", Type: "image/png", Size: 99}, []byte("1234")) {
		t.Fatal("an image that fits was refused")
	}
	if p, _ := a.panels.get(id); artBytes(p) != panelArtTotal {
		t.Fatalf("total %d", artBytes(p))
	}
}

// Two writers copy onto one target at once; each alone fits, both together do not.
func TestPanelArtConcurrentCopiesKeepTheTotal(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	dst := makePanel(t, a, ana, `{"name":"Dst"}`)
	fakeFull(t, a, dst, 60)
	var srcs []Panel
	for s := range 2 {
		id := makePanel(t, a, ana, `{"name":"Src"}`)
		for i := range 60 {
			if !a.panels.putArtAs(id, PanelArt{ID: fmt.Sprintf("a-%02x%06x", s, i), Type: "image/png", Size: 1}, []byte("x")) {
				t.Fatal("seed source")
			}
		}
		p, _ := a.panels.get(id)
		srcs = append(srcs, p)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	left := make([][]string, 2)
	for i, src := range srcs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			left[i] = a.panels.copyArt(src, dst)
		}()
	}
	close(start)
	wg.Wait()
	p, _ := a.panels.get(dst)
	if artBytes(p) > panelArtTotal {
		t.Fatalf("total passes %d by %d", panelArtTotal, artBytes(p)-panelArtTotal)
	}
	if n := len(left[0]) + len(left[1]); n != 60 {
		t.Errorf("%d reported as not copied, want 60", n)
	}
}

// Artwork stored before the 2 MB and 1024 by 1024 limits still loads and serves.
func TestPanelArtOldOversizeStillServes(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Old"}`)
	wide := imgBytes("png", 2000, 2000)
	big := append(bigSVG(), bytes.Repeat([]byte(" "), 1<<20)...)
	if !a.panels.putArtAs(id, PanelArt{ID: "a-0000000a", Type: "image/png", Size: len(wide)}, wide) ||
		!a.panels.putArtAs(id, PanelArt{ID: "a-0000000b", Type: "image/svg+xml", Size: len(big)}, big) {
		t.Fatal("seed")
	}
	a.panels = loadPanels(a.cfgPath)
	for art, want := range map[string][]byte{"a-0000000a": wide, "a-0000000b": big} {
		rec := apiDo(a, "GET", "/api/v1/panels/"+id+"/artwork/"+art, ana, nil)
		if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), want) {
			t.Errorf("%s: %d, %d bytes", art, rec.Code, rec.Body.Len())
		}
	}
}

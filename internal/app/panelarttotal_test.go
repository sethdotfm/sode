package app

import (
	"strings"
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

package app

import "testing"

func TestGridCopyAndPushCarryItemArtwork(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := makeGridPanel(t, a, ana, "Src")
	rec := apiSendRaw(a, "POST", "/api/v1/panels/"+src+"/artwork", ana, b64(t, tinyPNG))
	if rec.Code != 201 {
		t.Fatalf("art: %d %s", rec.Code, rec.Body.String())
	}
	art := apiBody(t, rec)["id"].(string)
	body := `{"name":"Src","sources":{"members":[1],"items":{"1":{"art":"` + art + `"}}}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+src, ana, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	rec = apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", ana, `{}`)
	if rec.Code != 201 {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body.String())
	}
	cp := apiBody(t, rec)["id"].(string)
	dst := makeGridPanel(t, a, ana, "Dst")
	pushTo(a, ana, src, dst)
	for _, id := range []string{cp, dst} {
		if rec := apiDo(a, "GET", "/api/v1/panels/"+id+"/artwork/"+art, ana, nil); rec.Code != 200 {
			t.Errorf("panel %s lacks the artwork: %d", id, rec.Code)
		}
		if s := gridEdgeJSON(t, readPanel(t, a, ana, id), "sources"); s != `{"items":{"1":{"art":"`+art+`"}},"members":[1]}` {
			t.Errorf("panel %s sources %s", id, s)
		}
	}
}

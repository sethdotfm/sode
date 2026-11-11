package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"
)

// animatedGIF has two flat frames, red then blue, that loop every 100 ms.
func animatedGIF(t *testing.T) []byte {
	t.Helper()
	pal := color.Palette{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}}
	anim := &gif.GIF{LoopCount: 0}
	for i := range 2 {
		f := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
		for p := range f.Pix {
			f.Pix[p] = uint8(i)
		}
		anim.Image = append(anim.Image, f)
		anim.Delay = append(anim.Delay, 10)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, anim); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// The editor's upload path: a PNG over 1024 is stored at most 1024 on a side, an animated GIF is stored byte for
// byte and still animates, and an oversize GIF is refused before anything is sent.
func TestChromeEditorArtUpload(t *testing.T) {
	b, a, ana, id := editorTabPrep(t, `{"name":"Art","rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"take"}]}`, nil)
	sq := `document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]')`
	b.eval(`(() => { const s = ` + sq + `; const r = s.getBoundingClientRect(); s.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); })()`)
	b.waitFor("the Cell tab", `document.querySelector('#ed-side .pb-tab[aria-selected="true"]')?.textContent === 'Cell'`)
	b.eval(`window.choose = (blob, name) => { const dt = new DataTransfer(); dt.items.add(new File([blob], name, { type: blob.type })); const f = document.querySelector('#panel-ed-cell input[type=file]'); f.files = dt.files; f.dispatchEvent(new Event('change')); }; true`)
	b.eval(`window.note = () => document.querySelector('#panel-ed-cell .sode__field:has(.sode-side__art) > .sode__hint'); true`)
	b.eval(`window.fromB64 = (s, type) => new Blob([Uint8Array.from(atob(s), (c) => c.charCodeAt(0))], { type }); true`)

	stored := func() []PanelArt {
		var p Panel
		if err := json.Unmarshal(apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil).Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p.Art
	}
	artBytes := func(art string) []byte {
		return apiDo(a, "GET", "/api/v1/panels/"+id+"/artwork/"+art, ana, nil).Body.Bytes()
	}

	// 2000 by 1500 with a band of noise: over 2 MB before the resize, under it after.
	b.eval(`(async () => { const c = document.createElement('canvas'); c.width = 2000; c.height = 1500; const x = c.getContext('2d'); const d = x.createImageData(2000, 1500); for (let i = 0; i < d.data.length; i++) d.data[i] = i % 4 === 3 ? 255 : i < 2000 * 700 * 4 ? (Math.random() * 256) | 0 : 128; x.putImageData(d, 0, 0); const blob = await new Promise((r) => c.toBlob(r, 'image/png')); window.bigPNG = blob.size; choose(blob, 'big.png'); return true; })()`)
	b.waitFor("the PNG is uploaded and shown", `!!`+sq+`.querySelector('img.sode-panel__art-a') && note().textContent === ''`)
	if before := b.eval(`window.bigPNG`).(float64); before <= 2<<20 {
		t.Fatalf("the test PNG is %.0f bytes; it must be over the 2 MB cap to show the resize", before)
	}
	arts := stored()
	if len(arts) != 1 {
		t.Fatalf("stored artwork: %+v", arts)
	}
	cfg, kind, err := image.DecodeConfig(bytes.NewReader(artBytes(arts[0].ID)))
	if err != nil || kind != "png" || cfg.Width != 1024 || cfg.Height != 768 {
		t.Fatalf("the PNG is stored as %s %dx%d (%v), want png 1024x768", kind, cfg.Width, cfg.Height, err)
	}
	t.Logf("a 2000x1500 PNG of %.0f bytes is stored as 1024x768, %d bytes", b.eval(`window.bigPNG`).(float64), arts[0].Size)

	anim := animatedGIF(t)
	b.eval(fmt.Sprintf(`choose(fromB64(%q, 'image/gif'), 'anim.gif'); true`, base64.StdEncoding.EncodeToString(anim)))
	b.waitFor("the GIF is uploaded", `(() => { const i = `+sq+`.querySelector('img.sode-panel__art-a'); return !!i && i.complete && i.naturalWidth === 64; })()`)
	arts = stored()
	if len(arts) != 2 {
		t.Fatalf("stored artwork: %+v", arts)
	}
	got := artBytes(arts[1].ID)
	if !bytes.Equal(got, anim) {
		t.Fatalf("the GIF was changed on the way: %d bytes sent, %d stored", len(anim), len(got))
	}
	if g, err := gif.DecodeAll(bytes.NewReader(got)); err != nil || len(g.Image) != 2 {
		t.Fatalf("the stored GIF lost its frames: %v", err)
	}
	// In headless Chrome a drawImage of the img stays on the first frame. A screenshot paints a new frame each time.
	box := b.eval(`(() => { const r = ` + sq + `.querySelector('img.sode-panel__art-a').getBoundingClientRect(); return [r.x + r.width / 2, r.y + r.height / 2]; })()`).([]any)
	seen := map[string]bool{}
	for deadline := time.Now().Add(5 * time.Second); len(seen) < 2 && time.Now().Before(deadline); time.Sleep(40 * time.Millisecond) {
		var shot struct{ Data []byte }
		_ = json.Unmarshal(b.call("Page.captureScreenshot", map[string]any{"format": "png", "clip": map[string]any{"x": box[0], "y": box[1], "width": 1, "height": 1, "scale": 1}}), &shot)
		img, _, err := image.Decode(bytes.NewReader(shot.Data))
		if err != nil {
			t.Fatal(err)
		}
		r, _, bl, _ := img.At(0, 0).RGBA()
		seen[map[bool]string{true: "red", false: "blue"}[r > bl]] = true
	}
	if len(seen) < 2 {
		t.Errorf("the GIF did not animate in the square: only %v was drawn", seen)
	}

	b.eval(fmt.Sprintf(`choose(fromB64(%q, 'image/gif'), 'wide.gif'); true`, base64.StdEncoding.EncodeToString(imgBytes("gif", 2000, 10))))
	b.waitFor("an oversize GIF is refused with how to fix it", `/Resize it and upload again/.test(note().textContent)`)
	if arts = stored(); len(arts) != 2 {
		t.Fatalf("the oversize GIF was sent: %+v", arts)
	}
	b.check()
}

package render

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Premultiplied invariant: image.RGBA documents RGB <= A. Violations
// silently corrupt every stdlib composite (draw.Draw/Over) and quantizer
// downstream, producing near-black garbage from vivid colors.
func assertPremultiplied(t *testing.T, img *image.RGBA, what string) {
	t.Helper()
	bad := 0
	for i := 0; i < len(img.Pix); i += 4 {
		r, g, b2, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
		if r > a || g > a || b2 > a {
			bad++
		}
	}
	if bad > 0 {
		t.Fatalf("%s: %d pixels violate RGB<=A", what, bad)
	}
}

func TestPremultipliedInvariant(t *testing.T) {
	wm := filepath.Join(t.TempDir(), "wm.png")
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 0xff, A: 0xff})
		}
	}
	f, err := os.Create(wm)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	for _, ft := range []string{"null", "window", "floating", "solid"} {
		title := "T"
		cfg := testConfig(ft, &title)
		term := NewTerminal(20, 5)
		term.Feed("\x1b[31mred\x1b[0m plain")
		snap := term.Snapshot()
		assertPremultiplied(t, DrawFrame(snap, term.CurX, term.CurY, true, cfg), ft+" plain")
		cfg.Watermark.ImagePath = &wm
		assertPremultiplied(t, DrawFrame(snap, term.CurX, term.CurY, true, cfg), ft+" watermark")
	}
}

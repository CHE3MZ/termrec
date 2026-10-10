package render

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeRedPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 0xff, A: 0xff})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func hasRedPixel(img *image.RGBA) bool {
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 200 && img.Pix[i+1] < 80 && img.Pix[i+2] < 80 && img.Pix[i+3] > 128 {
			return true
		}
	}
	return false
}

// Watermark must apply to every frame type, including null (regression:
// null returns skipped the overlay entirely).
func TestWatermarkAllFrameTypes(t *testing.T) {
	wm := filepath.Join(t.TempDir(), "wm.png")
	writeRedPNG(t, wm)
	for _, ft := range []string{"null", "window", "floating", "solid"} {
		title := "T"
		cfg := testConfig(ft, &title)
		cfg.Watermark.ImagePath = &wm
		term := NewTerminal(20, 5)
		term.Feed("hi")
		img := DrawFrame(term.Snapshot(), term.CurX, term.CurY, true, cfg)
		if !hasRedPixel(img) {
			t.Fatalf("%s: watermark missing", ft)
		}
	}
}

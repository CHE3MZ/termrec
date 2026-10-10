package render

import (
	"image"
	"image/color"
	"testing"
)

// The gaussian must spread wide (3 sigma coverage), unlike the old tight blur.
func TestGaussianBlurWidth(t *testing.T) {
	W, H := 200, 40
	sil := image.NewAlpha(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		for x := 50; x < 150; x++ {
			sil.SetAlpha(x, y, color.Alpha{A: 0xff})
		}
	}
	out := gaussianBlurAlpha(sil, 14)
	run := 0
	for x := 49; x >= 0; x-- {
		if out.AlphaAt(x, H/2).A > 4 {
			run++
		} else {
			break
		}
	}
	if run < 20 {
		t.Fatalf("blur too narrow: %d", run)
	}
}

// The cached shadow layer must carry a real glow falloff, not a hard edge.
func TestShadowLayerFalloff(t *testing.T) {
	l := shadowLayer(640, 400, 12, 14)
	b := l.Bounds()
	y := b.Dy() / 2
	edge := 42 + 640
	run := 0
	for x := edge; x < b.Dx(); x++ {
		if l.RGBAAt(x, y).A > 4 {
			run++
		} else {
			break
		}
	}
	if run < 20 {
		t.Fatalf("layer glow too narrow: %d", run)
	}
}

// Semi-transparent coverage must survive GIF quantization as stipple
// (1-bit alpha), not collapse to fully transparent.
func TestAlphaStipple(t *testing.T) {
	W, H := 64, 8
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			a := uint8(x * 255 / (W - 1))
			img.SetRGBA(x, y, color.RGBA{R: 0xB3, G: 0xB3, B: 0xB3, A: a})
		}
	}
	pal := color.Palette{color.RGBA{}, color.RGBA{0xB3, 0xB3, 0xB3, 0xff}, color.Black, color.White}
	pm := ditherFS(img, pal, true)
	opaque := 0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if pm.ColorIndexAt(x, y) != 0 {
				opaque++
			}
		}
	}
	if opaque == 0 {
		t.Fatalf("no stipple at all")
	}
	right := 0
	for y := 0; y < H; y++ {
		for x := W / 2; x < W; x++ {
			if pm.ColorIndexAt(x, y) != 0 {
				right++
			}
		}
	}
	if right < W*H/4 {
		t.Fatalf("right half should be mostly opaque, got %d", right)
	}
}

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

// Faint dark coverage (shadow tail: alpha < 24, straight color dark) must
// be absorbed to hard transparent, never diffused: diffusion would
// concentrate it into visible stipple dots far from the card. Bright faint
// pixels (text AA: straight color light) must keep diffusing so glyph
// edges stay smooth.
func TestFaintDarkAbsorbed(t *testing.T) {
	mkGrad := func(dark bool) *image.RGBA {
		W, H := 48, 8
		img := image.NewRGBA(image.Rect(0, 0, W, H))
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				a := uint8(x * 20 / (W - 1)) // 0..20, all below the absorb floor
				if dark {
					img.SetRGBA(x, y, color.RGBA{R: 0, G: 0, B: 0, A: a})
				} else {
					img.SetRGBA(x, y, color.RGBA{R: a, G: a, B: a, A: a})
				}
			}
		}
		return img
	}
	pal := color.Palette{color.RGBA{}, color.RGBA{0, 0, 0, 0xff}, color.White}
	dark := ditherFS(mkGrad(true), pal, true)
	for y := 0; y < 8; y++ {
		for x := 0; x < 48; x++ {
			if dark.ColorIndexAt(x, y) != 0 {
				t.Fatalf("dark faint dot at %d,%d: coverage must be absorbed", x, y)
			}
		}
	}
	light := ditherFS(mkGrad(false), pal, true)
	opaque := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 48; x++ {
			if light.ColorIndexAt(x, y) != 0 {
				opaque++
			}
		}
	}
	if opaque == 0 {
		t.Fatalf("bright faint coverage must still diffuse (text AA)")
	}
}

// Dark semi pixels that diffusion resolves to opaque must become pure
// black, never a dark-gray tone: black vanishes on dark pages and reads
// as shadow density on light pages, while dark gray would fringe.
func TestDarkSemiForcedBlack(t *testing.T) {
	W, H := 48, 8
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			a := uint8(100 + x*120/(W-1)) // 100..220
			// straight dark gray (#0e-ish, like card-edge AA): premultiplied
			v := uint8(uint16(14) * uint16(a) / 255)
			img.SetRGBA(x, y, color.RGBA{R: v, G: v, B: v, A: a})
		}
	}
	decoy := color.RGBA{0x0e, 0x0e, 0x0e, 0xff}
	pal := color.Palette{color.RGBA{}, color.RGBA{0, 0, 0, 0xff}, decoy, color.White}
	pm := ditherFS(img, pal, true)
	opaque := 0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			switch idx := pm.ColorIndexAt(x, y); idx {
			case 0:
				// transparent, fine
			case 1:
				opaque++
			default:
				t.Fatalf("dark semi pixel at %d,%d -> index %d, want black(1)", x, y, idx)
			}
		}
	}
	if opaque == 0 {
		t.Fatalf("expected some opaque black dots")
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

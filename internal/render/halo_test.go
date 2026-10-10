package render

import (
	"testing"
)

// White page + smooth shadow ramp (ref parity): outside the card the frame
// must be fully opaque, with pure white margins/corners and a gray glow
// band hugging the card — never transparency stipple, never a hard halo
// edge (regression: matteGlowOutside's opaque white ring, then the
// transparent-stipple halo that read as a "white thingy" on dark
// backgrounds; ref GIFs are all a=255 with a smooth #fff->#dadada ramp).
func TestShadowRampOnWhitePage(t *testing.T) {
	for _, ft := range []string{"window", "floating", "solid"} {
		title := "T"
		cfg := testConfig(ft, &title)
		term := NewTerminal(20, 5)
		term.Feed("hi")
		img := DrawFrame(term.Snapshot(), term.CurX, term.CurY, true, cfg)
		b := img.Bounds()
		W, H := b.Dx(), b.Dy()
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0xff {
				t.Fatalf("%s: pixel %d not opaque (alpha %d)", ft, i/4, img.Pix[i])
			}
		}
		for _, p := range [][2]int{{0, 0}, {W - 1, 0}, {0, H - 1}, {W - 1, H - 1}} {
			c := img.RGBAAt(p[0], p[1])
			if c.R != 0xff || c.G != 0xff || c.B != 0xff {
				t.Fatalf("%s: corner %v = #%02x%02x%02x, want white page", ft, p, c.R, c.G, c.B)
			}
		}
		cw, ch := cellSize(cfg)
		pad, titleH, _ := framePadTitle(cfg)
		cardW := 20*cw + pad*2
		cardH := 5*ch + pad*2 + titleH
		m := parseMarginPx(cfg)
		glow := 0
		if shadowEnabled(cfg) {
			glow = shadowGlow()
		}
		ox, oy := m+glow, m+glow
		// glow band: the margin strip just outside the card must dip below
		// near-white (the shadow), then recover to white at the canvas edge.
		y := oy + cardH/2
		darkest := uint8(0xff)
		for x := 0; x < ox; x++ {
			c := img.RGBAAt(x, y)
			v := c.R
			if c.G < v {
				v = c.G
			}
			if c.B < v {
				v = c.B
			}
			if v < darkest {
				darkest = v
			}
		}
		if darkest > 0xE8 {
			t.Fatalf("%s: no shadow band outside card (darkest #%02x%02x..)", ft, darkest, darkest)
		}
		// body padding strip (always empty) must be exact frame bg #1D1D1D.
		bg := img.RGBAAt(ox+cardW/2, oy+cardH-pad-2)
		if diff(bg.R, 0x1d) > 4 || diff(bg.G, 0x1d) > 4 || diff(bg.B, 0x1d) > 4 {
			t.Fatalf("%s: body pad #%02x%02x%02x want #1d1d1d", ft, bg.R, bg.G, bg.B)
		}
		assertPremultiplied(t, img, ft+" page")
	}
}

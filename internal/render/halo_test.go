package render

import (
	"image"
	"testing"
)

// Outside the card, the shadow glow must stay semi-transparent (it is
// stippled by the GIF dither). An opaque near-white ring there means the
// glow was flattened/matted and will read as a solid halo on dark
// backgrounds (regression: matteGlowOutside).
func TestNoOpaqueHaloOutsideCard(t *testing.T) {
	for _, ft := range []string{"window", "floating", "solid"} {
		title := "T"
		cfg := testConfig(ft, &title)
		term := NewTerminal(20, 5)
		term.Feed("hi")
		img := DrawFrame(term.Snapshot(), term.CurX, term.CurY, true, cfg)
		cw, ch := cellSize(cfg)
		pad, titleH, _ := framePadTitle(cfg)
		cardW := 20*cw + pad*2
		cardH := 5*ch + pad*2 + titleH
		m := parseMarginPx(cfg)
		glow := 0
		if shadowEnabled(cfg) {
			glow = shadowGlow()
		}
		card := image.Rect(m+glow, m+glow, m+glow+cardW, m+glow+cardH)
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if image.Pt(x, y).In(card) {
					continue
				}
				c := img.RGBAAt(x, y)
				if c.A > 128 && c.R > 0xE8 && c.G > 0xE8 && c.B > 0xE8 {
					t.Fatalf("%s: opaque near-white halo pixel at %d,%d: %+v", ft, x, y, c)
				}
			}
		}
		assertPremultiplied(t, img, ft+" halo")
	}
}

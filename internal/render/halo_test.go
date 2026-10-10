package render

import (
	"path/filepath"
	"testing"

	"github.com/CHE3MZ/termrec/internal/recording"
)

// No light halo outside the card: frames are transparent by default and
// the shadow glow is dark alpha, so outside the card there must be no
// opaque near-white pixels (regression: matteGlowOutside's opaque white
// ring, and the light-gray stipple halo that read as a "white thingy" on
// dark backgrounds). Corners stay fully transparent, a real semi shadow
// band hugs the card, and the body stays exact #1D1D1D.
func TestNoOpaqueHaloOutsideCard(t *testing.T) {
	for _, ft := range []string{"window", "floating", "solid"} {
		title := "T"
		cfg := testConfig(ft, &title)
		term := NewTerminal(20, 5)
		term.Feed("hi")
		img := DrawFrame(term.Snapshot(), term.CurX, term.CurY, true, cfg)
		b := img.Bounds()
		W, H := b.Dx(), b.Dy()
		for _, p := range [][2]int{{0, 0}, {W - 1, 0}, {0, H - 1}, {W - 1, H - 1}} {
			if c := img.RGBAAt(p[0], p[1]); c.A != 0 {
				t.Fatalf("%s: corner %v = %+v, want fully transparent", ft, p, c)
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
		semi := 0
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				// inside card rect -> chrome, skip (checked by other tests)
				if x >= ox && x < ox+cardW && y >= oy && y < oy+cardH {
					continue
				}
				c := img.RGBAAt(x, y)
				if c.A > 128 && c.R > 0xE8 && c.G > 0xE8 && c.B > 0xE8 {
					t.Fatalf("%s: opaque near-white halo pixel at %d,%d: %+v", ft, x, y, c)
				}
				if c.A != 0 && c.A != 0xff {
					semi++
				}
			}
		}
		if semi == 0 {
			t.Fatalf("%s: no semi shadow pixels outside card (glow missing)", ft)
		}
		// body padding strip (always empty) must be exact frame bg #1D1D1D.
		bg := img.RGBAAt(ox+cardW/2, oy+cardH-pad-2)
		if diff(bg.R, 0x1d) > 4 || diff(bg.G, 0x1d) > 4 || diff(bg.B, 0x1d) > 4 {
			t.Fatalf("%s: body pad #%02x%02x%02x want #1d1d1d", ft, bg.R, bg.G, bg.B)
		}
		assertPremultiplied(t, img, ft+" transparent")
	}
}

// Far field must be perfectly clean in the encoded GIF: beyond the card +
// glow band, every pixel must be the transparent index (no scattered
// stipple dust — faint dark coverage is absorbed, not diffused, by the
// ditherer, so error can never concentrate into dots far from the card).
func TestFarFieldCleanInGIF(t *testing.T) {
	title := "T"
	cfg := testConfig("floating", &title)
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "hi\r\n"},
		{Delay: 200, Content: "yo\r\n"},
	}}
	path := filepath.Join(t.TempDir(), "far.gif")
	out, err := Render(rec, path, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, out)
	cw, ch := cellSize(cfg)
	pad, titleH, _ := framePadTitle(cfg)
	cardW := 20*cw + pad*2
	cardH := 5*ch + pad*2 + titleH
	m := parseMarginPx(cfg)
	ox, oy := m+shadowGlow(), m+shadowGlow()
	const clear = 14
	for fi, pm := range g.Image {
		W, H := pm.Bounds().Dx(), pm.Bounds().Dy()
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				if x >= ox-clear && x < ox+cardW+clear && y >= oy-clear && y < oy+cardH+clear {
					continue
				}
				if idx := pm.ColorIndexAt(x, y); idx != 0 {
					t.Fatalf("frame %d far-field pixel at %d,%d = index %d, want transparent", fi, x, y, idx)
				}
			}
		}
	}
}

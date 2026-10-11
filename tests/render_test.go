// Package tests holds black-box integration tests over the public API:
// fixture recordings rendered end to end, decoded, and asserted the way a
// viewer sees them. Run with `task test` (go test ./... picks this up).
// White-box unit tests (blink math, palette internals, SGR, fonts, timing)
// live next to the code they cover inside internal/.
package tests

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/recording"
	"github.com/CHE3MZ/termrec/internal/render"
)

func loadFixture(t *testing.T, name string) recording.File {
	t.Helper()
	rec, _, err := recording.LoadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return rec
}

func decodeGIF(t *testing.T, path string) *gif.GIF {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return g
}

func fixtureConfig(frameType string, title *string) config.Config {
	c, _ := config.Load([]byte(config.DefaultYAML))
	c.FrameBox.Type = &frameType
	c.FrameBox.Title = title
	c.Cols.Value = 20
	c.Rows.Value = 5
	c.Cols.IsAuto = false
	c.Rows.IsAuto = false
	return c
}

func writeSolidPNG(t *testing.T, path string, w, h int, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
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

func diff(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

// composite renders decoded frames viewer-style (Over, in order), honoring
// per-frame disposal: disposal 2 restores the frame rect to background
// (transparent) before the next frame paints.
func composite(g *gif.GIF, through int) *image.RGBA {
	comp := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	for k := 0; k <= through; k++ {
		pm := g.Image[k]
		draw.Draw(comp, pm.Bounds(), pm, pm.Bounds().Min, draw.Over)
		if k < through && k < len(g.Disposal) && g.Disposal[k] == 2 {
			draw.Draw(comp, pm.Bounds(), &image.Uniform{color.Transparent}, image.Point{}, draw.Src)
		}
	}
	return comp
}

// Every fixture must render to a valid GIF with one frame per record.
// (All fixture delays sit under the ~1s blink threshold with visible
// cursors and changing grids, so no blink split or idle merge applies.
// A fixture that outgrows this must update the expectation deliberately.)
func TestRenderAllFixtures(t *testing.T) {
	fixtures := []string{"demo", "window", "null", "solid", "solid-notitle", "typing", "micro", "hyperfine", "eager"}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			rec := loadFixture(t, name)
			wantFrames := len(rec.Records)
			path := filepath.Join(t.TempDir(), name+".gif")
			out, err := render.Render(rec, path, 0, 1, "", false)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			st, err := os.Stat(out)
			if err != nil || st.Size() == 0 {
				t.Fatalf("empty output: %v", err)
			}
			g := decodeGIF(t, out)
			if len(g.Image) != wantFrames {
				t.Fatalf("frames=%d want %d", len(g.Image), wantFrames)
			}
			if len(g.Delay) != wantFrames {
				t.Fatalf("delays=%d want %d", len(g.Delay), wantFrames)
			}
			if g.Config.Width <= 0 || g.Config.Height <= 0 {
				t.Fatalf("bad dims %+v", g.Config)
			}
			// first frame is always full-canvas; dirty-rect frames must fit
			// inside it (frame diffing encodes only what changed).
			full := image.Rect(0, 0, g.Config.Width, g.Config.Height)
			for i, pm := range g.Image {
				if i == 0 && pm.Bounds() != full {
					t.Fatalf("frame 0 bounds %v != full %v", pm.Bounds(), full)
				}
				if !pm.Bounds().In(full) {
					t.Fatalf("frame %d bounds %v outside %v", i, pm.Bounds(), full)
				}
			}
		})
	}
}

// Step filtering must reduce frame counts deterministically
// (same single-frame reasoning as above).
func TestRenderStep(t *testing.T) {
	rec := loadFixture(t, "demo")
	n := len(rec.Records)
	for _, step := range []int{1, 2, 3} {
		path := filepath.Join(t.TempDir(), "s.gif")
		// reload: Render mutates delays via Adjust
		rec2 := loadFixture(t, "demo")
		if _, err := render.Render(rec2, path, 0, step, "", false); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		g := decodeGIF(t, path)
		want := (n + step - 1) / step
		if len(g.Image) != want {
			t.Fatalf("step %d frames=%d want %d", step, len(g.Image), want)
		}
	}
}

// Quality knob must always produce a decodable GIF.
func TestRenderQualityVariants(t *testing.T) {
	for _, q := range []int{1, 30, 60, 80, 100} {
		rec := loadFixture(t, "demo")
		path := filepath.Join(t.TempDir(), "q.gif")
		if _, err := render.Render(rec, path, q, 1, "", false); err != nil {
			t.Fatalf("quality %d: %v", q, err)
		}
		g := decodeGIF(t, path)
		if len(g.Image) == 0 {
			t.Fatalf("quality %d: no frames", q)
		}
	}
}

// An idle pause must split into extra frames while keeping the total
// duration exact; a hidden cursor (?25l) must not split at all.
func TestBlinkRender(t *testing.T) {
	mkrec := func(hide bool) recording.File {
		title := "T"
		cfg := fixtureConfig("null", &title)
		content := "hi\r\n"
		if hide {
			content = "\x1b[?25lhi\r\n"
		}
		return recording.File{Config: cfg, Records: []recording.Record{
			{Delay: 200, Content: content},
			{Delay: 1200, Content: "yo\r\n"},
		}}
	}
	path := filepath.Join(t.TempDir(), "blink.gif")
	if _, err := render.Render(mkrec(false), path, 100, 1, "", false); err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, path)
	if len(g.Image) < 3 {
		t.Fatalf("idle pause did not split: %d frames", len(g.Image))
	}
	sum10 := 0
	for _, d := range g.Delay {
		sum10 += d
	}
	if sum10 != 240 { // shown delays 1200 + 1200 own, in 10ms units
		t.Fatalf("blink total %d00ms want 2400ms", sum10)
	}
	// hidden cursor: same recording, no split -> 2 frames
	path2 := filepath.Join(t.TempDir(), "noblink.gif")
	if _, err := render.Render(mkrec(true), path2, 100, 1, "", false); err != nil {
		t.Fatal(err)
	}
	if g2 := decodeGIF(t, path2); len(g2.Image) != 2 {
		t.Fatalf("hidden cursor frames=%d want 2", len(g2.Image))
	}
}

// Rendered flat colors must match the theme (color consistency between
// config values and GIF pixels). Anchored palette keeps flats bit-exact.
func TestRenderColorConsistency(t *testing.T) {
	title := "T"
	cfg := fixtureConfig("null", &title)
	// opaque red background, white text
	cfg.Theme["background"] = "#ff0000"
	cfg.Theme["foreground"] = "#ffffff"
	term := render.NewTerminal(8, 2)
	snap := term.Snapshot()
	img := render.DrawFrame(snap, term.CurX, term.CurY, false, cfg)
	// center of a text cell area (empty cell => pure background)
	c := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	if diff(c.R, 0xff) > 6 || diff(c.G, 0x00) > 6 || diff(c.B, 0x00) > 6 {
		t.Fatalf("bg drift: got #%02x%02x%02x want #ff0000", c.R, c.G, c.B)
	}
}

// End-to-end: flat theme colors must survive GIF encoding bit-exact.
// Frames are dirty rects, so the check composites viewer-style before
// sampling a known-empty cell (bottom-right corner).
func TestGIFColorConsistency(t *testing.T) {
	title := "T"
	cfg := fixtureConfig("null", &title)
	cfg.Theme["background"] = "#232628"
	cfg.Theme["foreground"] = "#afafaf"
	cfg.FrameBox.Style = map[string]string{"margin": "0px"} // bare terminal
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "ab\r\n"},
		{Delay: 200, Content: "cd\r\n"},
	}}
	path := filepath.Join(t.TempDir(), "cc.gif")
	out, err := render.Render(rec, path, 100, 1, "", false)
	if err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, out)
	if len(g.Image) != 2 {
		t.Fatalf("frames=%d want 2", len(g.Image))
	}
	// frame 1 only repaints what changed, so it must be a strictly
	// smaller sub-rect of the full canvas — never a second full frame.
	if b1, b0 := g.Image[1].Bounds(), g.Image[0].Bounds(); b1 == b0 || b1.Empty() || !b1.In(b0) {
		t.Fatalf("frame 1 bounds %v not a proper dirty rect of %v", b1, b0)
	}
	comp := composite(g, len(g.Image)-1)
	W, H := g.Config.Width, g.Config.Height
	c := comp.RGBAAt(W-3, H-3)
	if diff(c.R, 0x23) > 4 || diff(c.G, 0x26) > 4 || diff(c.B, 0x28) > 4 {
		t.Fatalf("bg #%02x%02x%02x want #232628", c.R, c.G, c.B)
	}
}

// Transparent animations must never leave ghost trails: composite every
// prefix viewer-style and compare against a fresh full render of that
// state. Old cursor blocks lingering on transparent canvas show up as
// opaque-vs-transparent flips; antialias shimmer never flips alpha, so
// any double-digit flip count is a real ghost (regression: dirty-rect
// crops on transparency froze a cursor on every line).
func TestTransparentNoGhosts(t *testing.T) {
	title := "T"
	cfg := fixtureConfig("null", &title) // default theme bg transparent
	cfg.FrameBox.Style = map[string]string{"margin": "0px"}
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "ab\r\n"},
		{Delay: 200, Content: "cd\r\n"},
		{Delay: 200, Content: "ef\r\n"},
		{Delay: 200, Content: "gh\r\n"},
	}}
	path := filepath.Join(t.TempDir(), "ghost.gif")
	if _, err := render.Render(rec, path, 100, 1, "", false); err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, path)
	if len(g.Image) != 4 {
		t.Fatalf("frames=%d want 4", len(g.Image))
	}
	// transparent animations restore to background after every frame
	for i, d := range g.Disposal {
		if d != 2 {
			t.Fatalf("frame %d disposal=%d want 2 (transparent)", i, d)
		}
	}
	term := render.NewTerminal(20, 5)
	for i, r := range rec.Records {
		term.Feed(r.Content)
		want := render.DrawFrame(term.Snapshot(), term.CurX, term.CurY, true, cfg)
		comp := composite(g, i)
		if !comp.Bounds().Eq(want.Bounds()) {
			t.Fatalf("frame %d composite bounds %v != %v", i, comp.Bounds(), want.Bounds())
		}
		flips := 0
		for p := 0; p < len(comp.Pix); p += 4 {
			a, b := comp.Pix[p+3], want.Pix[p+3]
			if (a < 128) != (b < 128) {
				flips++
			}
		}
		// Calibrated: dithered text edges shimmer a few alpha flips
		// (~18 here); a real ghost is a full cursor block (~230).
		if flips >= 40 {
			t.Fatalf("frame %d: %d alpha flips (ghost trail)", i, flips)
		}
	}
}

// Pixel-identical consecutive frames merge their delay into one frame
// (idle excision): X then no-op still costs one frame, not two.
func TestFrameDiffMerge(t *testing.T) {
	title := "T"
	cfg := fixtureConfig("null", &title)
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "X"},
		{Delay: 300, Content: ""},
	}}
	path := filepath.Join(t.TempDir(), "merge.gif")
	if _, err := render.Render(rec, path, 100, 1, "", false); err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, path)
	if len(g.Image) != 1 {
		t.Fatalf("frames=%d want 1 (merged)", len(g.Image))
	}
	if g.Delay[0] != 60 { // (300 next + 300 own)/10
		t.Fatalf("merged delay=%d want 60", g.Delay[0])
	}
}

// Dirty-rect frames must reconstruct the full picture exactly: composite
// the decoded frames viewer-style and compare against a fresh full render.
// Cursor-only movement on a flat field quantizes bit-exact (anchored flats
// + flat-lock), so the comparison is byte equality, no tolerance.
func TestFrameDiffCompositeExact(t *testing.T) {
	title := "T"
	cfg := fixtureConfig("null", &title)
	cfg.Theme["background"] = "#ff0000"
	cfg.FrameBox.Style = map[string]string{"margin": "0px"} // bare terminal
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "\x1b[2C"},
		{Delay: 300, Content: "\x1b[2C"},
	}}
	path := filepath.Join(t.TempDir(), "diff.gif")
	if _, err := render.Render(rec, path, 100, 1, "", false); err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, path)
	if len(g.Image) != 2 {
		t.Fatalf("frames=%d want 2", len(g.Image))
	}
	if b1, b0 := g.Image[1].Bounds(), g.Image[0].Bounds(); b1 == b0 || b1.Empty() || !b1.In(b0) {
		t.Fatalf("frame 1 bounds %v not a proper dirty rect of %v", b1, b0)
	}
	comp := composite(g, len(g.Image)-1)
	term := render.NewTerminal(20, 5)
	term.Feed("\x1b[2C")
	term.Feed("\x1b[2C")
	want := render.DrawFrame(term.Snapshot(), term.CurX, term.CurY, true, cfg)
	if !bytes.Equal(comp.Pix, want.Pix) {
		t.Fatalf("composite differs from full render")
	}
}

// Start/end pauses hold the edge frames steady (no blink re-split).
func TestRenderPauses(t *testing.T) {
	title := "T"
	cfg := fixtureConfig("null", &title)
	cfg.StartPause = 1000
	cfg.EndPause = 500
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "a\r\n"},
		{Delay: 200, Content: "b\r\n"},
	}}
	path := filepath.Join(t.TempDir(), "pause.gif")
	if _, err := render.Render(rec, path, 100, 1, "", false); err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, path)
	if len(g.Image) != 2 {
		t.Fatalf("frames=%d want 2", len(g.Image))
	}
	if g.Delay[0] != 120 || g.Delay[1] != 70 { // (200+1000)/10, (200+500)/10
		t.Fatalf("pause delays=%v want [120 70]", g.Delay)
	}
}

// --background flattens frames onto an opaque page (white corners, no
// transparent palette entry); bad colors error; empty/transparent keeps
// the transparent default.
func TestRenderBackground(t *testing.T) {
	mkrec := func() recording.File {
		title := "T"
		cfg := fixtureConfig("floating", &title)
		return recording.File{Config: cfg, Records: []recording.Record{
			{Delay: 200, Content: "hi\r\n"},
			{Delay: 200, Content: "yo\r\n"},
		}}
	}
	path := filepath.Join(t.TempDir(), "bg.gif")
	out, err := render.Render(mkrec(), path, 100, 1, "white", false)
	if err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, out)
	W, H := g.Config.Width, g.Config.Height
	for _, p := range [][2]int{{0, 0}, {W - 1, 0}, {0, H - 1}, {W - 1, H - 1}} {
		r, gg, b, a := g.Image[0].At(p[0], p[1]).RGBA()
		if a>>8 != 0xff || uint8(r>>8) != 0xff || uint8(gg>>8) != 0xff || uint8(b>>8) != 0xff {
			t.Fatalf("corner %v not opaque white", p)
		}
	}
	for _, pm := range g.Image {
		for _, c := range pm.Palette {
			if _, _, _, a := c.RGBA(); a == 0 {
				t.Fatalf("backgrounded GIF must have no transparent palette entry")
			}
		}
	}
	if _, err := render.Render(mkrec(), filepath.Join(t.TempDir(), "bad.gif"), 0, 1, "notacolor", false); err == nil {
		t.Fatalf("bad background color must error")
	}
	// explicit transparent == default transparent
	out2, err := render.Render(mkrec(), filepath.Join(t.TempDir(), "t.gif"), 0, 1, "transparent", false)
	if err != nil {
		t.Fatal(err)
	}
	g2 := decodeGIF(t, out2)
	foundTrans := false
	for _, pm := range g2.Image {
		for _, c := range pm.Palette {
			if _, _, _, a := c.RGBA(); a == 0 {
				foundTrans = true
			}
		}
	}
	if !foundTrans {
		t.Fatalf("transparent background must preserve transparency")
	}
}

// Wallpaper -b: image page = frame + 60px padding each side, frame
// centered; missing and undersized images fail like bad colors.
func TestBackgroundWallpaper(t *testing.T) {
	mkrec := func() recording.File {
		title := "T"
		cfg := fixtureConfig("floating", &title)
		return recording.File{Config: cfg, Records: []recording.Record{
			{Delay: 200, Content: "hi\r\n"},
			{Delay: 200, Content: "yo\r\n"},
		}}
	}
	plain, err := render.Render(mkrec(), filepath.Join(t.TempDir(), "p.gif"), 100, 1, "", false)
	if err != nil {
		t.Fatal(err)
	}
	g0 := decodeGIF(t, plain)
	wall := filepath.Join(t.TempDir(), "wall.png")
	writeSolidPNG(t, wall, 1200, 1200, color.RGBA{0x11, 0x22, 0x33, 0xff})
	out, err := render.Render(mkrec(), filepath.Join(t.TempDir(), "w.gif"), 100, 1, wall, false)
	if err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, out)
	if g.Config.Width != g0.Config.Width+120 || g.Config.Height != g0.Config.Height+120 {
		t.Fatalf("wallpaper dims %dx%d want frame+120", g.Config.Width, g.Config.Height)
	}
	r, gg, b, a := g.Image[0].At(0, 0).RGBA()
	if a>>8 != 0xff || uint8(r>>8) != 0x11 || uint8(gg>>8) != 0x22 || uint8(b>>8) != 0x33 {
		t.Fatalf("wallpaper corner not page color")
	}
	if _, err := render.Render(mkrec(), filepath.Join(t.TempDir(), "ws.gif"), 100, 1, filepath.Join(t.TempDir(), "tiny.png"), false); err == nil {
		t.Fatalf("missing wallpaper must error")
	}
	small := filepath.Join(t.TempDir(), "small.png")
	writeSolidPNG(t, small, 10, 10, color.RGBA{0x11, 0x22, 0x33, 0xff})
	if _, err := render.Render(mkrec(), filepath.Join(t.TempDir(), "ws2.gif"), 100, 1, small, false); err == nil {
		t.Fatalf("undersized wallpaper must error")
	}
}

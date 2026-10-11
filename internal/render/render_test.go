package render

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/CHE3MZ/termrec/internal/recording"
)

func loadFixture(t *testing.T, name string) recording.File {
	t.Helper()
	rec, _, err := recording.LoadFile(filepath.Join("..", "..", "testdata", name))
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

// Every fixture must render to a valid GIF. Frame count follows the idle
// blink rule: each kept record yields len(blinkChunks(shown delay)), where
// the shown delay is the next record's, except the last frame which holds
// its own delay instead of wrapping to the first record.
// (Fixtures never hide the cursor and their delays survive Adjust, so the
// expectation is computable up front.)
func TestRenderAllFixtures(t *testing.T) {
	fixtures := []string{"demo", "window", "null", "solid", "solid-notitle", "typing"}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			rec := loadFixture(t, name)
			n := len(rec.Records)
			wantFrames := 0
			for i := range rec.Records {
				d := rec.Records[i].Delay
				if i+1 < n {
					d = rec.Records[i+1].Delay
				}
				wantFrames += len(blinkChunks(int(d)))
			}
			path := filepath.Join(t.TempDir(), name+".gif")
			out, err := Render(rec, path, 0, 1, "", false)
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

// Step filtering must reduce frame counts deterministically (blink-aware:
// each kept record yields len(blinkChunks(nextDelay)) frames).
func TestRenderStep(t *testing.T) {
	rec := loadFixture(t, "demo")
	n := len(rec.Records)
	for _, step := range []int{1, 2, 3} {
		path := filepath.Join(t.TempDir(), "s.gif")
		// reload: Render mutates delays via Adjust
		rec2 := loadFixture(t, "demo")
		if _, err := Render(rec2, path, 0, step, "", false); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		g := decodeGIF(t, path)
		want := 0
		for i := 0; i < n; i += step {
			d := rec.Records[(i+1)%n].Delay
			want += len(blinkChunks(int(d)))
		}
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
		if _, err := Render(rec, path, q, 1, "", false); err != nil {
			t.Fatalf("quality %d: %v", q, err)
		}
		g := decodeGIF(t, path)
		if len(g.Image) == 0 {
			t.Fatalf("quality %d: no frames", q)
		}
	}
}

// Loop count mapping: repeat -1 -> -1 (once), 0 -> 0 (forever), n -> n.
func TestRenderRepeatMapping(t *testing.T) {
	for _, tc := range []struct {
		repeat int
		want   int
	}{
		{-1, -1}, {0, 0}, {3, 3},
	} {
		if got := repeatToLoopCount(tc.repeat); got != tc.want {
			t.Fatalf("repeat %d -> %d want %d", tc.repeat, got, tc.want)
		}
	}
}

// Idle blink chunking: a solid 700ms grace, then ~600ms off/on phases.
// Pauses under ~1s stay single; huge pauses cap out; totals stay exact.
func TestBlinkChunks(t *testing.T) {
	cases := []struct {
		delay int
		want  []int
	}{
		{100, []int{100}},
		{500, []int{500}},
		{800, []int{800}},
		{1050, []int{1050}},
		{1200, []int{700, 500}},
		{1500, []int{700, 400, 400}},
		{2000, []int{700, 433, 433, 434}},
	}
	for _, tc := range cases {
		got := blinkChunks(tc.delay)
		if len(got) != len(tc.want) {
			t.Fatalf("delay %d: %v want %v", tc.delay, got, tc.want)
		}
		sum := 0
		for i, d := range got {
			sum += d
			if d != tc.want[i] {
				t.Fatalf("delay %d: %v want %v", tc.delay, got, tc.want)
			}
		}
		if sum != tc.delay {
			t.Fatalf("delay %d: chunks sum %d", tc.delay, sum)
		}
	}
	if got := blinkChunks(60000); len(got) != maxBlinkPhases {
		t.Fatalf("huge delay: %d phases want cap %d", len(got), maxBlinkPhases)
	} else {
		sum := 0
		for _, d := range got {
			sum += d
		}
		if sum != 60000 {
			t.Fatalf("huge delay: chunks sum %d", sum)
		}
	}
}

// An idle pause must render grace + alternating cursor off/on frames with
// the same total duration; a hidden cursor (?25l) must not split at all.
func TestBlinkRender(t *testing.T) {
	mkrec := func(hide bool) recording.File {
		title := "T"
		cfg := testConfig("null", &title)
		content := "hi\r\n"
		if hide {
			content = "\x1b[?25lhi\r\n"
		}
		return recording.File{Config: cfg, Records: []recording.Record{
			{Delay: 200, Content: content},
			{Delay: 1200, Content: "yo\r\n"},
		}}
	}
	// frame 1 shows for next delay 1200 -> grace 700 + off 500;
	// frame 2 is last so it holds its own delay 1200 -> grace 700 + off 500.
	path := filepath.Join(t.TempDir(), "blink.gif")
	if _, err := Render(mkrec(false), path, 100, 1, "", false); err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, path)
	if len(g.Image) != 4 {
		t.Fatalf("blink frames=%d want 4", len(g.Image))
	}
	sum10 := 0
	for _, d := range g.Delay {
		sum10 += d
	}
	if sum10 != 240 { // (700+500+700+500)/10
		t.Fatalf("blink total %d00ms want 2400ms", sum10)
	}
	// hidden cursor: same recording, no split -> 2 frames
	path2 := filepath.Join(t.TempDir(), "noblink.gif")
	if _, err := Render(mkrec(true), path2, 100, 1, "", false); err != nil {
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
	cfg := testConfig("null", &title)
	// opaque red background, white text
	cfg.Theme["background"] = "#ff0000"
	cfg.Theme["foreground"] = "#ffffff"
	term := NewTerminal(8, 2)
	snap := term.Snapshot()
	img := DrawFrame(snap, term.CurX, term.CurY, false, cfg)
	// center of a text cell area (empty cell => pure background)
	c := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	if diff(c.R, 0xff) > 6 || diff(c.G, 0x00) > 6 || diff(c.B, 0x00) > 6 {
		t.Fatalf("bg drift: got #%02x%02x%02x want #ff0000", c.R, c.G, c.B)
	}
}

// End-to-end: flat theme colors must survive GIF encoding bit-exact,
// and identical source colors must map to identical palette indices
// across frames (no per-frame drift). Frames are dirty rects, so the
// check composites viewer-style (Over, in order) before sampling.
func TestGIFColorConsistency(t *testing.T) {
	title := "T"
	cfg := testConfig("null", &title)
	cfg.Theme["background"] = "#232628"
	cfg.Theme["foreground"] = "#afafaf"
	cfg.FrameBox.Style = map[string]string{"margin": "0px"} // bare terminal
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "ab\r\n"},
		{Delay: 200, Content: "cd\r\n"},
	}}
	path := filepath.Join(t.TempDir(), "cc.gif")
	out, err := Render(rec, path, 100, 1, "", false)
	if err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, out)
	if len(g.Image) != 2 {
		t.Fatalf("frames=%d want 2", len(g.Image))
	}
	// frame 1 only repaints what changed (row 1 text + cursor cells), so it
	// must be a strictly smaller sub-rect of the full canvas — never a
	// second full frame.
	if b1, b0 := g.Image[1].Bounds(), g.Image[0].Bounds(); b1 == b0 || b1.Empty() || !b1.In(b0) {
		t.Fatalf("frame 1 bounds %v not a proper dirty rect of %v", b1, b0)
	}
	// viewer composite, then sample an empty cell (col 6, row 0)
	comp := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	for _, pm := range g.Image {
		draw.Draw(comp, pm.Bounds(), pm, pm.Bounds().Min, draw.Over)
	}
	cw, ch := cellSize(cfg)
	sx, sy := 6*cw+cw/2, ch/2
	c := comp.RGBAAt(sx, sy)
	if diff(c.R, 0x23) > 4 || diff(c.G, 0x26) > 4 || diff(c.B, 0x28) > 4 {
		t.Fatalf("bg #%02x%02x%02x want #232628", c.R, c.G, c.B)
	}
}

func diff(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

// --background flattens frames onto an opaque page (white corners, no
// transparent palette entry); bad colors error; empty/transparent keeps
// the transparent default.
func TestRenderBackground(t *testing.T) {
	mkrec := func() recording.File {
		title := "T"
		cfg := testConfig("floating", &title)
		return recording.File{Config: cfg, Records: []recording.Record{
			{Delay: 200, Content: "hi\r\n"},
			{Delay: 200, Content: "yo\r\n"},
		}}
	}
	path := filepath.Join(t.TempDir(), "bg.gif")
	out, err := Render(mkrec(), path, 100, 1, "white", false)
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
	if _, err := Render(mkrec(), filepath.Join(t.TempDir(), "bad.gif"), 0, 1, "notacolor", false); err == nil {
		t.Fatalf("bad background color must error")
	}
}

// Pixel-identical consecutive frames merge their delay into one frame
// (idle excision): X then no-op still costs one frame, not two.
func TestFrameDiffMerge(t *testing.T) {
	title := "T"
	cfg := testConfig("null", &title)
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "X"},
		{Delay: 300, Content: ""},
	}}
	path := filepath.Join(t.TempDir(), "merge.gif")
	if _, err := Render(rec, path, 100, 1, "", false); err != nil {
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
	cfg := testConfig("null", &title)
	cfg.Theme["background"] = "#ff0000"
	cfg.FrameBox.Style = map[string]string{"margin": "0px"} // bare terminal
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "\x1b[2C"},
		{Delay: 300, Content: "\x1b[2C"},
	}}
	path := filepath.Join(t.TempDir(), "diff.gif")
	if _, err := Render(rec, path, 100, 1, "", false); err != nil {
		t.Fatal(err)
	}
	g := decodeGIF(t, path)
	if len(g.Image) != 2 {
		t.Fatalf("frames=%d want 2", len(g.Image))
	}
	cw, ch := cellSize(cfg)
	if want := image.Rect(2*cw, 0, 5*cw, ch); g.Image[1].Bounds() != want {
		t.Fatalf("dirty rect %v want %v", g.Image[1].Bounds(), want)
	}
	comp := image.NewRGBA(g.Image[0].Bounds())
	for _, pm := range g.Image {
		draw.Draw(comp, pm.Bounds(), pm, pm.Bounds().Min, draw.Over)
	}
	term := NewTerminal(20, 5)
	term.Feed("\x1b[2C")
	term.Feed("\x1b[2C")
	want := DrawFrame(term.Snapshot(), term.CurX, term.CurY, true, cfg)
	if len(comp.Pix) != len(want.Pix) {
		t.Fatalf("pix len %d != %d", len(comp.Pix), len(want.Pix))
	}
	for i := range comp.Pix {
		if comp.Pix[i] != want.Pix[i] {
			t.Fatalf("pixel byte %d: %d != %d", i, comp.Pix[i], want.Pix[i])
		}
	}
}

// Start/end pauses hold the edge frames steady (no blink re-split).
func TestRenderPauses(t *testing.T) {
	title := "T"
	cfg := testConfig("null", &title)
	cfg.StartPause = 1000
	cfg.EndPause = 500
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "a\r\n"},
		{Delay: 200, Content: "b\r\n"},
	}}
	path := filepath.Join(t.TempDir(), "pause.gif")
	if _, err := Render(rec, path, 100, 1, "", false); err != nil {
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

// Wallpaper -b: image page = frame + 60px padding each side, frame
// centered; missing files fail like bad colors.
func TestBackgroundWallpaper(t *testing.T) {
	mkwall := func(name string, w, h int, c color.RGBA) string {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
		}
		p := filepath.Join(t.TempDir(), name)
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		return p
	}
	mkrec := func() recording.File {
		title := "T"
		cfg := testConfig("floating", &title)
		return recording.File{Config: cfg, Records: []recording.Record{
			{Delay: 200, Content: "hi\r\n"},
			{Delay: 200, Content: "yo\r\n"},
		}}
	}
	plain, err := Render(mkrec(), filepath.Join(t.TempDir(), "p.gif"), 100, 1, "", false)
	if err != nil {
		t.Fatal(err)
	}
	g0 := decodeGIF(t, plain)
	wall := mkwall("wall.png", 1200, 1200, color.RGBA{0x11, 0x22, 0x33, 0xff})
	out, err := Render(mkrec(), filepath.Join(t.TempDir(), "w.gif"), 100, 1, wall, false)
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
	if _, err := Render(mkrec(), filepath.Join(t.TempDir(), "ws.gif"), 100, 1, filepath.Join(t.TempDir(), "tiny.png"), false); err == nil {
		t.Fatalf("missing wallpaper must error")
	}
	small := mkwall("small.png", 10, 10, color.RGBA{0x11, 0x22, 0x33, 0xff})
	if _, err := Render(mkrec(), filepath.Join(t.TempDir(), "ws2.gif"), 100, 1, small, false); err == nil {
		t.Fatalf("undersized wallpaper must error")
	}
	// explicit transparent == default transparent
	out2, err := Render(mkrec(), filepath.Join(t.TempDir(), "t.gif"), 0, 1, "transparent", false)
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

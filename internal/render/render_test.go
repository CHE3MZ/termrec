package render

import (
	"image/gif"
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

// Every fixture must render to a valid GIF with matching frame/delay counts.
func TestRenderAllFixtures(t *testing.T) {
	fixtures := []string{"demo", "window", "null", "solid", "solid-notitle"}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			rec := loadFixture(t, name)
			wantFrames := len(rec.Records)
			path := filepath.Join(t.TempDir(), name+".gif")
			out, err := Render(rec, path, 0, 1)
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
			// all frames share dimensions
			for i, pm := range g.Image {
				if pm.Bounds().Dx() != g.Config.Width || pm.Bounds().Dy() != g.Config.Height {
					t.Fatalf("frame %d dims %v != %dx%d", i, pm.Bounds(), g.Config.Width, g.Config.Height)
				}
			}
		})
	}
}

// Step filtering must reduce frame counts deterministically.
func TestRenderStep(t *testing.T) {
	rec := loadFixture(t, "demo")
	n := len(rec.Records)
	for _, step := range []int{1, 2, 3} {
		path := filepath.Join(t.TempDir(), "s.gif")
		// reload: Render mutates delays via Adjust
		rec2 := loadFixture(t, "demo")
		if _, err := Render(rec2, path, 0, step); err != nil {
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
		if _, err := Render(rec, path, q, 1); err != nil {
			t.Fatalf("quality %d: %v", q, err)
		}
		g := decodeGIF(t, path)
		if len(g.Image) == 0 {
			t.Fatalf("quality %d: no frames", q)
		}
	}
}

// Loop count mapping: repeat -1 -> 1, 0 -> 0, n -> n.
func TestRenderRepeatMapping(t *testing.T) {
	for _, tc := range []struct {
		repeat int
		want   int
	}{
		{-1, 1}, {0, 0}, {3, 3},
	} {
		if got := repeatToLoopCount(tc.repeat); got != tc.want {
			t.Fatalf("repeat %d -> %d want %d", tc.repeat, got, tc.want)
		}
	}
}

// Rendered flat colors must match the theme (color consistency between
// config values and GIF pixels). Tolerance allows for palette rounding;
// the anchored-palette fix should keep this exact.
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
	if diff(c.R, 0xff) > 24 || diff(c.G, 0x00) > 24 || diff(c.B, 0x00) > 24 {
		t.Fatalf("bg drift: got #%02x%02x%02x want #ff0000", c.R, c.G, c.B)
	}
}

func diff(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

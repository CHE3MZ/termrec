package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CHE3MZ/termrec/assets/fonts"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
)

func TestFontFallbackUnknownFamily(t *testing.T) {
	face := monoFaceFor("No Such Font XYZ, Also Missing", 12)
	if face == nil {
		t.Fatalf("nil face")
	}
	// metrics must still work (fallback to Go Mono)
	cw, ch, asc := cellMetrics(testConfig("null", nil))
	_ = cw
	_ = ch
	_ = asc
	if cw <= 0 || ch <= 0 {
		t.Fatalf("bad metrics %dx%d", cw, ch)
	}
}

func TestSystemFontLookupViaEnv(t *testing.T) {
	dir := t.TempDir()
	// plant Go Mono bytes under a fake family name
	if err := os.WriteFile(filepath.Join(dir, "Testfakemono.ttf"), gomono.TTF, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TERMREC_FONT_DIRS", dir)
	ft, err := findSystemFont("TestFakeMono")
	if err != nil || ft == nil {
		t.Fatalf("lookup: %v", err)
	}
	face := monoFaceFor("TestFakeMono", 12)
	if face == nil {
		t.Fatalf("nil face")
	}
}

func TestCellMetricsVaryBySize(t *testing.T) {
	c1 := testConfig("null", nil)
	c1.FontSize = 12
	c2 := testConfig("null", nil)
	c2.FontSize = 24
	w1, h1, _ := cellMetrics(c1)
	w2, h2, _ := cellMetrics(c2)
	if w2 <= w1 || h2 <= h1 {
		t.Fatalf("metrics must grow: %dx%d vs %dx%d", w1, h1, w2, h2)
	}
}

// Unknown families (no generic in list) must fall back to embedded Geist
// Mono Medium even on machines with system mono fonts installed
// (regression: the generic fallback list used to swallow every lookup).
func TestUnknownFamilyFallsBackToGeist(t *testing.T) {
	gm, err := opentype.Parse(fonts.GeistMonoMedium)
	if err != nil {
		t.Fatal(err)
	}
	want, err := opentype.NewFace(gm, &opentype.FaceOptions{Size: 48, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		t.Fatal(err)
	}
	got := monoFaceFor("Definitely Not A Font ZZZ", 48)
	if font.MeasureString(got, "M") != font.MeasureString(want, "M") {
		t.Fatalf("unknown family did not fall back to Geist Mono Medium")
	}
	if _, err := findSystemFont("Definitely Not A Font ZZZ"); err == nil {
		t.Fatalf("unknown family should not resolve")
	}
}

// Per-rune fallback: latin stays on the primary face, CJK (absent from
// Geist Mono) falls through to a later chain face instead of drawing tofu.
func TestFaceForRuneFallback(t *testing.T) {
	cfg := testConfig("null", nil)
	cfg.FontFamily = ""
	chain := monoFacesFor(cfg, 24)
	primary := monoFaceFor("", 24)
	if got := faceForRune(chain, 'A'); got != primary {
		t.Fatalf("latin must resolve to primary face")
	}
	if got := faceForRune(chain, '中'); got == primary {
		t.Fatalf("CJK must fall past Geist Mono (no CJK coverage)")
	}
	if faceForRune(nil, 'A') == nil {
		t.Fatalf("empty chain must yield last-resort face, not nil")
	}
}

// Explicit fontFile wins over family lookup, even for uninstalled files.
func TestFontFileOverride(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "custom-mono.ttf")
	if err := os.WriteFile(p, gomono.TTF, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig("null", nil)
	cfg.FontFamily = "Definitely Not A Font ZZZ"
	cfg.FontFile = p
	chain := monoFacesFor(cfg, 24)
	gm, err := opentype.Parse(gomono.TTF)
	if err != nil {
		t.Fatal(err)
	}
	want, err := opentype.NewFace(gm, &opentype.FaceOptions{Size: 24, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		t.Fatal(err)
	}
	if font.MeasureString(chain[0], "M") != font.MeasureString(want, "M") {
		t.Fatalf("fontFile did not win over fontFamily")
	}
	// missing file falls through to the family face, never nil
	cfg.FontFile = filepath.Join(dir, "nope.ttf")
	fam := monoFaceFor("Definitely Not A Font ZZZ", 24)
	if got := primaryFace(cfg, 24); font.MeasureString(got, "M") != font.MeasureString(fam, "M") {
		t.Fatalf("missing fontFile must fall back to fontFamily")
	}
}

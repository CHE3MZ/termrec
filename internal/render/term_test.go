package render

import (
	"image/gif"
	"os"
	"testing"

	"github.com/CHE3MZ/termrec/internal/recording"
)

func TestSGRColors(t *testing.T) {
	term := NewTerminal(10, 2)
	term.Feed("\x1b[31mR\x1b[0mN")
	if term.Grid[0][0].Ch != 'R' || term.Grid[0][0].FG != "red" {
		t.Fatalf("SGR red: %+v", term.Grid[0][0])
	}
	if term.Grid[0][1].Ch != 'N' || term.Grid[0][1].FG != "foreground" {
		t.Fatalf("SGR reset: %+v", term.Grid[0][1])
	}
}

func TestWideRunes(t *testing.T) {
	if runeWidth('A') != 1 || runeWidth('中') != 2 || runeWidth('😀') != 2 {
		t.Fatalf("widths: A=%d 中=%d emoji=%d", runeWidth('A'), runeWidth('中'), runeWidth('😀'))
	}
	if runeWidth('́') != 0 { // combining acute
		t.Fatalf("combining width: %d", runeWidth('́'))
	}
	term := NewTerminal(6, 2)
	term.Feed("A中B")
	if term.Grid[0][0].Ch != 'A' || term.Grid[0][1].Ch != '中' {
		t.Fatalf("wide place: %+v %+v", term.Grid[0][0], term.Grid[0][1])
	}
	if term.Grid[0][2].Ch != 0 {
		t.Fatalf("continuation cell must be Ch==0, got %+v", term.Grid[0][2])
	}
	if term.Grid[0][3].Ch != 'B' || term.CurX != 4 {
		t.Fatalf("cursor must advance 2 for wide: CurX=%d %+v", term.CurX, term.Grid[0][3])
	}
	// wide rune at last column wraps instead of splitting
	term2 := NewTerminal(4, 2)
	term2.Feed("ABC中")
	if term2.Grid[1][0].Ch != '中' || term2.Grid[0][3].Ch != ' ' {
		t.Fatalf("wide wrap: row0=%q row1=%q", string(term2.Grid[0][3].Ch), string(term2.Grid[1][0].Ch))
	}
	// overwriting a wide half clears the orphaned partner
	term3 := NewTerminal(6, 2)
	term3.Feed("A中B")
	term3.Feed("\x1b[1;2HX") // X lands on the wide first half
	if term3.Grid[0][1].Ch != 'X' || term3.Grid[0][2].Ch != ' ' {
		t.Fatalf("orphan clear: %+v %+v", term3.Grid[0][1], term3.Grid[0][2])
	}
	term4 := NewTerminal(6, 2)
	term4.Feed("A中B")
	term4.Feed("\x1b[1;3HY") // Y lands on the continuation cell
	if term4.Grid[0][2].Ch != 'Y' || term4.Grid[0][1].Ch != ' ' {
		t.Fatalf("continuation overwrite: %+v %+v", term4.Grid[0][1], term4.Grid[0][2])
	}
}

func TestNewlineAndCursor(t *testing.T) {
	term := NewTerminal(5, 3)
	term.Feed("AB\r\nCD")
	if term.Grid[0][0].Ch != 'A' || term.Grid[1][0].Ch != 'C' {
		t.Fatalf("newline: %+v", term.Grid)
	}
	term2 := NewTerminal(5, 3)
	term2.Feed("\x1b[2J")
	for y := range term2.Grid {
		for x := range term2.Grid[y] {
			if term2.Grid[y][x].Ch != ' ' {
				t.Fatalf("clear")
			}
		}
	}
	term3 := NewTerminal(5, 3)
	term3.Feed("\x1b[2;3H*")
	if term3.Grid[1][2].Ch != '*' {
		t.Fatalf("cursor pos: %+v", term3.Grid[1][2])
	}
}

func TestBrightAndErase(t *testing.T) {
	term := NewTerminal(8, 2)
	term.Feed("\x1b[1;32mG")
	if term.Grid[0][0].FG != "brightGreen" {
		t.Fatalf("bold green -> brightGreen, got %q", term.Grid[0][0].FG)
	}
	term.Feed("\x1b[2K")
	for x := range term.Grid[0] {
		if term.Grid[0][x].Ch != ' ' {
			t.Fatalf("erase line")
		}
	}
}

func TestSGR256AndTruecolor(t *testing.T) {
	term := NewTerminal(8, 2)
	term.Feed("\x1b[38;5;196mX")
	if term.Grid[0][0].FGCustom == nil {
		t.Fatalf("256-color fg not stored: %+v", term.Grid[0][0])
	}
	term.Feed("\x1b[0m\x1b[48;2;10;20;30mY")
	if term.Grid[0][1].BGCustom == nil {
		t.Fatalf("truecolor bg not stored: %+v", term.Grid[0][1])
	}
	c := *term.Grid[0][1].BGCustom
	if c.R != 10 || c.G != 20 || c.B != 30 {
		t.Fatalf("truecolor value: %+v", c)
	}
}

func TestCursorVisibility(t *testing.T) {
	term := NewTerminal(8, 2)
	if !term.CursorVisible {
		t.Fatalf("default visible")
	}
	term.Feed("\x1b[?25l")
	if term.CursorVisible {
		t.Fatalf("hide failed")
	}
	term.Feed("\x1b[?25h")
	if !term.CursorVisible {
		t.Fatalf("show failed")
	}
}

func TestAltScreenClears(t *testing.T) {
	term := NewTerminal(8, 2)
	term.Feed("hi")
	term.Feed("\x1b[?1049h")
	if term.Grid[0][0].Ch != ' ' {
		t.Fatalf("alt screen should clear")
	}
}

func TestFrameSizes(t *testing.T) {
	cfgs := []string{"null", "window", "floating", "solid"}
	for _, ft := range cfgs {
		title := "T"
		s := ft
		cfg := testConfig(s, &title)
		term := NewTerminal(20, 5)
		term.Feed("hello")
		img := DrawFrame(term.Snapshot(), term.CurX, term.CurY, term.CursorVisible, cfg)
		if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
			t.Fatalf("%s zero size", ft)
		}
	}
}

// CSS-exact geometry: window titlebar 22px content + 1px top highlight +
// 1px bottom border (24 total) + body pad 10; floating 34 + 20.
func TestFrameGeometryCSS(t *testing.T) {
	title := "T"
	term := NewTerminal(20, 5)
	term.Feed("hi")
	snap := term.Snapshot()
	cw, ch := cellSize(testConfig("null", nil))
	termW, termH := 20*cw, 5*ch

	wcfg := testConfig("window", &title)
	wimg := DrawFrame(snap, term.CurX, term.CurY, true, wcfg)
	// card = term + 2*10 pad, +24 title; outer = card + 2*18 margin + 2*21 glow + 2 shadow drop
	wantW := termW + 20 + 36 + 42
	wantH := termH + 20 + 24 + 36 + 42 + 2
	if wimg.Bounds().Dx() != wantW || wimg.Bounds().Dy() != wantH {
		t.Fatalf("window geometry: got %dx%d want %dx%d", wimg.Bounds().Dx(), wimg.Bounds().Dy(), wantW, wantH)
	}

	fcfg := testConfig("floating", &title)
	fimg := DrawFrame(snap, term.CurX, term.CurY, true, fcfg)
	wantW = termW + 40 + 36 + 42
	wantH = termH + 40 + 34 + 36 + 42 + 2
	if fimg.Bounds().Dx() != wantW || fimg.Bounds().Dy() != wantH {
		t.Fatalf("floating geometry: got %dx%d want %dx%d", fimg.Bounds().Dx(), fimg.Bounds().Dy(), wantW, wantH)
	}
}

// Transparent by default: default theme bg is "transparent", so a fresh
// floating render must contain transparent pixels and the GIF must flag them.
func TestTransparentDefault(t *testing.T) {
	title := "T"
	cfg := testConfig("floating", &title) // default theme bg transparent
	term := NewTerminal(10, 3)
	snap := term.Snapshot()
	img := DrawFrame(snap, term.CurX, term.CurY, true, cfg)
	found := false
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] < 128 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected transparent pixels in default floating frame (corners/margins)")
	}
	// end-to-end: render GIF and check transparency survives encoding
	rec := recording.File{Config: cfg, Records: []recording.Record{
		{Delay: 200, Content: "hi\r\n"},
		{Delay: 200, Content: "yo\r\n"},
	}}
	path := t.TempDir() + "/out.gif"
	out, err := Render(rec, path, 0, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatal(err)
	}
	transparentFrame := false
	for _, pm := range g.Image {
		for _, c := range pm.Palette {
			_, _, _, a := c.RGBA()
			if a == 0 {
				transparentFrame = true
				break
			}
		}
	}
	if !transparentFrame {
		t.Fatalf("expected GIF palette transparency")
	}
}

package render

import "testing"

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

func TestNewlineAndCursor(t *testing.T) {
	term := NewTerminal(5, 3)
	term.Feed("AB\nCD")
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

func TestFrameSizes(t *testing.T) {
	cfgs := []string{"null", "window", "floating", "solid"}
	for _, ft := range cfgs {
		title := "T"
		s := ft
		cfg := testConfig(s, &title)
		term := NewTerminal(20, 5)
		term.Feed("hello")
		img := DrawFrame(term.Snapshot(), term.CurX, term.CurY, cfg)
		if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
			t.Fatalf("%s zero size", ft)
		}
	}
}

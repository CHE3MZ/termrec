// Package render converts recordings to animated GIFs without a browser.
// Pipeline: load .yml -> adjust delays -> emulate terminal grid per record
// -> draw RGBA (draw.go + framebox.go) -> watermark -> encode GIF (gif.go).
package render

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/CHE3MZ/termrec/internal/theme"
)

// Xterm256Color maps a 256-color index to RGBA via the theme palette table.
func Xterm256Color(n int) color.RGBA { return theme.Xterm256(n) }

// Cell is one terminal grid cell.
type Cell struct {
	Ch        rune
	FG        string // theme key, e.g. "foreground", "red", "brightWhite"; ""+FGCustom!=nil => custom
	BG        string // theme key or "" for default background; ""+BGCustom!=nil => custom
	FGCustom  *color.RGBA
	BGCustom  *color.RGBA
	Bold      bool
	Dim       bool
	Inverse   bool
	Underline bool
}

// Terminal is an ANSI/SGR emulator covering what xterm 3.x emits for
// terminalizer recordings: printable text, \n \r \b \t, SGR (incl. 256/truecolor),
// cursor moves, erase, scroll regions (treated as full), alt-screen (treated as clear),
// cursor save/restore + visibility.
type Terminal struct {
	Cols, Rows     int
	Grid           [][]Cell
	CurX, CurY     int
	CurFG          string
	CurBG          string
	CurFGCustom    *color.RGBA
	CurBGCustom    *color.RGBA
	CurBold        bool
	CurDim         bool
	CurInverse     bool
	CurUnderline   bool
	CursorVisible  bool
	savedX, savedY int
	savedFG        string
	savedBG        string
}

func NewTerminal(cols, rows int) *Terminal {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	t := &Terminal{Cols: cols, Rows: rows, CurFG: "foreground", CursorVisible: true}
	t.Grid = make([][]Cell, rows)
	for y := range t.Grid {
		t.Grid[y] = make([]Cell, cols)
		for x := range t.Grid[y] {
			t.Grid[y][x] = Cell{Ch: ' ', FG: "foreground"}
		}
	}
	return t
}

func (t *Terminal) resetRow(y int) {
	for x := range t.Grid[y] {
		t.Grid[y][x] = Cell{Ch: ' ', FG: "foreground"}
	}
}

func (t *Terminal) clearAll() {
	for y := range t.Grid {
		t.resetRow(y)
	}
	t.CurX, t.CurY = 0, 0
}

func (t *Terminal) scroll() {
	copy(t.Grid, t.Grid[1:])
	t.Grid[t.Rows-1] = make([]Cell, t.Cols)
	t.resetRow(t.Rows - 1)
	if t.CurY > 0 {
		t.CurY--
	}
}

func (t *Terminal) put(ch rune) {
	if t.CurY < 0 {
		t.CurY = 0
	}
	if t.CurY >= t.Rows {
		t.scroll()
		t.CurY = t.Rows - 1
	}
	if t.CurX < 0 {
		t.CurX = 0
	}
	if t.CurX >= t.Cols {
		t.CurX = 0
		t.CurY++
		if t.CurY >= t.Rows {
			t.scroll()
			t.CurY = t.Rows - 1
		}
	}
	t.Grid[t.CurY][t.CurX] = Cell{
		Ch: ch, FG: t.CurFG, BG: t.CurBG,
		FGCustom: t.CurFGCustom, BGCustom: t.CurBGCustom,
		Bold: t.CurBold, Dim: t.CurDim, Inverse: t.CurInverse, Underline: t.CurUnderline,
	}
	t.CurX++
}

func (t *Terminal) newline() {
	t.CurX = 0
	t.CurY++
	if t.CurY >= t.Rows {
		t.scroll()
		t.CurY = t.Rows - 1
	}
}

func sgrToFG(code int) (string, bool) {
	switch {
	case code >= 30 && code <= 37:
		return []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"}[code-30], true
	case code >= 90 && code <= 97:
		return []string{"brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightMagenta", "brightCyan", "brightWhite"}[code-90], true
	}
	return "", false
}

func sgrToBG(code int) (string, bool) {
	switch {
	case code >= 40 && code <= 47:
		return []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"}[code-40], true
	case code >= 100 && code <= 107:
		return []string{"brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightMagenta", "brightCyan", "brightWhite"}[code-100], true
	}
	return "", false
}

func brightOf(name string) string {
	if b, ok := map[string]string{
		"black": "brightBlack", "red": "brightRed", "green": "brightGreen",
		"yellow": "brightYellow", "blue": "brightBlue", "magenta": "brightMagenta",
		"cyan": "brightCyan", "white": "brightWhite",
	}[name]; ok {
		return b
	}
	return name
}

func (t *Terminal) sgrReset() {
	t.CurFG = "foreground"
	t.CurBG = ""
	t.CurFGCustom = nil
	t.CurBGCustom = nil
	t.CurBold = false
	t.CurDim = false
	t.CurInverse = false
	t.CurUnderline = false
}

func (t *Terminal) handleSGR(params []int) {
	if len(params) == 0 {
		params = []int{0}
	}
	for i := 0; i < len(params); i++ {
		p := params[i]
		switch p {
		case 0:
			t.sgrReset()
		case 1:
			t.CurBold = true
		case 2:
			t.CurDim = true
		case 3, 4:
			// italic treated as underline-ish? keep underline only for 4
			if p == 4 {
				t.CurUnderline = true
			}
		case 7:
			t.CurInverse = true
		case 22:
			t.CurBold = false
			t.CurDim = false
		case 23:
		case 24:
			t.CurUnderline = false
		case 27:
			t.CurInverse = false
		case 39:
			t.CurFG = "foreground"
			t.CurFGCustom = nil
		case 49:
			t.CurBG = ""
			t.CurBGCustom = nil
		case 38, 48:
			// extended color: 38;5;N / 38;2;r;g;b
			isFG := p == 38
			if i+2 < len(params) && params[i+1] == 5 {
				n := params[i+2]
				c := Xterm256Color(n)
				if isFG {
					t.CurFG = ""
					t.CurFGCustom = &c
				} else {
					t.CurBG = ""
					t.CurBGCustom = &c
				}
				i += 2
			} else if i+4 < len(params) && params[i+1] == 2 {
				c := color.RGBA{R: uint8(params[i+2]), G: uint8(params[i+3]), B: uint8(params[i+4]), A: 0xff}
				if isFG {
					t.CurFG = ""
					t.CurFGCustom = &c
				} else {
					t.CurBG = ""
					t.CurBGCustom = &c
				}
				i += 4
			}
		default:
			if fg, ok := sgrToFG(p); ok {
				if t.CurBold {
					fg = brightOf(fg)
				}
				t.CurFG = fg
				t.CurFGCustom = nil
			} else if bg, ok := sgrToBG(p); ok {
				t.CurBG = bg
				t.CurBGCustom = nil
			}
		}
	}
}

func parseCSIParams(s string) []int {
	if s == "" {
		return nil
	}
	// strip single private prefix for numeric parse (? # etc. handled by caller)
	parts := strings.Split(s, ";")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			out = append(out, 0)
			continue
		}
		for len(p) > 0 && (p[0] < '0' || p[0] > '9') && p[0] != '-' {
			p = p[1:]
		}
		// trailing non-digits (e.g. "$")?
		for len(p) > 0 && (p[len(p)-1] < '0' || p[len(p)-1] > '9') {
			p = p[:len(p)-1]
		}
		if p == "" || p == "-" {
			out = append(out, 0)
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	return out
}

func intAt(params []int, i, def int) int {
	if i < len(params) && params[i] != 0 {
		return params[i]
	}
	return def
}

// Feed consumes raw PTY output (may contain ESC sequences).
func (t *Terminal) Feed(s string) {
	i := 0
	for i < len(s) {
		c := s[i]
		if c == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// CSI: ESC [ [?params] final  (final in @-~)
			j := i + 2
			for j < len(s) && (s[j] < '@' || s[j] > '~') {
				j++
			}
			if j >= len(s) {
				break
			}
			raw := s[i+2 : j]
			final := s[j]
			private := strings.HasPrefix(raw, "?")
			params := parseCSIParams(raw)
			switch {
			case final == 'm':
				t.handleSGR(params)
			case final == 'H' || final == 'f':
				r := intAt(params, 0, 1) - 1
				cc := intAt(params, 1, 1) - 1
				if r < 0 {
					r = 0
				}
				if cc < 0 {
					cc = 0
				}
				if r >= t.Rows {
					r = t.Rows - 1
				}
				if cc >= t.Cols {
					cc = t.Cols - 1
				}
				t.CurY, t.CurX = r, cc
			case final == 'A':
				t.CurY -= intAt(params, 0, 1)
				if t.CurY < 0 {
					t.CurY = 0
				}
			case final == 'B' || final == 'e':
				t.CurY += intAt(params, 0, 1)
				if t.CurY >= t.Rows {
					t.CurY = t.Rows - 1
				}
			case final == 'C' || final == 'a':
				t.CurX += intAt(params, 0, 1)
				if t.CurX >= t.Cols {
					t.CurX = t.Cols - 1
				}
			case final == 'D':
				t.CurX -= intAt(params, 0, 1)
				if t.CurX < 0 {
					t.CurX = 0
				}
			case final == 'E':
				t.CurY += intAt(params, 0, 1)
				if t.CurY >= t.Rows {
					t.CurY = t.Rows - 1
				}
				t.CurX = 0
			case final == 'F':
				t.CurY -= intAt(params, 0, 1)
				if t.CurY < 0 {
					t.CurY = 0
				}
				t.CurX = 0
			case final == 'G' || final == '`':
				cc := intAt(params, 0, 1) - 1
				if cc < 0 {
					cc = 0
				}
				if cc >= t.Cols {
					cc = t.Cols - 1
				}
				t.CurX = cc
			case final == 'd':
				r := intAt(params, 0, 1) - 1
				if r < 0 {
					r = 0
				}
				if r >= t.Rows {
					r = t.Rows - 1
				}
				t.CurY = r
			case final == 'J':
				mode := 0
				if len(params) > 0 {
					mode = params[0]
				}
				switch mode {
				case 2, 3:
					t.clearAll()
				case 0:
					for x := t.CurX; x < t.Cols; x++ {
						t.Grid[t.CurY][x] = Cell{Ch: ' ', FG: "foreground"}
					}
					for y := t.CurY + 1; y < t.Rows; y++ {
						t.resetRow(y)
					}
				case 1:
					for y := 0; y < t.CurY; y++ {
						t.resetRow(y)
					}
					for x := 0; x <= t.CurX && x < t.Cols; x++ {
						t.Grid[t.CurY][x] = Cell{Ch: ' ', FG: "foreground"}
					}
				}
			case final == 'K':
				mode := 0
				if len(params) > 0 {
					mode = params[0]
				}
				switch mode {
				case 0:
					for x := t.CurX; x < t.Cols; x++ {
						t.Grid[t.CurY][x] = Cell{Ch: ' ', FG: "foreground"}
					}
				case 1:
					for x := 0; x <= t.CurX && x < t.Cols; x++ {
						t.Grid[t.CurY][x] = Cell{Ch: ' ', FG: "foreground"}
					}
				case 2:
					t.resetRow(t.CurY)
				}
			case final == 'X': // erase N chars
				n := intAt(params, 0, 1)
				for k := 0; k < n && t.CurX+k < t.Cols; k++ {
					t.Grid[t.CurY][t.CurX+k] = Cell{Ch: ' ', FG: "foreground"}
				}
			case final == 'P': // delete N chars (shift left)
				n := intAt(params, 0, 1)
				row := t.Grid[t.CurY]
				copy(row[t.CurX:], row[min(t.CurX+n, t.Cols):])
				for x := t.Cols - n; x < t.Cols; x++ {
					if x >= 0 {
						row[x] = Cell{Ch: ' ', FG: "foreground"}
					}
				}
			case final == '@': // insert N blanks
				n := intAt(params, 0, 1)
				row := t.Grid[t.CurY]
				for x := t.Cols - 1; x >= t.CurX+n; x-- {
					row[x] = row[x-n]
				}
				for x := t.CurX; x < t.CurX+n && x < t.Cols; x++ {
					row[x] = Cell{Ch: ' ', FG: "foreground"}
				}
			case final == 'L': // insert N lines
				n := intAt(params, 0, 1)
				for k := 0; k < n; k++ {
					if t.CurY < t.Rows {
						copy(t.Grid[t.CurY+1:], t.Grid[t.CurY:t.Rows-1])
						t.resetRow(t.CurY)
					}
				}
			case final == 'M': // delete N lines
				n := intAt(params, 0, 1)
				for k := 0; k < n; k++ {
					if t.CurY < t.Rows {
						copy(t.Grid[t.CurY:], t.Grid[t.CurY+1:])
						t.resetRow(t.Rows - 1)
					}
				}
			case final == 'S': // scroll up N
				n := intAt(params, 0, 1)
				for k := 0; k < n; k++ {
					t.scroll()
				}
			case final == 'T': // scroll down N (approx: prepend lines)
				n := intAt(params, 0, 1)
				for k := 0; k < n; k++ {
					copy(t.Grid[1:], t.Grid[:t.Rows-1])
					t.resetRow(0)
				}
			case final == 'l' && private:
				// DEC private reset, e.g. ?25l handled via h/l? 25l is hide via 'l'? xterm uses CSI ? 25 l
				for _, p := range params {
					if p == 25 {
						// ambiguous without tracking h/l; 'l' => hide
						t.CursorVisible = false
					}
					if p == 1049 || p == 1047 || p == 1048 {
						t.clearAll()
						t.sgrReset()
					}
				}
			case final == 'h' && private:
				for _, p := range params {
					if p == 25 {
						t.CursorVisible = true
					}
					if p == 1049 || p == 1047 || p == 1048 {
						t.clearAll()
						t.sgrReset()
					}
				}
			case final == 'r':
				// scroll region: treat as full (ignore), but ESC[r without params = reset
			case final == 's':
				t.savedX, t.savedY = t.CurX, t.CurY
				t.savedFG, t.savedBG = t.CurFG, t.CurBG
			case final == 'u':
				t.CurX, t.CurY = t.savedX, t.savedY
				t.CurFG, t.CurBG = t.savedFG, t.savedBG
			}
			i = j + 1
			continue
		}
		// ESC 7 / ESC 8 save/restore, ESC c/M/D/E, ESC ( B charset, ESC = > keypad
		if c == 0x1b && i+1 < len(s) {
			n1 := s[i+1]
			switch n1 {
			case 'c':
				t.clearAll()
				t.sgrReset()
				i += 2
				continue
			case '7':
				t.savedX, t.savedY = t.CurX, t.CurY
				t.savedFG, t.savedBG = t.CurFG, t.CurBG
				i += 2
				continue
			case '8':
				t.CurX, t.CurY = t.savedX, t.savedY
				t.CurFG, t.CurBG = t.savedFG, t.savedBG
				i += 2
				continue
			case 'M':
				if t.CurY > 0 {
					t.CurY--
				}
				i += 2
				continue
			case 'D', 'E':
				t.newline()
				i += 2
				continue
			case '(', ')', '#', '=', '>':
				// charset/keypad sequences: ESC ( B etc. skip 3 bytes if present
				if i+2 < len(s) {
					i += 3
				} else {
					i += 2
				}
				continue
			case ']':
				// OSC ... BEL / ESC \
				j := i + 2
				for j < len(s) {
					if s[j] == 0x07 {
						j++
						break
					}
					if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				i = j
				continue
			case '[', 'P', 'X', '^', '_':
				i += 2
				continue
			}
		}
		if c == 0x1b {
			i++
			continue
		}
		// C1 controls 0x9b (CSI), 0x9c etc.
		if c == 0x9b {
			j := i + 1
			for j < len(s) && (s[j] < '@' || s[j] > '~') {
				j++
			}
			i = j + 1
			continue
		}
		switch c {
		case '\n':
			// LF: xterm newline = down (not CR). Our recordings come from PTY
			// in ONLCR mode so \n arrives as \r\n usually; keep pure down here.
			t.CurY++
			if t.CurY >= t.Rows {
				t.scroll()
				t.CurY = t.Rows - 1
			}
		case '\r':
			t.CurX = 0
		case '\b', 0x7f:
			if t.CurX > 0 {
				t.CurX--
			}
		case '\t':
			t.CurX = (t.CurX/8 + 1) * 8
			if t.CurX >= t.Cols {
				t.CurX = 0
				t.CurY++
				if t.CurY >= t.Rows {
					t.scroll()
					t.CurY = t.Rows - 1
				}
			}
		case 0x00, 0x07, 0x0b, 0x0c, 0x0e, 0x0f:
			// NUL/BEL/VT/FF/SO/SI: ignore
		default:
			r := rune(c)
			n := 1
			if c >= 0x80 {
				rest := s[i:]
				rr := []rune(rest)
				if len(rr) > 0 {
					r = rr[0]
					n = len(string(r))
				}
			}
			if r < 0x20 {
				i += n
				continue
			}
			t.put(r)
			i += n
			continue
		}
		i++
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Snapshot returns a deep copy of the grid.
func (t *Terminal) Snapshot() [][]Cell {
	out := make([][]Cell, t.Rows)
	for y := range t.Grid {
		out[y] = make([]Cell, t.Cols)
		copy(out[y], t.Grid[y])
	}
	return out
}

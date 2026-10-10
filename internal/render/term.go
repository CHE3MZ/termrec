// Package render converts recordings to animated GIFs without a browser.
// Pipeline: load .yml -> adjust delays -> emulate terminal grid per record
// -> draw RGBA (draw.go + framebox.go) -> watermark -> encode GIF (gif.go).
package render

import (
	"strconv"
	"strings"
)

// Cell is one terminal grid cell.
type Cell struct {
	Ch rune
	FG string // theme key, e.g. "foreground", "red", "brightWhite"
	BG string // theme key or "" for default background
	Bold bool
	Inverse bool
}

// Terminal is a minimal ANSI/SGR emulator covering what ref/ recordings emit.
// Handles \n \r \b \t, SGR 0/1/7/27/22, 30-37/90-97, 40-47/100-107, 39/49,
// ESC[2J/2K, ESC[H/f, ESC[{n}A/B/C/D/G, ESC[{r};{c}H.
type Terminal struct {
	Cols, Rows int
	Grid       [][]Cell
	CurX, CurY int
	CurFG      string
	CurBG      string
	CurBold    bool
	CurInverse bool
}

func NewTerminal(cols, rows int) *Terminal {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	t := &Terminal{Cols: cols, Rows: rows, CurFG: "foreground"}
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
	t.Grid[t.CurY][t.CurX] = Cell{Ch: ch, FG: t.CurFG, BG: t.CurBG, Bold: t.CurBold, Inverse: t.CurInverse}
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

func (t *Terminal) handleSGR(params []int) {
	if len(params) == 0 {
		params = []int{0}
	}
	for _, p := range params {
		switch p {
		case 0:
			t.CurFG = "foreground"
			t.CurBG = ""
			t.CurBold = false
			t.CurInverse = false
		case 1:
			t.CurBold = true
		case 22:
			t.CurBold = false
		case 7:
			t.CurInverse = true
		case 27:
			t.CurInverse = false
		case 39:
			t.CurFG = "foreground"
		case 49:
			t.CurBG = ""
		default:
			if fg, ok := sgrToFG(p); ok {
				if t.CurBold {
					// bold maps normal -> bright (xterm-like)
					if b, ok2 := map[string]string{
						"black": "brightBlack", "red": "brightRed", "green": "brightGreen",
						"yellow": "brightYellow", "blue": "brightBlue", "magenta": "brightMagenta",
						"cyan": "brightCyan", "white": "brightWhite",
					}[fg]; ok2 {
						fg = b
					}
				}
				t.CurFG = fg
			} else if bg, ok := sgrToBG(p); ok {
				t.CurBG = bg
			}
		}
	}
}

func parseCSIParams(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			out = append(out, 0)
			continue
		}
		// strip private prefixes like ? > =
		for len(p) > 0 && (p[0] < '0' || p[0] > '9') && p[0] != '-' {
			p = p[1:]
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
			// CSI: ESC [ params final
			j := i + 2
			for j < len(s) && (s[j] < '@' || s[j] > '~') {
				j++
			}
			if j >= len(s) {
				break
			}
			paramsStr := s[i+2 : j]
			final := s[j]
			params := parseCSIParams(paramsStr)
			switch final {
			case 'm':
				t.handleSGR(params)
			case 'H', 'f':
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
			case 'A':
				t.CurY -= intAt(params, 0, 1)
				if t.CurY < 0 {
					t.CurY = 0
				}
			case 'B':
				t.CurY += intAt(params, 0, 1)
				if t.CurY >= t.Rows {
					t.CurY = t.Rows - 1
				}
			case 'C':
				t.CurX += intAt(params, 0, 1)
				if t.CurX >= t.Cols {
					t.CurX = t.Cols - 1
				}
			case 'D':
				t.CurX -= intAt(params, 0, 1)
				if t.CurX < 0 {
					t.CurX = 0
				}
			case 'G':
				cc := intAt(params, 0, 1) - 1
				if cc < 0 {
					cc = 0
				}
				if cc >= t.Cols {
					cc = t.Cols - 1
				}
				t.CurX = cc
			case 'J':
				mode := 0
				if len(params) > 0 {
					mode = params[0]
				}
				switch mode {
				case 2:
					for y := range t.Grid {
						t.resetRow(y)
					}
					t.CurX, t.CurY = 0, 0
				case 0: // cursor to end
					for x := t.CurX; x < t.Cols; x++ {
						t.Grid[t.CurY][x] = Cell{Ch: ' ', FG: "foreground"}
					}
					for y := t.CurY + 1; y < t.Rows; y++ {
						t.resetRow(y)
					}
				case 1: // start to cursor
					for y := 0; y < t.CurY; y++ {
						t.resetRow(y)
					}
					for x := 0; x <= t.CurX && x < t.Cols; x++ {
						t.Grid[t.CurY][x] = Cell{Ch: ' ', FG: "foreground"}
					}
				}
			case 'K':
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
			}
			i = j + 1
			continue
		}
		if c == 0x1b && i+1 < len(s) && (s[i+1] == 'c' || s[i+1] == 'M' || s[i+1] == 'D' || s[i+1] == 'E') {
			// RIS / RI / IND / NEL (minimal)
			switch s[i+1] {
			case 'c':
				for y := range t.Grid {
					t.resetRow(y)
				}
				t.CurX, t.CurY = 0, 0
				t.CurFG, t.CurBG = "foreground", ""
				t.CurBold, t.CurInverse = false, false
			case 'M':
				if t.CurY > 0 {
					t.CurY--
				}
			case 'D', 'E':
				t.newline()
			}
			i += 2
			continue
		}
		if c == 0x1b {
			// Unknown escape: skip one char to avoid infinite loop on OSC etc.
			// Skip OSC (ESC ] ... BEL) fully.
			if i+1 < len(s) && s[i+1] == ']' {
				j := i + 2
				for j < len(s) && s[j] != 0x07 && s[j] != '\\' {
					j++
				}
				i = j + 1
				continue
			}
			i++
			continue
		}
		switch c {
		case '\n':
			t.newline()
		case '\r':
			t.CurX = 0
		case '\b', 0x7f:
			if t.CurX > 0 {
				t.CurX--
			}
		case '\t':
			t.CurX = (t.CurX/8+1)*8
			if t.CurX >= t.Cols {
				t.newline()
			}
		case 0x00, 0x07:
			// NUL / BEL: ignore
		default:
			// Decode UTF-8 rune
			r := rune(c)
			n := 1
			if c >= 0x80 {
				// decode multibyte
				rest := s[i:]
				rr := []rune(rest)
				if len(rr) > 0 {
					r = rr[0]
					n = len(string(r))
				}
			}
			if r < 0x20 {
				// other controls ignored
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

// Snapshot returns a deep copy of the grid.
func (t *Terminal) Snapshot() [][]Cell {
	out := make([][]Cell, t.Rows)
	for y := range t.Grid {
		out[y] = make([]Cell, t.Cols)
		copy(out[y], t.Grid[y])
	}
	return out
}

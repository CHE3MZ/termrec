package render

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/theme"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

func resolveCellColors(cell Cell, colors map[string]color.RGBA, defBG color.RGBA, defBGTrans bool) (fg color.RGBA, bg color.RGBA, bgTrans bool) {
	fg = colors["foreground"]
	if cell.FGCustom != nil {
		fg = *cell.FGCustom
	} else if cell.FG != "" {
		if c, ok := colors[cell.FG]; ok {
			fg = c
		}
	}
	if cell.BGCustom != nil {
		return fg, *cell.BGCustom, false
	}
	if cell.BG != "" {
		if c, ok := colors[cell.BG]; ok {
			return fg, c, false
		}
	}
	return fg, defBG, defBGTrans
}

// drawTerminal renders a grid snapshot to RGBA without frame.
// Transparent theme background stays transparent (no flattening here);
// GIF encoding preserves it (see gif.go).
func drawTerminal(grid [][]Cell, cfg config.Config, colors map[string]color.RGBA, transparentBG bool) *image.RGBA {
	rows := len(grid)
	cols := 0
	if rows > 0 {
		cols = len(grid[0])
	}
	cw, ch, ascent := cellMetrics(cfg)
	W, H := cols*cw, rows*ch
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	// Start fully transparent; fill only if theme bg is opaque.
	defBG := colors["background"]
	if !transparentBG {
		draw.Draw(img, img.Bounds(), &image.Uniform{defBG}, image.Point{}, draw.Src)
	}

	px := float64(cfg.FontSize)
	if px <= 0 {
		px = 12
	}
	face := monoFace(px)
	boldFace := face // synthesized by double-strike below
	_ = boldFace

	for y, row := range grid {
		for x, cell := range row {
			fg, bg, bgT := resolveCellColors(cell, colors, defBG, transparentBG)
			if cell.Inverse {
				fg, bg, bgT = bg, fg, false
				if cell.BG == "" && cell.BGCustom == nil && transparentBG {
					// inverse of default on transparent bg: fg becomes transparent-ish?
					// Keep readable: use opaque fg as bg.
					bgT = false
				}
			}
			if cell.Dim {
				fg.A = uint8(uint16(fg.A) * 2 / 3)
			}
			x0, y0 := x*cw, y*ch
			if !bgT {
				draw.Draw(img, image.Rect(x0, y0, x0+cw, y0+ch), &image.Uniform{bg}, image.Point{}, draw.Src)
			}
			if cell.Ch != ' ' && cell.Ch != 0 {
				d := &font.Drawer{
					Dst:  img,
					Src:  &image.Uniform{fg},
					Face: face,
					Dot:  fixed.P(x0, y0+ascent),
				}
				d.DrawString(string(cell.Ch))
				if cell.Bold {
					// double-strike 1px for bold (Go Mono has no bold variant bundled)
					d.Dot = fixed.P(x0+1, y0+ascent)
					d.DrawString(string(cell.Ch))
				}
				if cell.Underline {
					uy := y0 + ch - 3
					if uy < y0 {
						uy = y0
					}
					draw.Draw(img, image.Rect(x0, uy, x0+cw, uy+1), &image.Uniform{fg}, image.Point{}, draw.Over)
				}
			} else if cell.Underline {
				uy := y0 + ch - 3
				draw.Draw(img, image.Rect(x0, uy, x0+cw, uy+1), &image.Uniform{fg}, image.Point{}, draw.Over)
			}
		}
	}
	return img
}

// DrawFrame renders grid + cursor + frameBox into final RGBA (alpha preserved).
func DrawFrame(grid [][]Cell, curX, curY int, cursorVisible bool, cfg config.Config) *image.RGBA {
	colors, trans := theme.Map(cfg.Theme)
	termImg := drawTerminal(grid, cfg, colors, trans["background"])
	if cursorVisible {
		termImg = drawCursor(termImg, grid, curX, curY, cfg, colors, trans["background"])
	}
	return applyFrameBox(termImg, cfg, colors, trans["background"])
}

func drawCursor(termImg *image.RGBA, grid [][]Cell, curX, curY int, cfg config.Config, colors map[string]color.RGBA, termTrans bool) *image.RGBA {
	cw, ch, ascent := cellMetrics(cfg)
	if curY < 0 || curY >= len(grid) || curX < 0 {
		return termImg
	}
	if curX >= len(grid[curY]) {
		return termImg
	}
	cur := colors["cursor"]
	cell := grid[curY][curX]
	// char under cursor (for block: redraw glyph in bg color)
	var underCh rune
	if cell.Ch != 0 {
		underCh = cell.Ch
	} else {
		underCh = ' '
	}
	// cell bg (for glyph color under block cursor)
	_, cellBG, cellBGTrans := resolveCellColors(cell, colors, colors["background"], termTrans)
	glyphCol := cellBG
	if cellBGTrans {
		// over transparent: use frame body color approx (#1D1D1D) so glyph stays visible
		glyphCol = color.RGBA{0x1d, 0x1d, 0x1d, 0xff}
	}
	var r image.Rectangle
	switch cfg.CursorStyle {
	case "underline":
		r = image.Rect(curX*cw, (curY+1)*ch-2, (curX+1)*cw, (curY+1)*ch)
		draw.Draw(termImg, r, &image.Uniform{cur}, image.Point{}, draw.Over)
	case "bar":
		r = image.Rect(curX*cw, curY*ch, curX*cw+2, (curY+1)*ch)
		draw.Draw(termImg, r, &image.Uniform{cur}, image.Point{}, draw.Over)
	default: // block: solid cursor color, glyph punched in cell-bg color (xterm-like)
		r = image.Rect(curX*cw, curY*ch, (curX+1)*cw, (curY+1)*ch)
		draw.Draw(termImg, r, &image.Uniform{cur}, image.Point{}, draw.Src)
		if underCh != ' ' {
			px := float64(cfg.FontSize)
			if px <= 0 {
				px = 12
			}
			d := &font.Drawer{
				Dst:  termImg,
				Src:  &image.Uniform{glyphCol},
				Face: monoFace(px),
				Dot:  fixed.P(r.Min.X, r.Min.Y+ascent),
			}
			d.DrawString(string(underCh))
		}
	}
	return termImg
}

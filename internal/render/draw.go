package render

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/theme"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Metrics for the embedded bitmap font.
const (
	baseCellW = 7
	baseCellH = 13
)

// cellSize computes cell dims from fontSize/lineHeight/letterSpacing.
// basicfont is fixed 7x13; fontSize scales linearly around 12.
func cellSize(cfg config.Config) (w, h int) {
	scale := float64(cfg.FontSize) / 12.0
	if scale <= 0 {
		scale = 1
	}
	w = int(float64(baseCellW)*scale) + cfg.LetterSpacing
	h = int(float64(baseCellH)*scale*cfg.LineHeight + 0.5)
	if w < 4 {
		w = 4
	}
	if h < 6 {
		h = 6
	}
	return w, h
}

// drawTerminal renders a grid snapshot to RGBA without frame.
func drawTerminal(grid [][]Cell, cfg config.Config, colors map[string]color.RGBA, transparentBG bool) *image.RGBA {
	rows := len(grid)
	cols := 0
	if rows > 0 {
		cols = len(grid[0])
	}
	cw, ch := cellSize(cfg)
	W, H := cols*cw, rows*ch
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	bg := colors["foreground"] // placeholder
	_ = bg
	bgColor := colors["black"]
	if c, ok := colors["background"]; ok {
		bgColor = c
	}
	if transparentBG {
		// GIF has no alpha: flatten transparent onto white (documented).
		bgColor = color.RGBA{0xff, 0xff, 0xff, 0xff}
	}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	face := basicfont.Face7x13
	fgDefault := colors["foreground"]
	for y, row := range grid {
		for x, cell := range row {
			fg := fgDefault
			if c, ok := colors[cell.FG]; ok {
				fg = c
			}
			var cellBG *color.RGBA
			if cell.BG != "" {
				if c, ok := colors[cell.BG]; ok {
					cellBG = &c
				}
			}
			if cell.Inverse {
				fg, cellBG = invert(fg, cellBG, bgColor)
			}
			x0, y0 := x*cw, y*ch
			if cellBG != nil {
				draw.Draw(img, image.Rect(x0, y0, x0+cw, y0+ch), &image.Uniform{*cellBG}, image.Point{}, draw.Src)
			}
			if cell.Ch != ' ' && cell.Ch != 0 {
				d := &font.Drawer{
					Dst:  img,
					Src:  &image.Uniform{fg},
					Face: face,
					Dot:  fixed.P(x0, y0+11*ch/13), // baseline approx
				}
				d.DrawString(string(cell.Ch))
			}
		}
	}
	// Cursor is drawn by caller (needs position); see DrawFrame.
	return img
}

func invert(fg color.RGBA, bg *color.RGBA, defBG color.RGBA) (color.RGBA, *color.RGBA) {
	actualBG := defBG
	if bg != nil {
		actualBG = *bg
	}
	nbg := fg
	nfg := actualBG
	return nfg, &nbg
}

// DrawFrame renders grid + cursor + frameBox into final RGBA.
func DrawFrame(grid [][]Cell, curX, curY int, cfg config.Config) *image.RGBA {
	colors, trans := theme.Map(cfg.Theme)
	termImg := drawTerminal(grid, cfg, colors, trans["background"])
	termImg = drawCursor(termImg, grid, curX, curY, cfg, colors)
	return applyFrameBox(termImg, cfg, colors)
}

func drawCursor(termImg *image.RGBA, grid [][]Cell, curX, curY int, cfg config.Config, colors map[string]color.RGBA) *image.RGBA {
	cw, ch := cellSize(cfg)
	if curY < 0 || curY >= len(grid) || curX < 0 {
		return termImg
	}
	if curX >= len(grid[curY]) {
		return termImg
	}
	cur := colors["cursor"]
	var r image.Rectangle
	switch cfg.CursorStyle {
	case "underline":
		r = image.Rect(curX*cw, (curY+1)*ch-2, (curX+1)*cw, (curY+1)*ch)
	case "bar":
		r = image.Rect(curX*cw, curY*ch, curX*cw+2, (curY+1)*ch)
	default: // block
		r = image.Rect(curX*cw, curY*ch, (curX+1)*cw, (curY+1)*ch)
		// block: invert-ish overlay with 60% cursor color
		overlay := image.NewRGBA(r.Sub(r.Min))
		draw.Draw(overlay, overlay.Bounds(), &image.Uniform{color.RGBA{cur.R, cur.G, cur.B, 0x99}}, image.Point{}, draw.Src)
		draw.Draw(termImg, r, overlay, image.Point{}, draw.Over)
		return termImg
	}
	draw.Draw(termImg, r, &image.Uniform{cur}, image.Point{}, draw.Src)
	return termImg
}

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
// GIF encoding preserves it (see gifutil.go).
func drawTerminal(grid [][]Cell, cfg config.Config, colors map[string]color.RGBA, transparentBG bool, ss int) *image.RGBA {
	rows := len(grid)
	cols := 0
	if rows > 0 {
		cols = len(grid[0])
	}
	if ss <= 0 {
		ss = 1
	}
	// Exact-multiple grid (see scaledMetrics): never re-measure at 2x.
	cw, ch, ascent := scaledMetrics(cfg, ss)
	W, H := cols*cw, rows*ch
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	// Start fully transparent; fill only if theme bg is opaque.
	defBG := colors["background"]
	if !transparentBG {
		draw.Draw(img, img.Bounds(), &image.Uniform{defBG}, image.Point{}, draw.Src)
	}

	px := float64(cfg.FontSize * ss)
	if px <= 0 {
		px = 12
	}
	// Per-rune fallback chain (primary, Go Mono, Go Regular, basicfont):
	// Chromium falls back per glyph, so a single face leaves tofu for
	// CJK/emoji/symbols it doesn't cover. Baseline stays the primary's.
	faces := monoFacesFor(cfg.FontFamily, px)

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
				// Scale premultiplied channels together (RGB <= A invariant).
				fg.R = uint8(uint16(fg.R) * 2 / 3)
				fg.G = uint8(uint16(fg.G) * 2 / 3)
				fg.B = uint8(uint16(fg.B) * 2 / 3)
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
					Face: faceForRune(faces, cell.Ch),
					Dot:  fixed.P(x0, y0+ascent),
				}
				d.DrawString(string(cell.Ch))
				if cell.Bold {
					// double-strike for bold (no bundled bold variant)
					d.Dot = fixed.P(x0+ss, y0+ascent)
					d.DrawString(string(cell.Ch))
				}
				if cell.Underline {
					uy := y0 + ch - 3*ss
					if uy < y0 {
						uy = y0
					}
					draw.Draw(img, image.Rect(x0, uy, x0+cw, uy+ss), &image.Uniform{fg}, image.Point{}, draw.Over)
				}
			} else if cell.Underline {
				uy := y0 + ch - 3*ss
				draw.Draw(img, image.Rect(x0, uy, x0+cw, uy+ss), &image.Uniform{fg}, image.Point{}, draw.Over)
			}
			if cell.Strikethrough {
				sy := y0 + ch/2 - ss/2
				if sy < y0 {
					sy = y0
				}
				draw.Draw(img, image.Rect(x0, sy, x0+cw, sy+ss), &image.Uniform{fg}, image.Point{}, draw.Over)
			}
		}
	}
	return img
}

// DrawFrame renders grid + cursor + frameBox into final RGBA (alpha preserved).
// The frame composes at renderSS x scale and is downscaled with area
// averaging, so text, dots, corners and gradients come out smooth.
// Compose geometry is exactly renderSS x the 1x layout (see scaledMetrics),
// so the downscale ratio is integral and flat fills survive bit-exact.
func DrawFrame(grid [][]Cell, curX, curY int, cursorVisible bool, cfg config.Config) *image.RGBA {
	colors, trans := theme.Map(cfg.Theme)
	termImg := drawTerminal(grid, cfg, colors, trans["background"], renderSS)
	if cursorVisible {
		termImg = drawCursor(termImg, grid, curX, curY, cfg, colors, trans["background"], renderSS)
	}
	big := applyFrameBox(termImg, cfg, colors, trans["background"], renderSS)
	// NOTE: no flattening — transparency is the default. The outer margin
	// area stays transparent and the shadow glow is a dark alpha gradient,
	// so frames sit cleanly on any background.
	// Target 1x dims from the unscaled layout.
	cw1, ch1 := cellSize(cfg)
	rows := len(grid)
	cols := 0
	if rows > 0 {
		cols = len(grid[0])
	}
	target := frameOuterSize(cfg, cols*cw1, rows*ch1)
	if target.Dx() <= 0 || target.Dy() <= 0 {
		return big
	}
	if big.Bounds().Dx() == target.Dx() && big.Bounds().Dy() == target.Dy() {
		return big
	}
	return downscaleRGBA(big, target.Dx(), target.Dy())
}

// downscaleRGBA area-averages src to w x h with premultiplied alpha,
// preserving transparency and smoothing supersampled edges.
func downscaleRGBA(src *image.RGBA, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if w <= 0 || h <= 0 || sw <= 0 || sh <= 0 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	rx := float64(sw) / float64(w)
	ry := float64(sh) / float64(h)
	for y := 0; y < h; y++ {
		y0 := int(float64(y) * ry)
		y1 := int(float64(y+1) * ry)
		if y1 <= y0 {
			y1 = y0 + 1
		}
		if y1 > sh {
			y1 = sh
		}
		for x := 0; x < w; x++ {
			x0 := int(float64(x) * rx)
			x1 := int(float64(x+1) * rx)
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if x1 > sw {
				x1 = sw
			}
			var r, g, b, a float64
			var n float64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					// Pix holds premultiplied values: average them directly.
					off := src.PixOffset(sb.Min.X+sx, sb.Min.Y+sy)
					r += float64(src.Pix[off])
					g += float64(src.Pix[off+1])
					b += float64(src.Pix[off+2])
					a += float64(src.Pix[off+3])
					n++
				}
			}
			if n == 0 || a == 0 {
				continue
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r/n + 0.5),
				G: uint8(g/n + 0.5),
				B: uint8(b/n + 0.5),
				A: uint8(a/n + 0.5),
			})
		}
	}
	return dst
}

func drawCursor(termImg *image.RGBA, grid [][]Cell, curX, curY int, cfg config.Config, colors map[string]color.RGBA, termTrans bool, ss int) *image.RGBA {
	if ss <= 0 {
		ss = 1
	}
	cw, ch, ascent := scaledMetrics(cfg, ss)
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
		r = image.Rect(curX*cw, (curY+1)*ch-2*ss, (curX+1)*cw, (curY+1)*ch)
		draw.Draw(termImg, r, &image.Uniform{cur}, image.Point{}, draw.Over)
	case "bar":
		r = image.Rect(curX*cw, curY*ch, curX*cw+2*ss, (curY+1)*ch)
		draw.Draw(termImg, r, &image.Uniform{cur}, image.Point{}, draw.Over)
	default: // block: solid cursor color, glyph punched in cell-bg color (xterm-like)
		r = image.Rect(curX*cw, curY*ch, (curX+1)*cw, (curY+1)*ch)
		draw.Draw(termImg, r, &image.Uniform{cur}, image.Point{}, draw.Src)
		if underCh != ' ' {
			px := float64(cfg.FontSize * ss)
			if px <= 0 {
				px = 12
			}
			d := &font.Drawer{
				Dst:  termImg,
				Src:  &image.Uniform{glyphCol},
				Face: faceForRune(monoFacesFor(cfg.FontFamily, px), underCh),
				Dot:  fixed.P(r.Min.X, r.Min.Y+ascent),
			}
			d.DrawString(string(underCh))
		}
	}
	return termImg
}

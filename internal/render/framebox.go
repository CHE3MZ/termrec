package render

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	"github.com/CHE3MZ/termrec/internal/config"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func frameType(cfg config.Config) string {
	if cfg.FrameBox.Type == nil {
		return "null"
	}
	return strings.ToLower(strings.TrimSpace(*cfg.FrameBox.Type))
}

func frameTitle(cfg config.Config) string {
	if cfg.FrameBox.Title == nil {
		return ""
	}
	return *cfg.FrameBox.Title
}

func styleHas(cfg config.Config, key, val string) bool {
	for k, v := range cfg.FrameBox.Style {
		if strings.EqualFold(k, key) && strings.Contains(strings.ToLower(v), strings.ToLower(val)) {
			return true
		}
	}
	return false
}

func styleGet(cfg config.Config, key string) string {
	for k, v := range cfg.FrameBox.Style {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// applyFrameBox wraps term image with null/window/floating/solid frames.
// Supports style overrides: boxShadow:none, margin:0, backgroundColor, border (ignored, documented).
func applyFrameBox(termImg *image.RGBA, cfg config.Config, colors map[string]color.RGBA) *image.RGBA {
	ft := frameType(cfg)
	title := frameTitle(cfg)

	margin := 12
	if styleHas(cfg, "margin", "0") {
		margin = 0
	}
	shadow := true
	if styleHas(cfg, "boxShadow", "none") {
		shadow = false
	}
	if ft == "null" {
		bgName := styleGet(cfg, "backgroundColor")
		var bg color.RGBA
		if bgName != "" {
			// tiny named support
			switch strings.ToLower(strings.TrimSpace(bgName)) {
			case "black":
				bg = color.RGBA{0, 0, 0, 0xff}
			case "white":
				bg = color.RGBA{0xff, 0xff, 0xff, 0xff}
			default:
				bg = colors["black"]
				if c, ok := colors[bgName]; ok {
					bg = c
				}
			}
		} else {
			bg = colors["background"]
			// transparent flattened to white for null frame too
			if bg.A == 0 {
				bg = color.RGBA{0xff, 0xff, 0xff, 0xff}
			}
			// if theme bg is dark-ish keep it, else white already
			if bg == (color.RGBA{}) {
				bg = color.RGBA{0, 0, 0, 0xff}
			}
		}
		if margin == 0 {
			return termImg
		}
		W := termImg.Bounds().Dx() + margin*2
		H := termImg.Bounds().Dy() + margin*2
		out := image.NewRGBA(image.Rect(0, 0, W, H))
		draw.Draw(out, out.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
		draw.Draw(out, image.Rect(margin, margin, margin+termImg.Bounds().Dx(), margin+termImg.Bounds().Dy()), termImg, image.Point{}, draw.Over)
		return out
	}

	// window/floating/solid share: title bar + padding + optional shadow
	titleH := 0
	if title != "" {
		titleH = 28
	} else if ft == "solid" {
		titleH = 0
	} else {
		titleH = 12 // small top rounding padding
	}
	pad := 12
	if margin == 0 {
		pad = 0
	}
	innerW := termImg.Bounds().Dx()
	innerH := termImg.Bounds().Dy()
	W := innerW + pad*2
	H := innerH + pad*2 + titleH
	shadowPad := 0
	if shadow && ft != "solid" {
		shadowPad = 16
		W += shadowPad
		H += shadowPad
	}
	out := image.NewRGBA(image.Rect(0, 0, W, H))
	// page bg: light gray for floating/window, theme-ish for solid
	pageBG := color.RGBA{0xe8, 0xe8, 0xe8, 0xff}
	if ft == "solid" {
		pageBG = color.RGBA{0x2b, 0x2b, 0x2b, 0xff}
	}
	draw.Draw(out, out.Bounds(), &image.Uniform{pageBG}, image.Point{}, draw.Src)

	ox, oy := pad, pad+titleH
	if shadow && ft != "solid" {
		// simple soft shadow: dark translucent rect offset
		sh := image.Rect(ox+6, oy+8, ox+6+innerW, oy+8+innerH+titleHOffset(titleH))
		draw.Draw(out, sh, &image.Uniform{color.RGBA{0, 0, 0, 0x55}}, image.Point{}, draw.Over)
		_ = shadowPad
	}
	// card bg (terminal surround)
	cardBG := color.RGBA{0x1e, 0x1e, 0x1e, 0xff}
	if len(termImg.Pix) > 0 {
		// sample top-left of term image as card hint? keep dark for contrast
	}
	card := image.Rect(ox, oy-titleH, ox+innerW, oy+innerH)
	if title == "" && ft != "solid" {
		card = image.Rect(ox, oy, ox+innerW, oy+innerH)
	}
	draw.Draw(out, card, &image.Uniform{cardBG}, image.Point{}, draw.Src)

	if title != "" {
		drawTitleBar(out, image.Rect(ox, oy-titleH, ox+innerW, oy), title)
	}
	draw.Draw(out, image.Rect(ox, oy, ox+innerW, oy+innerH), termImg, image.Point{}, draw.Over)
	_ = floatingExtra(ft)
	return out
}

func titleHOffset(h int) int { return 0 }

func floatingExtra(ft string) int { return 0 }

func drawTitleBar(dst *image.RGBA, r image.Rectangle, title string) {
	// macOS-like bar
	draw.Draw(dst, r, &image.Uniform{color.RGBA{0x3a, 0x3a, 0x3a, 0xff}}, image.Point{}, draw.Src)
	// traffic dots
	dots := []color.RGBA{{0xff, 0x5f, 0x57, 0xff}, {0xfe, 0xbd, 0x2e, 0xff}, {0x28, 0xc8, 0x40, 0xff}}
	for i, c := range dots {
		cx := r.Min.X + 14 + i*18
		cy := (r.Min.Y + r.Max.Y) / 2
		fillCircle(dst, cx, cy, 6, c)
	}
	// centered title (basicfont)
	face := basicfont.Face7x13
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	ty := r.Min.Y + 19
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0xdd, 0xdd, 0xdd, 0xff}}, Face: face, Dot: fixed.P(tx, ty)}
	d.DrawString(title)
}

func fillCircle(dst *image.RGBA, cx, cy, rad int, c color.RGBA) {
	for y := -rad; y <= rad; y++ {
		for x := -rad; x <= rad; x++ {
			if x*x+y*y <= rad*rad {
				px, py := cx+x, cy+y
				if image.Pt(px, py).In(dst.Bounds()) {
					dst.Set(px, py, c)
				}
			}
		}
	}
}

package render

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/theme"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// CSS-exact values from terminalizer-player (see ref docs):
// radius 6px; window titlebar 22px #E8E8E8; floating titlebar 34px;
// solid title margin 15px; body padding window 10px / floating+solid 20px;
// margin 18px; shadow 0 0 18px #B3B3B3; frame/body bg #1D1D1D.
const (
	frameRadius      = 6
	frameMargin      = 18
	shadowBlur       = 18
	shadowColorR     = 0xB3
	shadowColorG     = 0xB3
	shadowColorB     = 0xB3
	bodyBG_R         = 0x1D
	bodyBG_G         = 0x1D
	bodyBG_B         = 0x1D
	windowPad        = 10
	floatingPad      = 20
	windowTitleH     = 22
	floatingTitleH   = 34
	solidTitleMargin = 15
)

func frameType(cfg config.Config) string {
	if cfg.FrameBox.Type == nil {
		return "null"
	}
	t := strings.ToLower(strings.TrimSpace(*cfg.FrameBox.Type))
	if t == "" || t == "null" || t == "none" {
		return "null"
	}
	return t
}

func frameTitle(cfg config.Config) string {
	if cfg.FrameBox.Title == nil {
		return ""
	}
	return *cfg.FrameBox.Title
}

func styleGet(cfg config.Config, key string) string {
	for k, v := range cfg.FrameBox.Style {
		if strings.EqualFold(strings.TrimSpace(k), key) {
			return v
		}
	}
	return ""
}

func styleHas(cfg config.Config, key, val string) bool {
	return strings.Contains(strings.ToLower(styleGet(cfg, key)), strings.ToLower(val))
}

func parseMarginPx(cfg config.Config) int {
	m := strings.TrimSpace(styleGet(cfg, "margin"))
	if m == "" {
		return frameMargin
	}
	low := strings.ToLower(m)
	if low == "0" || low == "0px" || strings.HasPrefix(low, "0 ") {
		return 0
	}
	digits := ""
	for _, r := range low {
		if r >= '0' && r <= '9' {
			digits += string(r)
		} else if digits != "" {
			break
		}
	}
	if digits == "" {
		return frameMargin
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return frameMargin
	}
	return n
}

func shadowEnabled(cfg config.Config) bool {
	return !styleHas(cfg, "boxShadow", "none")
}

// frameBG returns the frame/body background: style background/backgroundColor
// override, else #1D1D1D (CSS-exact for window/floating/solid).
func frameBG(cfg config.Config, colors map[string]color.RGBA) color.RGBA {
	if v := styleGet(cfg, "backgroundColor"); strings.TrimSpace(v) != "" {
		if c, t, err := theme.Parse(v); err == nil && !t {
			return c
		}
	}
	if v := styleGet(cfg, "background"); strings.TrimSpace(v) != "" {
		if c, t, err := theme.Parse(v); err == nil && !t {
			return c
		}
	}
	return color.RGBA{bodyBG_R, bodyBG_G, bodyBG_B, 0xff}
}

// applyFrameBox wraps term image with null/window/floating/solid frames.
// The outer margin area stays transparent (default) so GIFs are transparent
// by default; corners are rounded (6px) with transparency.
func applyFrameBox(termImg *image.RGBA, cfg config.Config, colors map[string]color.RGBA, termTrans bool) *image.RGBA {
	ft := frameType(cfg)
	title := frameTitle(cfg)
	margin := parseMarginPx(cfg)

	if ft == "null" {
		bgName := styleGet(cfg, "backgroundColor")
		if bgName == "" {
			bgName = styleGet(cfg, "background")
		}
		if strings.TrimSpace(bgName) == "" {
			// Bare terminal: transparency preserved (default transparent).
			// Apply margin as transparent padding so layout matches framed modes.
			if margin == 0 {
				return termImg
			}
			W := termImg.Bounds().Dx() + margin*2
			H := termImg.Bounds().Dy() + margin*2
			out := image.NewRGBA(image.Rect(0, 0, W, H))
			draw.Draw(out, image.Rect(margin, margin, margin+termImg.Bounds().Dx(), margin+termImg.Bounds().Dy()), termImg, image.Point{}, draw.Over)
			return out
		}
		bg := colors["background"]
		if c, t, err := theme.Parse(bgName); err == nil && !t {
			bg = c
		} else if t {
			bg = color.RGBA{}
		}
		if margin == 0 {
			// fill behind terminal
			out := image.NewRGBA(termImg.Bounds())
			if bg.A != 0 {
				draw.Draw(out, out.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
			}
			draw.Draw(out, out.Bounds(), termImg, image.Point{}, draw.Over)
			return out
		}
		W := termImg.Bounds().Dx() + margin*2
		H := termImg.Bounds().Dy() + margin*2
		out := image.NewRGBA(image.Rect(0, 0, W, H))
		if bg.A != 0 {
			draw.Draw(out, out.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
		}
		draw.Draw(out, image.Rect(margin, margin, margin+termImg.Bounds().Dx(), margin+termImg.Bounds().Dy()), termImg, image.Point{}, draw.Over)
		return out
	}

	bg := frameBG(cfg, colors)
	pad := floatingPad
	titleH := 0
	switch ft {
	case "window":
		pad = windowPad
		if title != "" {
			titleH = windowTitleH
		}
	case "floating":
		pad = floatingPad
		if title != "" {
			titleH = floatingTitleH
		} else {
			titleH = 0
		}
	case "solid":
		pad = floatingPad
		if title != "" {
			titleH = -1 // computed from text metrics + margins
		} else {
			titleH = 0
		}
	default: // unknown type -> treat as floating without title
		pad = floatingPad
		titleH = 0
		title = ""
	}
	// style padding override (e.g. padding: 10px)
	if v := styleGet(cfg, "padding"); strings.TrimSpace(v) != "" {
		if n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(v)), "px")); err == nil && n >= 0 {
			pad = n
		}
	}
	if margin == 0 {
		// still keep internal padding; margin only affects outer transparent area
	}

	innerW := termImg.Bounds().Dx()
	innerH := termImg.Bounds().Dy()

	// Solid title block height: 15px margins + ~20px text.
	solidTitleH := 0
	if titleH == -1 {
		solidTitleH = solidTitleMargin*2 + 20
		titleH = solidTitleH
	}

	cardW := innerW + pad*2
	cardH := innerH + pad*2 + titleH

	// 1. Build opaque card (rounded).
	card := image.NewRGBA(image.Rect(0, 0, cardW, cardH))
	draw.Draw(card, card.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	switch ft {
	case "window":
		if title != "" {
			drawWindowTitleBar(card, image.Rect(0, 0, cardW, windowTitleH), title)
			// bottom border of titlebar
			draw.Draw(card, image.Rect(0, windowTitleH-1, cardW, windowTitleH), &image.Uniform{color.RGBA{0xB1, 0xAE, 0xB1, 0xff}}, image.Point{}, draw.Src)
		}
	case "floating":
		if title != "" {
			drawFloatingTitleBar(card, image.Rect(0, 0, cardW, floatingTitleH), title)
		}
	case "solid":
		if title != "" {
			drawSolidTitle(card, image.Rect(0, 0, cardW, titleH), title)
		}
	}
	// terminal body over card bg
	oy := pad + titleH
	draw.Draw(card, image.Rect(pad, oy, pad+innerW, oy+innerH), termImg, image.Point{}, draw.Over)

	// window 1px border (#B3B3B3 or style.border override)
	borderW, borderCol := parseBorder(cfg)
	if ft == "window" && borderW > 0 {
		strokeRoundedRect(card, borderW, borderCol)
	}

	// rounded corners -> transparent
	radius := frameRadius
	if v := styleGet(cfg, "borderRadius"); strings.TrimSpace(v) != "" {
		low := strings.ToLower(strings.TrimSpace(v))
		if low == "0" || low == "0px" || low == "none" {
			radius = 0
		}
	}
	if radius > 0 {
		applyRoundedMask(card, radius)
	}

	// watermark is positioned relative to the frame (CSS: .terminalizer-frame prepend)
	card = applyWatermark(card, cfg)

	// 2. Composite onto transparent canvas with margin + soft shadow.
	W := cardW + margin*2
	H := cardH + margin*2
	if shadowEnabled(cfg) {
		W += shadowBlur
		H += shadowBlur
	}
	out := image.NewRGBA(image.Rect(0, 0, W, H))
	ox := margin
	oy2 := margin
	if shadowEnabled(cfg) {
		drawSoftShadow(out, image.Rect(ox, oy2, ox+cardW, oy2+cardH), radius)
		// shadowBlur/2 offset keeps glow centered-ish like CSS `0 0 18px`
	}
	draw.Draw(out, image.Rect(ox, oy2, ox+cardW, oy2+cardH), card, image.Point{}, draw.Over)
	return out
}

func parseBorder(cfg config.Config) (int, color.RGBA) {
	def := color.RGBA{0xB3, 0xB3, 0xB3, 0xff}
	v := strings.TrimSpace(styleGet(cfg, "border"))
	if v == "" {
		// window default per CSS; other frames have no border
		return 1, def
	}
	low := strings.ToLower(v)
	if low == "none" || strings.HasPrefix(low, "0px") || low == "0" {
		// allow "0px black solid"
		if strings.HasPrefix(low, "0") {
			return 0, def
		}
		return 0, def
	}
	// "1px solid #B3B3B3" / "1px #B3B3B3 solid"
	width := 1
	for _, tok := range strings.Fields(low) {
		d := ""
		for _, r := range tok {
			if r >= '0' && r <= '9' {
				d += string(r)
			} else {
				break
			}
		}
		if d != "" {
			if n, err := strconv.Atoi(d); err == nil {
				width = n
				break
			}
		}
	}
	col := def
	for _, tok := range strings.Fields(v) {
		if strings.HasPrefix(tok, "#") || strings.HasPrefix(strings.ToLower(tok), "rgb") || strings.HasPrefix(strings.ToLower(tok), "hsl") {
			if c, t, err := theme.Parse(tok); err == nil && !t {
				col = c
				break
			}
		}
	}
	// named color as last resort
	for _, tok := range strings.Fields(v) {
		if c, t, err := theme.Parse(tok); err == nil && !t {
			// skip keywords like solid
			if strings.EqualFold(tok, "solid") || strings.EqualFold(tok, "dashed") || strings.EqualFold(tok, "dotted") {
				continue
			}
			col = c
			break
		}
	}
	return width, col
}

// --- rounded corners + shadow ---

func applyRoundedMask(img *image.RGBA, r int) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	for y := 0; y < r; y++ {
		for x := 0; x < r; x++ {
			if x*x+y*y >= r*r {
				img.SetRGBA(x, y, color.RGBA{})
				img.SetRGBA(W-1-x, y, color.RGBA{})
				img.SetRGBA(x, H-1-y, color.RGBA{})
				img.SetRGBA(W-1-x, H-1-y, color.RGBA{})
			}
		}
	}
}

func strokeRoundedRect(img *image.RGBA, w int, c color.RGBA) {
	if w <= 0 {
		return
	}
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	for i := 0; i < w; i++ {
		// top/bottom rows
		for x := i; x < W-i; x++ {
			img.SetRGBA(x, i, c)
			img.SetRGBA(x, H-1-i, c)
		}
		for y := i; y < H-i; y++ {
			img.SetRGBA(i, y, c)
			img.SetRGBA(W-1-i, y, c)
		}
	}
}

// drawSoftShadow paints a blurred #B3B3B3 glow of rect r onto dst (transparent bg).
// Simple separable box blur of a rounded-rect silhouette; radius ~ shadowBlur/3.
func drawSoftShadow(dst *image.RGBA, r image.Rectangle, radius int) {
	// silhouette
	sw, sh := r.Dx(), r.Dy()
	sil := image.NewAlpha(image.Rect(0, 0, sw, sh))
	// fill rounded rect silhouette (approx: full rect minus corners)
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			a := uint8(0xff)
			if radius > 0 {
				// corner cutout
				inCorner := func(cx, cy int) bool {
					dx := x - cx
					dy := y - cy
					return dx*dx+dy*dy < radius*radius
				}
				corner := false
				switch {
				case x < radius && y < radius:
					corner = !inCorner(radius, radius)
				case x >= sw-radius && y < radius:
					corner = !inCorner(sw-1-radius, radius)
				case x < radius && y >= sh-radius:
					corner = !inCorner(radius, sh-1-radius)
				case x >= sw-radius && y >= sh-radius:
					corner = !inCorner(sw-1-radius, sh-1-radius)
				}
				if corner {
					continue
				}
			}
			sil.SetAlpha(x, y, color.Alpha{A: a})
		}
	}
	blurred := boxBlurAlpha(sil, 6)
	blurred = boxBlurAlpha(blurred, 6)
	// stamp with low alpha color
	scol := color.RGBA{shadowColorR, shadowColorG, shadowColorB, 0}
	_ = scol
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			a := blurred.AlphaAt(x, y).A
			if a == 0 {
				continue
			}
			// CSS 18px soft shadow: scale to ~35% peak
			na := uint16(a) * 90 / 255
			if na == 0 {
				continue
			}
			dx, dy := r.Min.X+x, r.Min.Y+y
			if !image.Pt(dx, dy).In(dst.Bounds()) {
				continue
			}
			// over-composite gray at na alpha
			do, dg, db, da := dst.RGBAAt(dx, dy).RGBA()
			_ = do
			sa := float64(na) / 255
			dr := float64(uint8(do>>8))*(1-sa) + float64(shadowColorR)*sa
			dg2 := float64(uint8(dg>>8))*(1-sa) + float64(shadowColorG)*sa
			db2 := float64(uint8(db>>8))*(1-sa) + float64(shadowColorB)*sa
			da2 := float64(uint8(da>>8)) + sa*(255-float64(uint8(da>>8)))
			dst.SetRGBA(dx, dy, color.RGBA{R: uint8(dr), G: uint8(dg2), B: uint8(db2), A: uint8(da2)})
		}
	}
}

func boxBlurAlpha(src *image.Alpha, r int) *image.Alpha {
	b := src.Bounds()
	W, H := b.Dx(), b.Dy()
	tmp := image.NewAlpha(b)
	out := image.NewAlpha(b)
	// horizontal
	for y := 0; y < H; y++ {
		acc := 0
		for x := -r; x <= r; x++ {
			xx := x
			if xx < 0 {
				xx = 0
			}
			if xx >= W {
				xx = W - 1
			}
			acc += int(src.AlphaAt(xx, y).A)
		}
		for x := 0; x < W; x++ {
			tmp.SetAlpha(x, y, color.Alpha{A: uint8(acc / (2*r + 1))})
			xo := x - r
			if xo < 0 {
				xo = 0
			}
			xn := x + r + 1
			if xn >= W {
				xn = W - 1
			}
			acc += int(src.AlphaAt(xn, y).A) - int(src.AlphaAt(xo, y).A)
		}
	}
	// vertical
	for x := 0; x < W; x++ {
		acc := 0
		for y := -r; y <= r; y++ {
			yy := y
			if yy < 0 {
				yy = 0
			}
			if yy >= H {
				yy = H - 1
			}
			acc += int(tmp.AlphaAt(x, yy).A)
		}
		for y := 0; y < H; y++ {
			out.SetAlpha(x, y, color.Alpha{A: uint8(acc / (2*r + 1))})
			yo := y - r
			if yo < 0 {
				yo = 0
			}
			yn := y + r + 1
			if yn >= H {
				yn = H - 1
			}
			acc += int(tmp.AlphaAt(x, yn).A) - int(tmp.AlphaAt(x, yo).A)
		}
	}
	return out
}

// --- titlebars (CSS-exact) ---

func drawWindowTitleBar(dst *image.RGBA, r image.Rectangle, title string) {
	draw.Draw(dst, r, &image.Uniform{color.RGBA{0xE8, 0xE8, 0xE8, 0xff}}, image.Point{}, draw.Src)
	// top highlight
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), &image.Uniform{color.RGBA{0xF3, 0xF1, 0xF3, 0xff}}, image.Point{}, draw.Src)
	dots := []struct {
		fill, border color.RGBA
	}{
		{color.RGBA{0xFF, 0x5C, 0x5C, 0xff}, color.RGBA{0xE3, 0x3E, 0x41, 0xff}},
		{color.RGBA{0xFF, 0xBD, 0x4C, 0xff}, color.RGBA{0xE0, 0x9E, 0x3E, 0xff}},
		{color.RGBA{0x00, 0xCA, 0x56, 0xff}, color.RGBA{0x14, 0xAE, 0x46, 0xff}},
	}
	cy := (r.Min.Y + r.Max.Y) / 2
	for i, d := range dots {
		cx := r.Min.X + 8 + 6 + i*(12+4)
		fillCircleBorder(dst, cx, cy, 6, d.fill, d.border)
	}
	face := titleFace()
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0x3B, 0x42, 0x47, 0xff}}, Face: face, Dot: fixed.P(tx, r.Min.Y+16)}
	d.DrawString(title)
}

func drawFloatingTitleBar(dst *image.RGBA, r image.Rectangle, title string) {
	// transparent bg (frame bg already painted); white centered text
	dots := []color.RGBA{
		{0xFF, 0x5C, 0x5C, 0xff}, {0xFF, 0xBD, 0x4C, 0xff}, {0x00, 0xCA, 0x56, 0xff},
	}
	cy := r.Min.Y + 9 + 7 // top 9px + radius
	for i, c := range dots {
		cx := r.Min.X + 13 + 7 + i*(15+4)
		fillCircle(dst, cx, cy, 7, c)
	}
	face := titleFace()
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0xff, 0xff, 0xff, 0xff}}, Face: face, Dot: fixed.P(tx, r.Min.Y+22)}
	d.DrawString(title)
}

func drawSolidTitle(dst *image.RGBA, r image.Rectangle, title string) {
	face := titleFace()
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	// title block: 15px top margin + text
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0xff, 0xff, 0xff, 0xff}}, Face: face, Dot: fixed.P(tx, r.Min.Y+solidTitleMargin+14)}
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

func fillCircleBorder(dst *image.RGBA, cx, cy, rad int, fill, border color.RGBA) {
	for y := -rad; y <= rad; y++ {
		for x := -rad; x <= rad; x++ {
			d2 := x*x + y*y
			if d2 <= rad*rad {
				px, py := cx+x, cy+y
				if !image.Pt(px, py).In(dst.Bounds()) {
					continue
				}
				if d2 >= (rad-1)*(rad-1) {
					dst.Set(px, py, border)
				} else {
					dst.Set(px, py, fill)
				}
			}
		}
	}
}

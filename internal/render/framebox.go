package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/theme"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// CSS-exact values from terminalizer-player (see ref docs), in 1x px:
// radius 6px; window titlebar 22px #E8E8E8; floating titlebar 34px;
// solid title margin 15px; body padding window 10px / floating+solid 20px;
// margin 18px; shadow 0 0 18px #B3B3B3; frame/body bg #1D1D1D.
// Compose happens at renderSS x via the ss parameter; all pixel constants
// below are multiplied by ss so output stays proportional.
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

// framePadTitle resolves body padding + titlebar height (1x px) for cfg.
func framePadTitle(cfg config.Config) (pad, titleH int, title string) {
	ft := frameType(cfg)
	title = frameTitle(cfg)
	pad = floatingPad
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
		}
	case "solid":
		pad = floatingPad
		if title != "" {
			titleH = solidTitleMargin*2 + 20
		}
	default:
		pad = floatingPad
		titleH = 0
		title = ""
	}
	if v := styleGet(cfg, "padding"); strings.TrimSpace(v) != "" {
		if n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(v)), "px")); err == nil && n >= 0 {
			pad = n
		}
	}
	return pad, titleH, title
}

// frameOuterSize returns the final 1x canvas size for a terminal of
// innerW x innerH. DrawFrame downscales the supersampled compose to this.
func frameOuterSize(cfg config.Config, innerW, innerH int) image.Rectangle {
	ft := frameType(cfg)
	margin := parseMarginPx(cfg)
	if ft == "null" {
		return image.Rect(0, 0, innerW+margin*2, innerH+margin*2)
	}
	pad, titleH, _ := framePadTitle(cfg)
	cardW := innerW + pad*2
	cardH := innerH + pad*2 + titleH
	W := cardW + margin*2
	H := cardH + margin*2
	if shadowEnabled(cfg) {
		W += shadowBlur
		H += shadowBlur
	}
	return image.Rect(0, 0, W, H)
}

// applyFrameBox wraps term image with null/window/floating/solid frames.
// cfg must already be supersampled-scaled (see scaledConfig); ss scales the
// frame pixel constants to match. The outer margin area stays transparent
// (default) so GIFs are transparent by default; corners are rounded.
func applyFrameBox(termImg *image.RGBA, cfg config.Config, colors map[string]color.RGBA, termTrans bool, ss int) *image.RGBA {
	if ss <= 0 {
		ss = 1
	}
	ft := frameType(cfg)
	title := frameTitle(cfg)
	margin := parseMarginPx(cfg) * ss

	if ft == "null" {
		bgName := styleGet(cfg, "backgroundColor")
		if bgName == "" {
			bgName = styleGet(cfg, "background")
		}
		if strings.TrimSpace(bgName) == "" {
			// Bare terminal: transparency preserved (default transparent).
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
	pad, titleH1x, _ := framePadTitle(cfg)
	pad *= ss
	titleH := titleH1x * ss

	innerW := termImg.Bounds().Dx()
	innerH := termImg.Bounds().Dy()

	cardW := innerW + pad*2
	cardH := innerH + pad*2 + titleH

	// 1. Build opaque card (rounded).
	card := image.NewRGBA(image.Rect(0, 0, cardW, cardH))
	draw.Draw(card, card.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	switch ft {
	case "window":
		if title != "" {
			wth := windowTitleH * ss
			drawWindowTitleBar(card, image.Rect(0, 0, cardW, wth), title, ss)
			draw.Draw(card, image.Rect(0, wth-1, cardW, wth), &image.Uniform{color.RGBA{0xB1, 0xAE, 0xB1, 0xff}}, image.Point{}, draw.Src)
		}
	case "floating":
		if title != "" {
			drawFloatingTitleBar(card, image.Rect(0, 0, cardW, floatingTitleH*ss), title, ss)
		}
	case "solid":
		if title != "" {
			drawSolidTitle(card, image.Rect(0, 0, cardW, titleH), title, ss)
		}
	}
	// terminal body over card bg
	oy := pad + titleH
	draw.Draw(card, image.Rect(pad, oy, pad+innerW, oy+innerH), termImg, image.Point{}, draw.Over)

	// window 1px border (#B3B3B3 or style.border override)
	borderW, borderCol := parseBorder(cfg)
	borderW *= ss
	if borderW <= 0 && ft == "window" && strings.TrimSpace(styleGet(cfg, "border")) == "" {
		borderW = ss
	}
	if ft == "window" && borderW > 0 {
		strokeRoundedRect(card, borderW, borderCol)
	}

	// rounded corners -> transparent
	radius := frameRadius * ss
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
		W += shadowBlur * ss
		H += shadowBlur * ss
	}
	out := image.NewRGBA(image.Rect(0, 0, W, H))
	ox := margin
	oy2 := margin
	if shadowEnabled(cfg) {
		drawSoftShadow(out, image.Rect(ox, oy2, ox+cardW, oy2+cardH), radius)
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
			d := math.Sqrt(float64(x*x + y*y))
			a := uint8(0)
			if d < float64(r-1) {
				continue // fully opaque
			} else if d < float64(r) {
				a = uint8((float64(r) - d) * 255)
			}
			blend := func(px, py int) {
				if !image.Pt(px, py).In(b) {
					return
				}
				old := img.RGBAAt(px, py)
				k := float64(a) / 255
				old.A = uint8(float64(old.A) * k)
				img.SetRGBA(px, py, old)
			}
			blend(x, y)
			blend(W-1-x, y)
			blend(x, H-1-y)
			blend(W-1-x, H-1-y)
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
func drawSoftShadow(dst *image.RGBA, r image.Rectangle, radius int) {
	sw, sh := r.Dx(), r.Dy()
	sil := image.NewAlpha(image.Rect(0, 0, sw, sh))
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			a := uint8(0xff)
			if radius > 0 {
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
	blurR := 6 * renderSS
	if blurR < 3 {
		blurR = 3
	}
	blurred := boxBlurAlpha(sil, blurR)
	blurred = boxBlurAlpha(blurred, blurR)
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

// --- titlebars (CSS-exact); all rects already scaled, ss scales sub-elements ---

func drawWindowTitleBar(dst *image.RGBA, r image.Rectangle, title string, ss int) {
	if ss <= 0 {
		ss = 1
	}
	draw.Draw(dst, r, &image.Uniform{color.RGBA{0xE8, 0xE8, 0xE8, 0xff}}, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+ss), &image.Uniform{color.RGBA{0xF3, 0xF1, 0xF3, 0xff}}, image.Point{}, draw.Src)
	dots := []struct {
		fill, border color.RGBA
	}{
		{color.RGBA{0xFF, 0x5C, 0x5C, 0xff}, color.RGBA{0xE3, 0x3E, 0x41, 0xff}},
		{color.RGBA{0xFF, 0xBD, 0x4C, 0xff}, color.RGBA{0xE0, 0x9E, 0x3E, 0xff}},
		{color.RGBA{0x00, 0xCA, 0x56, 0xff}, color.RGBA{0x14, 0xAE, 0x46, 0xff}},
	}
	cy := (r.Min.Y + r.Max.Y) / 2
	for i, d := range dots {
		cx := r.Min.X + 8*ss + 6*ss + i*(12*ss+4*ss)
		fillCircleBorder(dst, cx, cy, 6*ss, d.fill, d.border)
	}
	face := titleFacePx(14 * float64(ss))
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0x3B, 0x42, 0x47, 0xff}}, Face: face, Dot: fixed.P(tx, r.Min.Y+16*ss)}
	d.DrawString(title)
}

func drawFloatingTitleBar(dst *image.RGBA, r image.Rectangle, title string, ss int) {
	if ss <= 0 {
		ss = 1
	}
	dots := []color.RGBA{
		{0xFF, 0x5C, 0x5C, 0xff}, {0xFF, 0xBD, 0x4C, 0xff}, {0x00, 0xCA, 0x56, 0xff},
	}
	cy := r.Min.Y + 9*ss + 7*ss
	for i, c := range dots {
		cx := r.Min.X + 13*ss + 7*ss + i*(15*ss+4*ss)
		fillCircle(dst, cx, cy, 7*ss, c)
	}
	face := titleFacePx(14 * float64(ss))
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0xff, 0xff, 0xff, 0xff}}, Face: face, Dot: fixed.P(tx, r.Min.Y+22*ss)}
	d.DrawString(title)
}

func drawSolidTitle(dst *image.RGBA, r image.Rectangle, title string, ss int) {
	if ss <= 0 {
		ss = 1
	}
	face := titleFacePx(14 * float64(ss))
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0xff, 0xff, 0xff, 0xff}}, Face: face, Dot: fixed.P(tx, r.Min.Y+solidTitleMargin*ss+14*ss)}
	d.DrawString(title)
}

// fillCircle blends edges over 1px for smooth dots.
func fillCircle(dst *image.RGBA, cx, cy, rad int, c color.RGBA) {
	if rad <= 0 {
		return
	}
	fr, fg, fb, fa := float64(c.R), float64(c.G), float64(c.B), float64(c.A)/255
	for y := -rad - 1; y <= rad+1; y++ {
		for x := -rad - 1; x <= rad+1; x++ {
			d := math.Sqrt(float64(x*x + y*y))
			if d > float64(rad)+0.5 {
				continue
			}
			cover := 1.0
			if d > float64(rad)-0.5 {
				cover = float64(rad) + 0.5 - d
			}
			px, py := cx+x, cy+y
			if !image.Pt(px, py).In(dst.Bounds()) {
				continue
			}
			old := dst.RGBAAt(px, py)
			sa := fa * cover
			oa := float64(old.A) / 255
			outA := sa + oa*(1-sa)
			if outA == 0 {
				dst.SetRGBA(px, py, color.RGBA{})
				continue
			}
			dst.SetRGBA(px, py, color.RGBA{
				R: uint8((fr*sa+float64(old.R)*oa*(1-sa))/outA + 0.5),
				G: uint8((fg*sa+float64(old.G)*oa*(1-sa))/outA + 0.5),
				B: uint8((fb*sa+float64(old.B)*oa*(1-sa))/outA + 0.5),
				A: uint8(outA*255 + 0.5),
			})
		}
	}
}

func fillCircleBorder(dst *image.RGBA, cx, cy, rad int, fill, border color.RGBA) {
	if rad <= 0 {
		return
	}
	for y := -rad - 1; y <= rad+1; y++ {
		for x := -rad - 1; x <= rad+1; x++ {
			d := math.Sqrt(float64(x*x + y*y))
			if d > float64(rad)+0.5 {
				continue
			}
			cover := 1.0
			if d > float64(rad)-0.5 {
				cover = float64(rad) + 0.5 - d
			}
			px, py := cx+x, cy+y
			if !image.Pt(px, py).In(dst.Bounds()) {
				continue
			}
			c := fill
			if d >= float64(rad-1) {
				c = border
			}
			old := dst.RGBAAt(px, py)
			sa := float64(c.A) / 255 * cover
			oa := float64(old.A) / 255
			outA := sa + oa*(1-sa)
			if outA == 0 {
				dst.SetRGBA(px, py, color.RGBA{})
				continue
			}
			dst.SetRGBA(px, py, color.RGBA{
				R: uint8((float64(c.R)*sa+float64(old.R)*oa*(1-sa))/outA + 0.5),
				G: uint8((float64(c.G)*sa+float64(old.G)*oa*(1-sa))/outA + 0.5),
				B: uint8((float64(c.B)*sa+float64(old.B)*oa*(1-sa))/outA + 0.5),
				A: uint8(outA*255 + 0.5),
			})
		}
	}
}

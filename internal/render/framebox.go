package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/theme"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// CSS-exact values from terminalizer-player (see ref docs), in 1x px at the
// ref default 12px type: radius 6px; window titlebar 22px #E8E8E8; floating
// titlebar 34px; solid title margin 15px; body padding window 10px /
// floating+solid 20px; margin 18px; shadow 0 0 18px; frame/body bg #1D1D1D.
// Frame chrome scales with fontSize via frameScale (12px renders exactly
// like ref; larger type grows dots, bars, pads, radius, margin and shadow
// with it), while 1px hairlines (borders, dividers, dot rings) stay 1px.
// Compose happens at renderSS x via the ss parameter; all pixel constants
// below are multiplied by ss so output stays proportional.
//
// Shadow color note: ref uses #B3B3B3, but that only works composited over
// its white page. Our GIFs are transparent by default and must sit on any
// background, so the glow is neutral black at low peak alpha: over white
// it reads as the same gray ramp as ref, over dark backgrounds it darkens
// like a real shadow instead of glowing white.
const (
	frameRadius   = 6
	frameMargin   = 18
	shadowSigma1x = 7
	// Vertical drop-shadow offset (1x px, downward only): the glow is drawn
	// shadowOffsetY below center, like CSS `box-shadow: 0 2px 18px ...`.
	shadowOffsetY = 2
	// Peak glow opacity (0-255). Blurred edge coverage halves it, so the
	// pixels hugging the card sit near ~0.12 alpha: a faint, fine stipple
	// instead of a coarse dotted band. Higher values look chunkier in GIF's
	// 1-bit alpha, not smoother.
	shadowAlphaPeak = 64
	bodyBG_R        = 0x1D
	bodyBG_G        = 0x1D
	bodyBG_B        = 0x1D
	windowPad       = 10
	floatingPad     = 20
	// Window titlebar total height: CSS height 22px (content-box) plus the
	// 1px top highlight border and 1px bottom divider = 24px.
	windowTitleH     = 24
	floatingTitleH   = 34
	solidTitleMargin = 15
)

// shadowGlow returns the 1x glow extent beyond the card (3 sigma covers it),
// scaled with the frame factor like all structural chrome.
func shadowGlow(cfg config.Config) int {
	sc := frameScale(cfg)
	return sc1(3*shadowSigma1x, sc)
}

// frameScale keeps frame chrome proportional to the terminal text: 12px
// (the ref default) yields factor 1, i.e. ref-exact geometry.
func frameScale(cfg config.Config) float64 {
	px := float64(cfg.FontSize)
	if px <= 0 {
		px = 12
	}
	return px / 12
}

// sc1 scales a 1x structural value by the frame factor (rounded).
func sc1(v float64, sc float64) int { return int(v*sc + 0.5) }

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
	sc := frameScale(cfg)
	if m == "" {
		return sc1(frameMargin, sc)
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
		return sc1(frameMargin, sc)
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return sc1(frameMargin, sc)
	}
	return sc1(float64(n), sc)
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
	sc := frameScale(cfg)
	pad = sc1(floatingPad, sc)
	switch ft {
	case "window":
		pad = sc1(windowPad, sc)
		if title != "" {
			titleH = sc1(windowTitleH, sc)
		}
	case "floating":
		pad = sc1(floatingPad, sc)
		if title != "" {
			titleH = sc1(floatingTitleH, sc)
		}
	case "solid":
		pad = sc1(floatingPad, sc)
		if title != "" {
			titleH = sc1(solidTitleMargin, sc)*2 + sc1(20, sc)
		}
	default:
		pad = sc1(floatingPad, sc)
		titleH = 0
		title = ""
	}
	if v := styleGet(cfg, "padding"); strings.TrimSpace(v) != "" {
		if n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(v)), "px")); err == nil && n >= 0 {
			pad = sc1(float64(n), sc)
		}
	}
	return pad, titleH, title
}

// frameOuterSize returns the final 1x canvas size for a terminal of
// innerW x innerH. DrawFrame downscales the supersampled compose to this.
func frameOuterSize(cfg config.Config, innerW, innerH int) image.Rectangle {
	ft := frameType(cfg)
	sc := frameScale(cfg)
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
		W += shadowGlow(cfg) * 2
		H += shadowGlow(cfg)*2 + sc1(shadowOffsetY, sc)
	}
	return image.Rect(0, 0, W, H)
}

// applyFrameBox wraps term image with null/window/floating/solid frames.
// cfg is the 1x config; ss scales the frame pixel constants to match the
// supersampled terminal image, keeping compose geometry an exact multiple
// of the final layout (see scaledMetrics). The outer margin area stays
// transparent (default) so GIFs are transparent by default; corners round.
func applyFrameBox(termImg *image.RGBA, cfg config.Config, colors map[string]color.RGBA, termTrans bool, ss int) *image.RGBA {
	if ss <= 0 {
		ss = 1
	}
	ft := frameType(cfg)
	title := frameTitle(cfg)
	sc := frameScale(cfg)
	margin := parseMarginPx(cfg) * ss

	if ft == "null" {
		bgName := styleGet(cfg, "backgroundColor")
		if bgName == "" {
			bgName = styleGet(cfg, "background")
		}
		if strings.TrimSpace(bgName) == "" {
			// Bare terminal: transparency preserved (default transparent).
			if margin == 0 {
				return applyWatermark(termImg, cfg, ss)
			}
			W := termImg.Bounds().Dx() + margin*2
			H := termImg.Bounds().Dy() + margin*2
			out := image.NewRGBA(image.Rect(0, 0, W, H))
			draw.Draw(out, image.Rect(margin, margin, margin+termImg.Bounds().Dx(), margin+termImg.Bounds().Dy()), termImg, image.Point{}, draw.Over)
			return applyWatermark(out, cfg, ss)
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
			return applyWatermark(out, cfg, ss)
		}
		W := termImg.Bounds().Dx() + margin*2
		H := termImg.Bounds().Dy() + margin*2
		out := image.NewRGBA(image.Rect(0, 0, W, H))
		if bg.A != 0 {
			draw.Draw(out, out.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
		}
		draw.Draw(out, image.Rect(margin, margin, margin+termImg.Bounds().Dx(), margin+termImg.Bounds().Dy()), termImg, image.Point{}, draw.Over)
		return applyWatermark(out, cfg, ss)
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
			wth := titleH
			drawWindowTitleBar(card, image.Rect(0, 0, cardW, wth), title, ss, sc)
			draw.Draw(card, image.Rect(0, wth-1, cardW, wth), &image.Uniform{color.RGBA{0xB1, 0xAE, 0xB1, 0xff}}, image.Point{}, draw.Src)
		}
	case "floating":
		if title != "" {
			drawFloatingTitleBar(card, image.Rect(0, 0, cardW, titleH), title, ss, sc)
		}
	case "solid":
		if title != "" {
			drawSolidTitle(card, image.Rect(0, 0, cardW, titleH), title, ss, sc)
		}
	}
	// terminal body over card bg
	oy := pad + titleH
	draw.Draw(card, image.Rect(pad, oy, pad+innerW, oy+innerH), termImg, image.Point{}, draw.Over)

	// window 1px border (#B3B3B3 or style.border override), following the
	// rounded edge (CSS border follows border-radius)
	borderW, borderCol := parseBorder(cfg)
	borderW *= ss
	if borderW <= 0 && ft == "window" && strings.TrimSpace(styleGet(cfg, "border")) == "" {
		borderW = ss
	}

	// rounded corners -> transparent (2px AA ramp at compose scale)
	radius := sc1(frameRadius, sc) * ss
	if v := styleGet(cfg, "borderRadius"); strings.TrimSpace(v) != "" {
		low := strings.ToLower(strings.TrimSpace(v))
		if low == "0" || low == "0px" || low == "none" {
			radius = 0
		}
	}
	if radius > 0 {
		applyRoundedMask(card, radius)
	}
	if ft == "window" && borderW > 0 {
		strokeRoundedRectInner(card, borderW, radius, borderCol)
	}

	// watermark is positioned relative to the frame (CSS: .terminalizer-frame prepend)
	card = applyWatermark(card, cfg, ss)

	// 2. Composite onto transparent canvas with margin + soft gaussian glow.
	// Glow spread covers 3 sigma so the falloff is never clipped; the glow
	// is dropped shadowOffsetY below center (y = y + offset).
	glow := shadowGlow(cfg) * ss
	sigma := glow / 3
	if sigma < 1 {
		sigma = 1
	}
	dy := sc1(shadowOffsetY, sc) * ss
	W := cardW + margin*2
	H := cardH + margin*2
	if shadowEnabled(cfg) {
		W += glow * 2
		H += glow*2 + dy
	}
	out := image.NewRGBA(image.Rect(0, 0, W, H))
	ox := margin
	oy2 := margin
	if shadowEnabled(cfg) {
		ox += glow
		oy2 += glow
		draw.Draw(out, image.Rect(ox-glow, oy2-glow+dy, ox+cardW+glow, oy2+cardH+glow+dy),
			shadowLayer(cardW, cardH, radius, sigma), image.Point{}, draw.Over)
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
	if r <= 0 {
		return
	}
	b := img.Bounds()
	W, H := float64(b.Dx()), float64(b.Dy())
	// 2px AA ramp at compose scale for a smooth edge after downscaling
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			d := sdRoundBox(float64(x), float64(y), W, H, float64(r))
			if d <= -2 {
				continue
			}
			if d >= 0 {
				img.SetRGBA(b.Min.X+x, b.Min.Y+y, color.RGBA{})
				continue
			}
			old := img.RGBAAt(b.Min.X+x, b.Min.Y+y)
			k := -d / 2
			if k > 1 {
				k = 1
			}
			// Scale straight RGB and alpha together so the stored
			// premultiplied pixel stays valid (RGB <= A).
			old.R = uint8(float64(old.R) * k)
			old.G = uint8(float64(old.G) * k)
			old.B = uint8(float64(old.B) * k)
			old.A = uint8(float64(old.A) * k)
			img.SetRGBA(b.Min.X+x, b.Min.Y+y, old)
		}
	}
}

// sdRoundBox is the signed distance (px) to a W x H rounded-rect boundary
// with corner radius r; negative inside, centered on the image.
func sdRoundBox(x, y, W, H, r float64) float64 {
	qx := math.Abs(x-(W-1)/2) - ((W-1)/2 - r)
	qy := math.Abs(y-(H-1)/2) - ((H-1)/2 - r)
	ax := math.Max(qx, 0)
	ay := math.Max(qy, 0)
	return math.Sqrt(ax*ax+ay*ay) + math.Min(math.Max(qx, qy), 0) - r
}

func strokeRoundedRectInner(img *image.RGBA, w, radius int, c color.RGBA) {
	if w <= 0 {
		return
	}
	b := img.Bounds()
	W, H := float64(b.Dx()), float64(b.Dy())
	fw := float64(w)
	fr, fg, fb, fa := float64(c.R), float64(c.G), float64(c.B), float64(c.A)/255
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			d := sdRoundBox(float64(x), float64(y), W, H, float64(radius))
			if d < -fw-1 || d > 0 {
				continue
			}
			cover := 1.0
			if d > -1 {
				cover = -d
			} else if d < -fw {
				cover = d + fw + 1
			}
			old := img.RGBAAt(b.Min.X+x, b.Min.Y+y)
			sa := fa * cover
			oa := float64(old.A) / 255
			outA := sa + oa*(1-sa)
			if outA == 0 {
				continue
			}
			// Numerator is already premultiplied (straight coverage math),
			// store it directly: dividing by outA would break RGB <= A.
			img.SetRGBA(b.Min.X+x, b.Min.Y+y, color.RGBA{
				R: uint8(fr*sa + float64(old.R)*oa*(1-sa) + 0.5),
				G: uint8(fg*sa + float64(old.G)*oa*(1-sa) + 0.5),
				B: uint8(fb*sa + float64(old.B)*oa*(1-sa) + 0.5),
				A: uint8(outA*255 + 0.5),
			})
		}
	}
}

type shadowKey struct {
	w, h, radius, sigma int
}

var (
	shadowCacheMu sync.Mutex
	shadowCache   = map[shadowKey]*image.RGBA{}
)

// shadowLayer returns a cached transparent glow canvas (card + 3 sigma each
// side) for the card silhouette: true gaussian blur, energy conserving, so
// the glow spreads wide and falls off smoothly like CSS `0 0 18px`.
func shadowLayer(cardW, cardH, radius, sigma int) *image.RGBA {
	if sigma < 1 {
		sigma = 1
	}
	key := shadowKey{cardW, cardH, radius, sigma}
	shadowCacheMu.Lock()
	if l, ok := shadowCache[key]; ok {
		shadowCacheMu.Unlock()
		return l
	}
	shadowCacheMu.Unlock()

	glow := 3 * sigma
	W, H := cardW+glow*2, cardH+glow*2
	sil := image.NewAlpha(image.Rect(0, 0, W, H))
	fr := float64(radius)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			lx, ly := float64(x-glow), float64(y-glow)
			var a uint8
			if lx < 0 || ly < 0 || lx >= float64(cardW) || ly >= float64(cardH) {
				a = 0
			} else if fr <= 0 {
				a = 0xff
			} else {
				d := sdRoundBox(lx, ly, float64(cardW), float64(cardH), fr)
				if d <= -1 {
					a = 0xff
				} else if d < 0 {
					a = uint8(-d * 255)
				}
			}
			if a != 0 {
				sil.SetAlpha(x, y, color.Alpha{A: a})
			}
		}
	}
	blurred := gaussianBlurAlpha(sil, sigma)
	layer := image.NewRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			a := blurred.AlphaAt(x, y).A
			if a == 0 {
				continue
			}
			// Neutral black glow at low peak alpha (premultiplied: RGB 0).
			// Reads as a gray ramp over white pages and as a real darkening
			// shadow over dark backgrounds — never a white halo.
			outA := uint8(float64(a)*shadowAlphaPeak/255 + 0.5)
			if outA == 0 {
				continue
			}
			layer.SetRGBA(x, y, color.RGBA{R: 0, G: 0, B: 0, A: outA})
		}
	}

	shadowCacheMu.Lock()
	shadowCache[key] = layer
	shadowCacheMu.Unlock()
	return layer
}

// gaussianBlurAlpha applies a true separable gaussian (sigma) to an alpha
// mask, kernel truncated at 3 sigma.
func gaussianBlurAlpha(src *image.Alpha, sigma int) *image.Alpha {
	b := src.Bounds()
	W, H := b.Dx(), b.Dy()
	if sigma <= 0 || W == 0 || H == 0 {
		return src
	}
	rad := 3 * sigma
	k := make([]float64, 2*rad+1)
	sum := 0.0
	s2 := 2 * float64(sigma*sigma)
	for i := range k {
		d := float64(i - rad)
		k[i] = math.Exp(-d * d / s2)
		sum += k[i]
	}
	for i := range k {
		k[i] /= sum
	}
	tmp := image.NewAlpha(b)
	out := image.NewAlpha(b)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			acc := 0.0
			for i, kv := range k {
				xx := x + i - rad
				if xx < 0 {
					xx = 0
				}
				if xx >= W {
					xx = W - 1
				}
				acc += float64(src.AlphaAt(xx, y).A) * kv
			}
			tmp.SetAlpha(x, y, color.Alpha{A: uint8(acc + 0.5)})
		}
	}
	for x := 0; x < W; x++ {
		for y := 0; y < H; y++ {
			acc := 0.0
			for i, kv := range k {
				yy := y + i - rad
				if yy < 0 {
					yy = 0
				}
				if yy >= H {
					yy = H - 1
				}
				acc += float64(tmp.AlphaAt(x, yy).A) * kv
			}
			out.SetAlpha(x, y, color.Alpha{A: uint8(acc + 0.5)})
		}
	}
	return out
}

// --- titlebars (CSS-exact at 12px, proportional above); all rects already
// scaled, ss scales sub-elements, sc scales the 1x chrome values ---

func drawWindowTitleBar(dst *image.RGBA, r image.Rectangle, title string, ss int, sc float64) {
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
	// Dots sit 3px below the CSS top:3.5px center (9.5): eyeballed lower
	// against real renders, the strict CSS value reads too high.
	rad := sc1(6, sc)
	cy := r.Min.Y + int((9.5+3)*sc*float64(ss)+0.5)
	for i, d := range dots {
		cx := r.Min.X + sc1(8, sc)*ss + sc1(6, sc)*ss + i*(sc1(12, sc)*ss+sc1(4, sc)*ss)
		fillCircleBorder(dst, cx, cy, rad*ss, d.fill, d.border)
	}
	face := titleFacePx(14 * sc * float64(ss))
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0x3B, 0x42, 0x47, 0xff}}, Face: face, Dot: fixed.P(tx, r.Min.Y+sc1(17, sc)*ss)}
	d.DrawString(title)
}

func drawFloatingTitleBar(dst *image.RGBA, r image.Rectangle, title string, ss int, sc float64) {
	if ss <= 0 {
		ss = 1
	}
	dots := []color.RGBA{
		{0xFF, 0x5C, 0x5C, 0xff}, {0xFF, 0xBD, 0x4C, 0xff}, {0x00, 0xCA, 0x56, 0xff},
	}
	// CSS: 15px dots, buttons left 13px top 9px -> centers (20.5, 16.5).
	rad := int(7.5*sc*float64(ss) + 0.5)
	cy := r.Min.Y + int(16.5*sc*float64(ss)+0.5)
	for i, c := range dots {
		cx := r.Min.X + int(20.5*sc*float64(ss)+0.5) + i*(sc1(15, sc)*ss+sc1(4, sc)*ss)
		fillCircle(dst, cx, cy, rad, c)
	}
	face := titleFacePx(14 * sc * float64(ss))
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0xff, 0xff, 0xff, 0xff}}, Face: face, Dot: fixed.P(tx, r.Min.Y+sc1(22, sc)*ss)}
	d.DrawString(title)
}

func drawSolidTitle(dst *image.RGBA, r image.Rectangle, title string, ss int, sc float64) {
	if ss <= 0 {
		ss = 1
	}
	face := titleFacePx(14 * sc * float64(ss))
	tw := font.MeasureString(face, title).Ceil()
	tx := r.Min.X + (r.Dx()-tw)/2
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{color.RGBA{0xff, 0xff, 0xff, 0xff}}, Face: face, Dot: fixed.P(tx, r.Min.Y+sc1(solidTitleMargin, sc)*ss+sc1(14, sc)*ss)}
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
			// Premultiplied store (no /outA division): numerator already is.
			dst.SetRGBA(px, py, color.RGBA{
				R: uint8(fr*sa + float64(old.R)*oa*(1-sa) + 0.5),
				G: uint8(fg*sa + float64(old.G)*oa*(1-sa) + 0.5),
				B: uint8(fb*sa + float64(old.B)*oa*(1-sa) + 0.5),
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
			// Premultiplied store (no /outA division): numerator already is.
			dst.SetRGBA(px, py, color.RGBA{
				R: uint8(float64(c.R)*sa + float64(old.R)*oa*(1-sa) + 0.5),
				G: uint8(float64(c.G)*sa + float64(old.G)*oa*(1-sa) + 0.5),
				B: uint8(float64(c.B)*sa + float64(old.B)*oa*(1-sa) + 0.5),
				A: uint8(outA*255 + 0.5),
			})
		}
	}
}

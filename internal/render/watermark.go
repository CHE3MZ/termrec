package render

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/CHE3MZ/termrec/internal/config"
)

// warnOnce prints each distinct warning to stderr a single time per process.
// applyWatermark runs once per frame, so without this every notice would
// repeat for every frame of the GIF.
var (
	warnMu  sync.Mutex
	warned  = map[string]bool{}
)

func warnOnce(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	warnMu.Lock()
	defer warnMu.Unlock()
	if warned[msg] {
		return
	}
	warned[msg] = true
	fmt.Fprintf(os.Stderr, "termrec: warn: %s\n", msg)
}

// applyWatermark overlays a local PNG onto base (frame-relative, CSS absolute).
// Remote URLs are ignored (privacy). Supports right/bottom (default 15px),
// top/left overrides, width/height scaling, opacity.
// Style px values are final-output px; ss scales them to the compose scale.
// Every skip/ignore prints a once-only stderr warning (this runs per frame).
func applyWatermark(base *image.RGBA, cfg config.Config, ss int) *image.RGBA {
	if ss <= 0 {
		ss = 1
	}
	if cfg.Watermark.ImagePath == nil || strings.TrimSpace(*cfg.Watermark.ImagePath) == "" {
		return base
	}
	p := strings.TrimSpace(*cfg.Watermark.ImagePath)
	low := strings.ToLower(p)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		// privacy: never fetch remote watermark
		warnOnce("watermark %q ignored: remote URLs are never fetched (local PNG files only)", p)
		return base
	}
	// data: URLs not supported either (keep local-file only)
	if strings.HasPrefix(low, "data:") {
		warnOnce("watermark ignored: data: URLs are not supported (local PNG files only)")
		return base
	}
	f, err := os.Open(p)
	if err != nil {
		warnOnce("watermark %q ignored: cannot open (%v)", p, err)
		return base
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		warnOnce("watermark %q ignored: not a decodable PNG (%v)", p, err)
		return base
	}
	warnUnknownWatermarkStyle(cfg.Watermark.Style)
	st := cfg.Watermark.Style
	targetW := parsePx(styleGetLower(st, "width"), 0) * ss
	targetH := parsePx(styleGetLower(st, "height"), 0) * ss
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if targetW <= 0 && targetH <= 0 {
		targetW = 100 * ss // CSS default in ref/config.yml
	}
	scaled := scaleBox(src, targetW, targetH)
	_ = sw
	_ = sh
	opacity := parseOpacity(styleGetLower(st, "opacity"), 0.9)
	b := scaled.Bounds()
	tmp := image.NewRGBA(b)
	draw.Draw(tmp, b, scaled, image.Point{}, draw.Src)
	if opacity < 1 {
		// Scale all channels: Pix holds premultiplied values, so alpha
		// must never exceed any RGB channel (RGB <= A invariant).
		for i := 0; i < len(tmp.Pix); i++ {
			tmp.Pix[i] = uint8(float64(tmp.Pix[i]) * opacity)
		}
	}
	// position: default right:15 bottom:15; top/left override when set
	var x, y int
	if v := styleGetLower(st, "left"); strings.TrimSpace(v) != "" {
		x = parsePx(v, 15) * ss
	} else {
		right := parsePx(styleGetLower(st, "right"), 15) * ss
		x = base.Bounds().Dx() - tmp.Bounds().Dx() - right
	}
	if v := styleGetLower(st, "top"); strings.TrimSpace(v) != "" {
		y = parsePx(v, 15) * ss
	} else {
		bottom := parsePx(styleGetLower(st, "bottom"), 15) * ss
		y = base.Bounds().Dy() - tmp.Bounds().Dy() - bottom
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	draw.Draw(base, image.Rect(x, y, x+tmp.Bounds().Dx(), y+tmp.Bounds().Dy()), tmp, image.Point{}, draw.Over)
	return base
}

func styleGetLower(m map[string]string, key string) string {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// warnUnknownWatermarkStyle flags style keys we parse-and-ignore. width,
// height, left, right, top, bottom and opacity are honored; position is
// accepted but harmless (we always overlay absolute, like the CSS default).
// Anything else (ref applies arbitrary CSS via jQuery) is silently dropped
// without this notice.
func warnUnknownWatermarkStyle(st map[string]string) {
	for k := range st {
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "width", "height", "left", "right", "top", "bottom", "opacity", "position":
		default:
			warnOnce("watermark style %q ignored (honored: width/height/left/right/top/bottom/opacity)", k)
		}
	}
}

func parsePx(s string, def int) int {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, "px")
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func parseOpacity(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return f
}

// scaleBox scales src to targetW x targetH (either may be 0 = keep aspect).
func scaleBox(src image.Image, targetW, targetH int) image.Image {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if targetW <= 0 && targetH <= 0 {
		return src
	}
	if targetW <= 0 {
		targetW = int(float64(sw)*float64(targetH)/float64(sh) + 0.5)
	}
	if targetH <= 0 {
		targetH = int(float64(sh)*float64(targetW)/float64(sw) + 0.5)
	}
	if targetW <= 0 {
		targetW = 1
	}
	if targetH <= 0 {
		targetH = 1
	}
	if targetW == sw && targetH == sh {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	rx := float64(sw) / float64(targetW)
	ry := float64(sh) / float64(targetH)
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			sx := int(float64(x)*rx + 0.5)
			sy := int(float64(y)*ry + 0.5)
			if sx >= sw {
				sx = sw - 1
			}
			if sy >= sh {
				sy = sh - 1
			}
			dst.Set(x, y, src.At(sb.Min.X+sx, sb.Min.Y+sy))
		}
	}
	return dst
}

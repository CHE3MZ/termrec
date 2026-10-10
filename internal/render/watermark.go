package render

import (
	"image"
	"image/draw"
	_ "image/png"
	"os"
	"strconv"
	"strings"

	"github.com/CHE3MZ/termrec/internal/config"
)

// applyWatermark overlays a local PNG onto base (frame-relative, CSS absolute).
// Remote URLs are ignored (privacy). Supports right/bottom (default 15px),
// top/left overrides, width/height scaling, opacity.
func applyWatermark(base *image.RGBA, cfg config.Config) *image.RGBA {
	if cfg.Watermark.ImagePath == nil || strings.TrimSpace(*cfg.Watermark.ImagePath) == "" {
		return base
	}
	p := strings.TrimSpace(*cfg.Watermark.ImagePath)
	low := strings.ToLower(p)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		// privacy: never fetch remote watermark
		return base
	}
	// data: URLs not supported either (keep local-file only)
	if strings.HasPrefix(low, "data:") {
		return base
	}
	f, err := os.Open(p)
	if err != nil {
		return base
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return base
	}
	st := cfg.Watermark.Style
	targetW := parsePx(styleGetLower(st, "width"), 0)
	targetH := parsePx(styleGetLower(st, "height"), 0)
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if targetW <= 0 && targetH <= 0 {
		targetW = 100 // CSS default in ref/config.yml
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
		x = parsePx(v, 15)
	} else {
		right := parsePx(styleGetLower(st, "right"), 15)
		x = base.Bounds().Dx() - tmp.Bounds().Dx() - right
	}
	if v := styleGetLower(st, "top"); strings.TrimSpace(v) != "" {
		y = parsePx(v, 15)
	} else {
		bottom := parsePx(styleGetLower(st, "bottom"), 15)
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

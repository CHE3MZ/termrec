package render

import (
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"os"
	"strconv"
	"strings"

	"github.com/CHE3MZ/termrec/internal/config"
)

// applyWatermark overlays a local PNG onto base. Remote URLs are ignored (privacy).
func applyWatermark(base *image.RGBA, cfg config.Config) *image.RGBA {
	if cfg.Watermark.ImagePath == nil || strings.TrimSpace(*cfg.Watermark.ImagePath) == "" {
		return base
	}
	p := strings.TrimSpace(*cfg.Watermark.ImagePath)
	if strings.HasPrefix(strings.ToLower(p), "http://") || strings.HasPrefix(strings.ToLower(p), "https://") {
		// privacy: never fetch remote watermark
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
	targetW := parsePx(styleGetLower(cfg.Watermark.Style, "width"), src.Bounds().Dx())
	opacity := parseOpacity(styleGetLower(cfg.Watermark.Style, "opacity"), 0.9)
	scaled := scaleNearest(src, targetW)
	// apply opacity
	b := scaled.Bounds()
	tmp := image.NewRGBA(b)
	draw.Draw(tmp, b, scaled, image.Point{}, draw.Src)
	if opacity < 1 {
		for i := 3; i < len(tmp.Pix); i += 4 {
			tmp.Pix[i] = uint8(float64(tmp.Pix[i]) * opacity)
		}
	}
	right := parsePx(styleGetLower(cfg.Watermark.Style, "right"), 15)
	bottom := parsePx(styleGetLower(cfg.Watermark.Style, "bottom"), 15)
	x := base.Bounds().Dx() - tmp.Bounds().Dx() - right
	y := base.Bounds().Dy() - tmp.Bounds().Dy() - bottom
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
	if err != nil || n <= 0 {
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

func scaleNearest(src image.Image, targetW int) image.Image {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if targetW <= 0 || targetW == sw {
		return src
	}
	ratio := float64(targetW) / float64(sw)
	targetH := int(float64(sh) * ratio)
	if targetH <= 0 {
		targetH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			sx := int(float64(x) / ratio)
			sy := int(float64(y) / ratio)
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

// ensure opaque flattens alpha onto white for GIF encoding.
func flatten(img *image.RGBA) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
	white := &image.Uniform{color.RGBA{0xff, 0xff, 0xff, 0xff}}
	draw.Draw(out, out.Bounds(), white, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), img, image.Point{}, draw.Over)
	return out
}

package render

import (
	"image"
	"image/color"
)

// hasTransparency reports whether any pixel is (semi-)transparent.
func hasTransparency(img *image.RGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] < 128 {
			return true
		}
	}
	return false
}

// rgbaToPaletted quantizes an RGBA frame to Paletted, preserving transparency.
// When transparent pixels exist, palette index 0 is fully transparent
// (which Go's gif encoder emits as the GIF transparent index), so renders
// are transparent by default instead of flattened to white.
// Deterministic and dep-free; tuned for terminal art (flat fills + text).
func rgbaToPaletted(img *image.RGBA, n int) *image.Paletted {
	if n <= 0 || n > 256 {
		n = 256
	}
	transparent := hasTransparency(img)
	budget := n
	if transparent {
		budget = n - 1
		if budget < 1 {
			budget = 1
		}
	}
	pal := paletteForOpaque(img, budget)
	full := make(color.Palette, 0, len(pal)+1)
	if transparent {
		full = append(full, color.RGBA{})
	}
	full = append(full, pal...)
	pm := image.NewPaletted(img.Bounds(), full)
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if transparent && c.A < 128 {
				pm.SetColorIndex(x, y, 0)
				continue
			}
			pm.Set(x, y, c)
		}
	}
	return pm
}

// paletteForOpaque builds a palette of up to n opaque colors by sampling.
// First-seen order, deterministic.
func paletteForOpaque(img *image.RGBA, n int) color.Palette {
	if n <= 0 {
		n = 1
	}
	if n > 256 {
		n = 256
	}
	b := img.Bounds()
	const maxSamples = 20000
	total := b.Dx() * b.Dy()
	step := 1
	if total > maxSamples {
		step = total / maxSamples
		if step < 1 {
			step = 1
		}
	}
	seen := map[uint32]struct{}{}
	pal := color.Palette{}
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if i%step != 0 {
				i++
				continue
			}
			i++
			off := img.PixOffset(x, y)
			a := img.Pix[off+3]
			if a < 128 {
				continue
			}
			r, g, bl := img.Pix[off], img.Pix[off+1], img.Pix[off+2]
			key := uint32(r)<<16 | uint32(g)<<8 | uint32(bl)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			pal = append(pal, color.RGBA{R: r, G: g, B: bl, A: 0xff})
			if len(pal) >= n {
				return pal
			}
		}
	}
	if len(pal) == 0 {
		return color.Palette{color.Black, color.White}
	}
	return pal
}

// paletteFor keeps the old name for callers/tests: opaque sampling.
func paletteFor(img image.Image, n int) color.Palette {
	if rgba, ok := img.(*image.RGBA); ok {
		return paletteForOpaque(rgba, n)
	}
	b := img.Bounds()
	tmp := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			tmp.Set(x, y, img.At(x, y))
		}
	}
	return paletteForOpaque(tmp, n)
}

// ditherCopy copies src into dst paletted (nearest, no error diffusion).
func ditherCopy(dst *image.Paletted, src image.Image) {
	b := dst.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
}

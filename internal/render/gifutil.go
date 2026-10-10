package render

import (
	"image"
	"image/color"
)

// paletteFor builds a palette capped at n colors by sampling the image.
// Simple + deterministic (no external quantizer dep).
func paletteFor(img image.Image, n int) color.Palette {
	if n <= 0 || n > 256 {
		n = 256
	}
	b := img.Bounds()
	// Sample up to 20k pixels uniformly.
	const maxSamples = 20000
	w, h := b.Dx(), b.Dy()
	total := w * h
	step := 1
	if total > maxSamples {
		step = total / maxSamples
		if step < 1 {
			step = 1
		}
	}
	seen := map[uint32]color.Color{}
	pal := color.Palette{}
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if i%step != 0 {
				i++
				continue
			}
			i++
			c := img.At(x, y)
			r, g, bl, a := c.RGBA()
			key := (r>>8)<<24 | (g>>8)<<16 | (bl>>8)<<8 | (a >> 8)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = c
			pal = append(pal, c)
			if len(pal) >= n*4 { // oversample then truncate by frequency-ish (first wins, deterministic)
				// keep collecting keys but stop growing palette; simplest: stop
				if len(pal) >= 256 {
					return pal[:n]
				}
			}
			if len(pal) >= 256 {
				return pal[:n]
			}
		}
	}
	if len(pal) == 0 {
		return color.Palette{color.Black, color.White}
	}
	if len(pal) > n {
		pal = pal[:n]
	}
	return pal
}

// ditherCopy copies src into dst paletted without Floyd-Steinberg (nearest, no error diffusion)
// to keep output deterministic and dep-free. GIF viewers handle it fine for terminal art.
func ditherCopy(dst *image.Paletted, src image.Image) {
	b := dst.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
}

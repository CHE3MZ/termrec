package render

import (
	"image"
	"image/color"
	"sort"
)

// Global palette + Floyd-Steinberg dithering for smooth gradients and
// exact flat colors (color consistency: same config color -> same GIF index
// in every frame, instead of per-frame first-seen palettes drifting).

// hasTransparency reports whether any pixel is (semi-)transparent.
func hasTransparency(img *image.RGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] < 128 {
			return true
		}
	}
	return false
}

func anyTransparency(frames []*image.RGBA) bool {
	for _, f := range frames {
		if hasTransparency(f) {
			return true
		}
	}
	return false
}

// anchorPalette returns exact colors that must survive quantization losslessly:
// flat fills (theme, frame chrome, cursor). Anchors come first so indices are
// stable across frames and renders with the same config.
func anchorPalette(anchors []color.RGBA, transparent bool) color.Palette {
	pal := color.Palette{}
	if transparent {
		pal = append(pal, color.RGBA{})
	}
	seen := map[uint32]struct{}{}
	if transparent {
		seen[0] = struct{}{}
	}
	for _, c := range anchors {
		if c.A < 128 {
			continue
		}
		key := uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		pal = append(pal, color.RGBA{R: c.R, G: c.G, B: c.B, A: 0xff})
	}
	return pal
}

// buildGlobalPalette merges anchors + most-frequent frame colors up to n.
// Deterministic: anchors first (stable order), then frequency desc, RGB asc.
func buildGlobalPalette(frames []*image.RGBA, anchors []color.RGBA, n int, transparent bool) color.Palette {
	if n <= 0 || n > 256 {
		n = 256
	}
	pal := anchorPalette(anchors, transparent)
	if len(pal) >= n {
		return pal[:n]
	}
	freq := map[uint32]int{}
	for _, img := range frames {
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y += 2 {
			for x := b.Min.X; x < b.Max.X; x += 2 {
				off := img.PixOffset(x, y)
				if img.Pix[off+3] < 128 {
					continue
				}
				key := uint32(img.Pix[off])<<16 | uint32(img.Pix[off+1])<<8 | uint32(img.Pix[off+2])
				freq[key]++
			}
		}
	}
	// seed "seen" with anchors
	seen := map[uint32]struct{}{}
	for _, c := range pal {
		r, g, b, _ := c.RGBA()
		seen[uint32(r>>8)<<16|uint32(g>>8)<<8|uint32(b>>8)] = struct{}{}
	}
	type kv struct {
		key uint32
		n   int
	}
	list := make([]kv, 0, len(freq))
	for k, v := range freq {
		if _, ok := seen[k]; ok {
			continue
		}
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].key < list[j].key
	})
	for _, e := range list {
		if len(pal) >= n {
			break
		}
		pal = append(pal, color.RGBA{R: uint8(e.key >> 16), G: uint8(e.key >> 8), B: uint8(e.key), A: 0xff})
	}
	if len(pal) == 0 {
		return color.Palette{color.Black, color.White}
	}
	// transparent slot must stay at 0 but palette needs >= 2 entries for gif
	if transparent && len(pal) == 1 {
		pal = append(pal, color.Black)
	}
	return pal
}

// quantCache maps 15-bit RGB to palette index to avoid O(256) scans per pixel.
type quantCache struct {
	pal   color.Palette
	start int // skip transparent index 0
	memo  map[uint32]int
}

func newQuantCache(pal color.Palette, transparent bool) *quantCache {
	start := 0
	if transparent {
		start = 1
	}
	return &quantCache{pal: pal, start: start, memo: map[uint32]int{}}
}

func absI(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (q *quantCache) nearest(r, g, b int) int {
	key := uint32(r>>3)<<10 | uint32(g>>3)<<5 | uint32(b>>3)
	if idx, ok := q.memo[key]; ok {
		return idx
	}
	best, bestD := q.start, 1<<30
	for i := q.start; i < len(q.pal); i++ {
		pr, pg, pb, _ := q.pal[i].RGBA()
		dr := r - int(pr>>8)
		dg := g - int(pg>>8)
		db := b - int(pb>>8)
		// fast weighted distance (redmean-ish without float)
		d := (dr*dr*3 + dg*dg*4 + db*db*2)
		if d < bestD {
			bestD = d
			best = i
			if d == 0 {
				break
			}
		}
	}
	// memo only on reasonable precision loss (15-bit key)
	q.memo[key] = best
	return best
}

// ditherFS maps src into a paletted image with serpentine Floyd-Steinberg
// diffusion. Transparent pixels stay index 0 and neither emit nor absorb
// error across the transparency edge (keeps edges crisp, no halos).
// Flat-lock: pixels exactly matching a palette entry bypass diffusion
// entirely (no error in or out), so large flat fills stay bit-exact instead
// of speckling with neighbor tones.
func ditherFS(src *image.RGBA, pal color.Palette, transparent bool) *image.Paletted {
	b := src.Bounds()
	W, H := b.Dx(), b.Dy()
	dst := image.NewPaletted(b, pal)
	// exact palette lookup for the flat-lock (opaque entries only)
	exact := map[uint32]uint8{}
	for i := 0; i < len(pal); i++ {
		if transparent && i == 0 {
			continue
		}
		pr, pg, pb, pa := pal[i].RGBA()
		if pa>>8 < 128 {
			continue
		}
		exact[uint32(pr>>8)<<16|uint32(pg>>8)<<8|uint32(pb>>8)] = uint8(i)
	}
	// float error buffers (premultiplied space not needed; diffusion on RGB)
	bufR := make([][]float64, H)
	bufG := make([][]float64, H)
	bufB := make([][]float64, H)
	skip := make([][]bool, H)
	locked := make([][]bool, H)
	for y := 0; y < H; y++ {
		bufR[y] = make([]float64, W)
		bufG[y] = make([]float64, W)
		bufB[y] = make([]float64, W)
		skip[y] = make([]bool, W)
		locked[y] = make([]bool, W)
		for x := 0; x < W; x++ {
			off := src.PixOffset(b.Min.X+x, b.Min.Y+y)
			if transparent && src.Pix[off+3] < 128 {
				skip[y][x] = true
				continue
			}
			r, g, bl := src.Pix[off], src.Pix[off+1], src.Pix[off+2]
			if idx, ok := exact[uint32(r)<<16|uint32(g)<<8|uint32(bl)]; ok {
				locked[y][x] = true
				dst.SetColorIndex(b.Min.X+x, b.Min.Y+y, idx)
				continue
			}
			bufR[y][x] = float64(r)
			bufG[y][x] = float64(g)
			bufB[y][x] = float64(bl)
		}
	}
	qc := newQuantCache(pal, transparent)
	for y := 0; y < H; y++ {
		var x0, x1, dx int
		if y%2 == 0 {
			x0, x1, dx = 0, W, 1
		} else {
			x0, x1, dx = W-1, -1, -1
		}
		for x := x0; x != x1; x += dx {
			if skip[y][x] {
				dst.SetColorIndex(b.Min.X+x, b.Min.Y+y, 0)
				continue
			}
			if locked[y][x] {
				continue // already set; absorbs/emits no error
			}
			or := clampF(bufR[y][x])
			og := clampF(bufG[y][x])
			ob := clampF(bufB[y][x])
			idx := qc.nearest(or, og, ob)
			dst.SetColorIndex(b.Min.X+x, b.Min.Y+y, uint8(idx))
			pr, pg, pb, _ := pal[idx].RGBA()
			er := float64(or) - float64(pr>>8)
			eg := float64(og) - float64(pg>>8)
			eb := float64(ob) - float64(pb>>8)
			// distribute (mirrored on odd rows)
			type nb struct {
				dx, dy int
				w      float64
			}
			var nbs []nb
			if dx == 1 {
				nbs = []nb{{1, 0, 7.0 / 16}, {-1, 1, 3.0 / 16}, {0, 1, 5.0 / 16}, {1, 1, 1.0 / 16}}
			} else {
				nbs = []nb{{-1, 0, 7.0 / 16}, {1, 1, 3.0 / 16}, {0, 1, 5.0 / 16}, {-1, 1, 1.0 / 16}}
			}
			for _, nb := range nbs {
				nx, ny := x+nb.dx, y+nb.dy
				if nx < 0 || nx >= W || ny < 0 || ny >= H || skip[ny][nx] || locked[ny][nx] {
					continue
				}
				bufR[ny][nx] += er * nb.w
				bufG[ny][nx] += eg * nb.w
				bufB[ny][nx] += eb * nb.w
			}
		}
	}
	return dst
}

func clampF(f float64) int {
	if f < 0 {
		return 0
	}
	if f > 255 {
		return 255
	}
	return int(f + 0.5)
}

// rgbaToPaletted keeps the old single-frame entry point (used by tests):
// anchors empty, global built from this frame only.
func rgbaToPaletted(img *image.RGBA, n int) *image.Paletted {
	if n <= 0 || n > 256 {
		n = 256
	}
	transparent := hasTransparency(img)
	budget := n
	if transparent {
		budget = n - 1
		if budget < 2 {
			budget = 2
		}
	}
	pal := buildGlobalPalette([]*image.RGBA{img}, nil, budget, transparent)
	return ditherFS(img, pal, transparent)
}

// paletteForOpaque builds a palette of up to n opaque colors by sampling.
// Kept for compatibility; prefers frequency order.
func paletteForOpaque(img *image.RGBA, n int) color.Palette {
	if n <= 0 {
		n = 1
	}
	if n > 256 {
		n = 256
	}
	return buildGlobalPalette([]*image.RGBA{img}, nil, n, false)
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

// ditherCopy maps src into dst with FS diffusion using dst's palette.
func ditherCopy(dst *image.Paletted, src image.Image) {
	b := dst.Bounds()
	tmp := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			tmp.Set(x, y, src.At(x, y))
		}
	}
	mapped := ditherFS(tmp, dst.Palette, false)
	copy(dst.Pix, mapped.Pix)
}

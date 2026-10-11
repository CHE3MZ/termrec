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
// diffusion. GIF has 1-bit alpha, so semi-transparent pixels (drop shadows,
// antialiased outer edges) are decided by diffused alpha: the coverage is
// preserved as a stipple instead of collapsing to a hard transparent/opaque
// edge. Fully transparent pixels stay index 0.
// Flat-lock: fully opaque pixels exactly matching a palette entry bypass
// diffusion entirely (no error in or out), so large flat fills stay
// bit-exact instead of speckling with neighbor tones.
func ditherFS(src *image.RGBA, pal color.Palette, transparent bool) *image.Paletted {
	return ditherFSRect(src, src.Bounds(), pal, transparent)
}

// ditherFSRect quantizes only r (a dirty bbox of src) with the same
// Stucki diffusion, flat-lock, and transparency rules. The output keeps
// r's offset so stdlib image/gif writes it as a positioned sub-frame;
// diffusion at the rect edge simply has no outside neighbors.
func ditherFSRect(src *image.RGBA, r image.Rectangle, pal color.Palette, transparent bool) *image.Paletted {
	b := r
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
	// float error buffers (diffusion on RGB + alpha coverage)
	bufR := make([][]float64, H)
	bufG := make([][]float64, H)
	bufB := make([][]float64, H)
	bufA := make([][]float64, H)
	skip := make([][]bool, H)
	locked := make([][]bool, H)
	for y := 0; y < H; y++ {
		bufR[y] = make([]float64, W)
		bufG[y] = make([]float64, W)
		bufB[y] = make([]float64, W)
		bufA[y] = make([]float64, W)
		skip[y] = make([]bool, W)
		locked[y] = make([]bool, W)
		for x := 0; x < W; x++ {
			off := src.PixOffset(b.Min.X+x, b.Min.Y+y)
			a := src.Pix[off+3]
			if transparent && a < 8 {
				skip[y][x] = true
				continue
			}
			r, g, bl := src.Pix[off], src.Pix[off+1], src.Pix[off+2]
			if transparent && a < 24 && r*2 <= a && g*2 <= a && bl*2 <= a {
				// Absorb faint dark coverage (shadow tail, AA dust): it is
				// invisible on every background by itself — keeping it would
				// only let error diffusion concentrate it into a visible
				// stipple fringe far from the card. Hard transparent, and
				// excluded from error flow both ways (see skip checks in
				// the diffusion loop). Bright faint pixels (text AA) are
				// unaffected: their straight color is light (R ~= A).
				skip[y][x] = true
				continue
			}
			if a > 247 {
				if idx, ok := exact[uint32(r)<<16|uint32(g)<<8|uint32(bl)]; ok {
					locked[y][x] = true
					dst.SetColorIndex(b.Min.X+x, b.Min.Y+y, idx)
					continue
				}
			}
			bufR[y][x] = float64(r)
			bufG[y][x] = float64(g)
			bufB[y][x] = float64(bl)
			bufA[y][x] = float64(a)
		}
	}
	qc := newQuantCache(pal, transparent)
	// darkSemi marks pixels that were semi-transparent with a dark straight
	// color (shadow glow, card-edge AA): when diffusion resolves one to
	// opaque, it must become pure black, never a dark-gray tone that would
	// read as a dotted fringe on dark pages. Light semi pixels (text AA)
	// keep the normal nearest-color path.
	darkSemi := make([][]bool, H)
	for y := 0; y < H; y++ {
		darkSemi[y] = make([]bool, W)
		for x := 0; x < W; x++ {
			if skip[y][x] || locked[y][x] {
				continue
			}
			off := src.PixOffset(b.Min.X+x, b.Min.Y+y)
			a := uint16(src.Pix[off+3])
			if !transparent || a >= 250 || a == 0 {
				continue
			}
			r, g, bl := uint16(src.Pix[off]), uint16(src.Pix[off+1]), uint16(src.Pix[off+2])
			if r*2 <= a && g*2 <= a && bl*2 <= a {
				darkSemi[y][x] = true
			}
		}
	}
	// blackIdx is the opaque pure-black palette slot (anchored by render.go
	// for transparent output). Dark semi pixels forced there vanish on dark
	// pages and read as shadow density on light pages.
	blackIdx := -1
	for i := 0; i < len(pal); i++ {
		if transparent && i == 0 {
			continue
		}
		pr, pg, pb, pa := pal[i].RGBA()
		if pr>>8 == 0 && pg>>8 == 0 && pb>>8 == 0 && pa>>8 >= 128 {
			blackIdx = i
			break
		}
	}
	// Stucki error-diffusion kernel (×/42, mirrored on odd rows). It spreads
	// error over two rows ahead instead of one, which breaks up the worm
	// trails Floyd-Steinberg leaves in smooth gradients (like the shadow
	// falloff) into finer, more uniform grain.
	// Weights: row0 (1,8)(2,4); row1 (-2,2)(-1,4)(0,8)(1,4)(2,2);
	// row2 (-2,1)(-1,2)(0,4)(1,2)(2,1), all /42.
	type nb struct {
		dx, dy int
		w      float64
	}
	stucki := [12][3]float64{
		{1, 0, 8}, {2, 0, 4},
		{-2, 1, 2}, {-1, 1, 4}, {0, 1, 8}, {1, 1, 4}, {2, 1, 2},
		{-2, 2, 1}, {-1, 2, 2}, {0, 2, 4}, {1, 2, 2}, {2, 2, 1},
	}
	nbsFwd := make([]nb, 0, len(stucki))
	nbsBwd := make([]nb, 0, len(stucki))
	for _, k := range stucki {
		w := k[2] / 42
		nbsFwd = append(nbsFwd, nb{int(k[0]), int(k[1]), w})
		nbsBwd = append(nbsBwd, nb{-int(k[0]), int(k[1]), w})
	}
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
			oa := clampF(bufA[y][x])
			var idx int
			var er, eg, eb, ea float64
			if transparent && oa < 128 {
				// coverage says transparent: emit nothing, diffuse alpha error
				idx = 0
				ea = float64(oa)
			} else {
				idx = qc.nearest(or, og, ob)
				pr, pg, pb, _ := pal[idx].RGBA()
				if darkSemi[y][x] && blackIdx >= 0 {
					idx = blackIdx
					pr, pg, pb = 0, 0, 0
				}
				er = float64(or) - float64(pr>>8)
				eg = float64(og) - float64(pg>>8)
				eb = float64(ob) - float64(pb>>8)
				ea = float64(oa) - 255
			}
			dst.SetColorIndex(b.Min.X+x, b.Min.Y+y, uint8(idx))
			nbs := nbsFwd
			if dx == -1 {
				nbs = nbsBwd
			}
			for _, nb := range nbs {
				nx, ny := x+nb.dx, y+nb.dy
				if nx < 0 || nx >= W || ny < 0 || ny >= H || skip[ny][nx] || locked[ny][nx] {
					continue
				}
				bufR[ny][nx] += er * nb.w
				bufG[ny][nx] += eg * nb.w
				bufB[ny][nx] += eb * nb.w
				bufA[ny][nx] += ea * nb.w
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

package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"os"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/recording"
	"github.com/CHE3MZ/termrec/internal/theme"
	"github.com/CHE3MZ/termrec/internal/timing"
)

// qualityToColors maps 1-100 to GIF palette size. stdlib has no quality knob.
// One slot is reserved for transparency when the frame needs it.
func qualityToColors(q int) int {
	if q <= 0 {
		q = 75
	}
	switch {
	case q >= 95:
		return 256
	case q >= 80:
		return 128
	case q >= 60:
		return 64
	default:
		return 32
	}
}

// anchorColors lists every exact flat color the composer paints, so the
// global GIF palette keeps them bit-exact: theme, cursor, frame chrome
// (titlebars, dots + borders, body bg, title text).
func anchorColors(cfg config.Config) []color.RGBA {
	colors, _ := theme.Map(cfg.Theme)
	out := make([]color.RGBA, 0, len(colors)+16)
	for _, k := range theme.Names {
		out = append(out, colors[k])
	}
	out = append(out,
		color.RGBA{0x1D, 0x1D, 0x1D, 0xff}, // frame/body bg
		color.RGBA{0xE8, 0xE8, 0xE8, 0xff}, // window titlebar
		color.RGBA{0xF3, 0xF1, 0xF3, 0xff}, // window top highlight
		color.RGBA{0xB1, 0xAE, 0xB1, 0xff}, // window titlebar bottom border
		color.RGBA{0xB3, 0xB3, 0xB3, 0xff}, // window border
		color.RGBA{0x3B, 0x42, 0x47, 0xff}, // window title text
		color.RGBA{0xFF, 0x5C, 0x5C, 0xff}, // close dot
		color.RGBA{0xE3, 0x3E, 0x41, 0xff}, // close border
		color.RGBA{0xFF, 0xBD, 0x4C, 0xff}, // minimize dot
		color.RGBA{0xE0, 0x9E, 0x3E, 0xff}, // minimize border
		color.RGBA{0x00, 0xCA, 0x56, 0xff}, // maximize dot
		color.RGBA{0x14, 0xAE, 0x46, 0xff}, // maximize border
		color.RGBA{0xFF, 0xFF, 0xFF, 0xff}, // floating/solid title text
		color.RGBA{0x00, 0x00, 0x00, 0xff}, // shadow stipple dots vanish on dark pages
	)
	if v := styleGet(cfg, "backgroundColor"); v != "" {
		if c, t, err := theme.Parse(v); err == nil && !t {
			out = append(out, c)
		}
	}
	if v := styleGet(cfg, "background"); v != "" {
		if c, t, err := theme.Parse(v); err == nil && !t {
			out = append(out, c)
		}
	}
	return out
}

// flattenOnto composites a transparent frame over the pageBG color.
// Empty/"transparent" returns img untouched; unparseable colors error.
func flattenOnto(img *image.RGBA, pageBG string) (*image.RGBA, error) {
	c, isTrans, err := theme.Parse(pageBG)
	if err != nil {
		return nil, fmt.Errorf("bad background %q: %v", pageBG, err)
	}
	if isTrans {
		return img, nil
	}
	out := image.NewRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), img, image.Point{}, draw.Over)
	return out, nil
}

// wallpaperPad is the padding around the frame on a wallpaper page, in
// final-output px (t-rec default 60).
const wallpaperPad = 60

// resolvePage applies pageBG to a frame: a CSS color (or "transparent")
// flattens via flattenOnto, otherwise pageBG is a local image file
// composed as wallpaper (t-rec parity). Anything else errors.
func resolvePage(img *image.RGBA, pageBG string) (*image.RGBA, error) {
	if _, _, err := theme.Parse(pageBG); err == nil {
		return flattenOnto(img, pageBG)
	}
	return flattenWallpaper(img, pageBG)
}

// flattenWallpaper centers the frame over a local image page: the page is
// the frame plus wallpaperPad each side, center-cropped from the image,
// which must cover it or render fails up front (t-rec pre-validates the
// wallpaper before recording too). PNG and JPEG decode from stdlib.
func flattenWallpaper(img *image.RGBA, path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("bad background %q: %v", path, err)
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("bad background %q: not a decodable image (%v)", path, err)
	}
	sb := src.Bounds()
	ib := img.Bounds()
	W, H := ib.Dx()+wallpaperPad*2, ib.Dy()+wallpaperPad*2
	if sb.Dx() < W || sb.Dy() < H {
		return nil, fmt.Errorf("bad background %q: image %dx%d smaller than %dx%d page", path, sb.Dx(), sb.Dy(), W, H)
	}
	ox := sb.Min.X + (sb.Dx()-W)/2
	oy := sb.Min.Y + (sb.Dy()-H)/2
	out := image.NewRGBA(image.Rect(0, 0, W, H))
	draw.Draw(out, out.Bounds(), src, image.Point{ox, oy}, draw.Src)
	draw.Draw(out, image.Rect(wallpaperPad, wallpaperPad, wallpaperPad+ib.Dx(), wallpaperPad+ib.Dy()), img, image.Point{}, draw.Over)
	return out, nil
}

// diffRect returns the dirty bounding box between consecutive same-canvas
// frames, so each frame encodes only what changed (ImageMagick's -layers
// Optimize, reimplemented for stdlib image/gif). ok=false means the frames
// are pixel-identical and the delay should merge into the previous frame
// (t-rec's idle-frame excision).
func diffRect(a, b *image.RGBA) (r image.Rectangle, ok bool) {
	ba := a.Bounds()
	if !ba.Eq(b.Bounds()) {
		return ba, true
	}
	minX, minY := ba.Max.X, ba.Max.Y
	maxX, maxY := ba.Min.X-1, ba.Min.Y-1
	for y := ba.Min.Y; y < ba.Max.Y; y++ {
		for x := ba.Min.X; x < ba.Max.X; x++ {
			ao := a.PixOffset(x, y)
			bo := b.PixOffset(x, y)
			if a.Pix[ao] != b.Pix[bo] || a.Pix[ao+1] != b.Pix[bo+1] ||
				a.Pix[ao+2] != b.Pix[bo+2] || a.Pix[ao+3] != b.Pix[bo+3] {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}

func repeatToLoopCount(r int) int {
	// GIF LoopCount semantics (see image/gif docs): 0 = loop forever,
	// -1 = show each frame only once, else looped LoopCount+1 times.
	// ref passes repeat straight to the encoder, so -1 must stay -1.
	if r == -1 {
		return -1 // play once
	}
	if r < -1 {
		return 0
	}
	return r
}

// blinkGrace is how long the cursor stays solid before the first toggle:
// new content lands, the eye settles, and only then does it blink. Without
// this the cursor visibly flashes the instant a line appears.
// blinkCycle sizes the off/on phases after the grace; blinkMinTail absorbs
// small remainders so no phase is ever a quick strobe blip.
const blinkGrace = 700
const blinkCycle = 600
const blinkMinTail = 350

// maxPauseMS caps start/end hold pauses (t-rec caps pauses at 5min).
const maxPauseMS = 300000

// maxBlinkPhases caps the split so one huge uncapped pause (maxIdleTime:
// auto) can't explode into hundreds of near-identical frames. The last
// phase absorbs the remainder, so total duration is always exact.
const maxBlinkPhases = 8

// blinkChunks splits delay into a solid-on grace plus alternating off/on
// phases. Pauses under ~1s stay a single steady frame: typing never
// strobes and fresh lines never flash.
func blinkChunks(delay int) []int {
	if delay <= blinkGrace+blinkMinTail {
		return []int{delay}
	}
	rest := delay - blinkGrace
	n := (rest + blinkCycle - 1) / blinkCycle
	if n > maxBlinkPhases-1 {
		n = maxBlinkPhases - 1
	}
	out := make([]int, 0, n+1)
	out = append(out, blinkGrace)
	base := rest / n
	for i := 0; i < n; i++ {
		out = append(out, base)
	}
	out[len(out)-1] += rest - base*n // remainder to last chunk
	return out
}

// Render converts a recording to an animated GIF.
// Transparency is preserved by default: theme background "transparent"
// (the default) yields transparent GIF pixels. Pass pageBG (any CSS color:
// name, #hex, rgb(), hsl()) to flatten frames onto an opaque page instead,
// e.g. "white" for the classic backgrounded look with a smooth shadow; a
// local image file path composes frames centered over it like wallpaper
// (t-rec parity). Empty or "transparent" keeps transparency; anything
// unparseable errors.
// cfg.StartPause/EndPause hold the first/last frame (ms). Only frames that
// change are encoded (dirty rectangles over a shared palette); pixel-equal
// frames merge their delay into the previous frame. Progress and the final
// summary go to stderr unless quiet.
func Render(rec recording.File, outPath string, qualityOverride int, step int, pageBG string, quiet bool) (string, error) {
	if step <= 0 {
		step = 1
	}
	if len(rec.Records) == 0 {
		return "", fmt.Errorf("no records to render")
	}
	cfg := rec.Config
	// Adjust delays like ref/commands/render.js via play.adjustFramesDelays.
	timing.Adjust(rec.Records, timing.Options{
		FrameDelayAuto: cfg.FrameDelay.IsAuto,
		FrameDelay:     cfg.FrameDelay.Value,
		MaxIdleAuto:    cfg.MaxIdleTime.IsAuto,
		MaxIdle:        cfg.MaxIdleTime.Value,
		SpeedFactor:    1,
	})

	quality := cfg.Quality
	if qualityOverride > 0 {
		quality = qualityOverride
	}
	numColors := qualityToColors(quality)

	// Hold pauses (t-rec start-pause/end-pause, max 5min like t-rec).
	startPause, endPause := cfg.StartPause, cfg.EndPause
	if startPause < 0 {
		startPause = 0
	}
	if endPause < 0 {
		endPause = 0
	}
	if startPause > maxPauseMS {
		startPause = maxPauseMS
	}
	if endPause > maxPauseMS {
		endPause = maxPauseMS
	}

	cols := cfg.Cols.Value
	rows := cfg.Rows.Value
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	term := NewTerminal(cols, rows)
	type pending struct {
		snap      [][]Cell
		x, y      int
		cursorVis bool
		delay     int // next record's delay (ref/render.js:237)
		own       int // this record's own delay (last frame holds it)
	}
	var pend []pending
	stepsCounter := 0
	total := len(rec.Records)
	if !quiet {
		fmt.Fprintf(os.Stderr, "termrec: rendering %d records (step %d)...\n", total, step)
	}
	kept := 0
	for i, r := range rec.Records {
		term.Feed(r.Content)
		if stepsCounter != 0 {
			stepsCounter = (stepsCounter + 1) % step
			continue
		}
		stepsCounter = (stepsCounter + 1) % step
		snap := term.Snapshot()
		nextDelay := rec.Records[(i+1)%len(rec.Records)].Delay
		pend = append(pend, pending{snap, term.CurX, term.CurY, term.CursorVisible, int(nextDelay), int(r.Delay)})
		kept++
		if !quiet && (kept == 1 || kept%25 == 0) {
			fmt.Fprintf(os.Stderr, "termrec: kept frame %d (record %d/%d)...\n", kept, i+1, total)
		}
	}
	type frame struct {
		img   *image.RGBA
		delay int
	}
	var frames []frame
	for j, p := range pend {
		d := p.delay
		if j == len(pend)-1 {
			// Last frame holds its own delay instead of wrapping to the
			// first record: the ending spans to the real end (t-rec's
			// timeline rule), it doesn't borrow the opening pause.
			d = p.own
		}
		// Idle blink expansion: hidden phases only when the terminal
		// cursor itself is visible (?25h state); hidden cursors keep one
		// frame per record exactly as before.
		chunks := []int{d}
		if p.cursorVis {
			chunks = blinkChunks(d)
		}
		for k, cd := range chunks {
			// DrawFrame composes at 2x + downscales (smooth) and applies the
			// watermark frame-relative (CSS: prepended to .terminalizer-frame).
			img := DrawFrame(p.snap, p.x, p.y, p.cursorVis && k%2 == 0, cfg)
			if pageBG != "" {
				var err error
				if img, err = resolvePage(img, pageBG); err != nil {
					return "", err
				}
			}
			frames = append(frames, frame{img: img, delay: cd})
		}
	}
	if len(frames) == 0 {
		return "", fmt.Errorf("step filtered out all frames")
	}
	if startPause > 0 {
		frames[0].delay += startPause
	}
	if endPause > 0 {
		frames[len(frames)-1].delay += endPause
	}

	// Identical-frame excision + dirty rectangles (t-rec's idle dedup and
	// ImageMagick's -layers Optimize, reimplemented for stdlib image/gif):
	// a frame pixel-equal to the previous merges its delay into it, and
	// changed frames encode only their dirty bbox over the shared palette.
	type eframe struct {
		img   *image.RGBA
		delay int
		rect  image.Rectangle
	}
	var efs []eframe
	for _, fr := range frames {
		if len(efs) > 0 {
			if r, ok := diffRect(efs[len(efs)-1].img, fr.img); ok {
				efs = append(efs, eframe{fr.img, fr.delay, r})
			} else {
				efs[len(efs)-1].delay += fr.delay
			}
			continue
		}
		efs = append(efs, eframe{fr.img, fr.delay, fr.img.Bounds()})
	}

	out := &gif.GIF{LoopCount: repeatToLoopCount(cfg.Repeat)}
	// One global palette for every frame: exact theme/chrome anchors first
	// (flat fills stay bit-exact = color consistency), then frequent colors.
	// Sharing the palette across frames also stops per-frame index drift.
	imgs := make([]*image.RGBA, len(efs))
	for i, ef := range efs {
		imgs[i] = ef.img
	}
	transparent := anyTransparency(imgs)
	pal := buildGlobalPalette(imgs, anchorColors(cfg), numColors, transparent)
	for _, ef := range efs {
		d10 := int(ef.delay / 10)
		if d10 < 2 {
			d10 = 2 // min 20ms; viewers ignore 0/1
		}
		if d10 > 65535 {
			d10 = 65535
		}
		pm := ditherFSRect(ef.img, ef.rect, pal, transparent)
		out.Image = append(out.Image, pm)
		out.Delay = append(out.Delay, d10)
		// Full snapshots: leave previous in place (matches terminal frames).
		out.Disposal = append(out.Disposal, 1)
	}
	if outPath == "" {
		outPath = config.ResolveFilePath("render", "gif")
	}
	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := gif.EncodeAll(f, out); err != nil {
		return "", err
	}
	if st, serr := f.Stat(); serr == nil && !quiet {
		totalMS := 0
		for _, ef := range efs {
			totalMS += ef.delay
		}
		fmt.Fprintf(os.Stderr, "termrec: %d frames, %.1fs, %d bytes -> %s\n",
			len(efs), float64(totalMS)/1000, st.Size(), outPath)
	}
	return outPath, nil
}

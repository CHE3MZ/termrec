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

func repeatToLoopCount(r int) int {
	if r == -1 {
		return 1 // play once (no infinite loop); 0 == infinite in GIF
	}
	if r < -1 {
		return 0
	}
	return r
}

// Render converts a recording to an animated GIF.
// Transparency is preserved by default: theme background "transparent"
// (the default) yields transparent GIF pixels. Pass pageBG (any CSS color:
// name, #hex, rgb(), hsl()) to flatten frames onto an opaque page instead,
// e.g. "white" for the classic backgrounded look with a smooth shadow.
// Empty or "transparent" keeps transparency; anything unparseable errors.
func Render(rec recording.File, outPath string, qualityOverride int, step int, pageBG string) (string, error) {
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

	cols := cfg.Cols.Value
	rows := cfg.Rows.Value
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	term := NewTerminal(cols, rows)
	type frame struct {
		img   *image.RGBA
		delay int
	}
	var frames []frame
	stepsCounter := 0
	for i, r := range rec.Records {
		term.Feed(r.Content)
		if stepsCounter != 0 {
			stepsCounter = (stepsCounter + 1) % step
			continue
		}
		stepsCounter = (stepsCounter + 1) % step
		snap := term.Snapshot()
		// DrawFrame composes at 2x + downscales (smooth) and applies the
		// watermark frame-relative (CSS: prepended to .terminalizer-frame).
		img := DrawFrame(snap, term.CurX, term.CurY, term.CursorVisible, cfg)
		if pageBG != "" {
			var err error
			if img, err = flattenOnto(img, pageBG); err != nil {
				return "", err
			}
		}
		// delay = next frame's delay (ref/render.js:237), % len for last frame
		nextDelay := rec.Records[(i+1)%len(rec.Records)].Delay
		frames = append(frames, frame{img: img, delay: int(nextDelay)})
	}
	if len(frames) == 0 {
		return "", fmt.Errorf("step filtered out all frames")
	}

	out := &gif.GIF{LoopCount: repeatToLoopCount(cfg.Repeat)}
	// One global palette for every frame: exact theme/chrome anchors first
	// (flat fills stay bit-exact = color consistency), then frequent colors.
	// Sharing the palette across frames also stops per-frame index drift.
	imgs := make([]*image.RGBA, len(frames))
	for i, fr := range frames {
		imgs[i] = fr.img
	}
	transparent := anyTransparency(imgs)
	pal := buildGlobalPalette(imgs, anchorColors(cfg), numColors, transparent)
	for _, fr := range frames {
		d10 := int(fr.delay / 10)
		if d10 < 2 {
			d10 = 2 // min 20ms; viewers ignore 0/1
		}
		if d10 > 65535 {
			d10 = 65535
		}
		pm := ditherFS(fr.img, pal, transparent)
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
	return outPath, nil
}

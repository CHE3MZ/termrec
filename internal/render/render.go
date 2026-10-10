package render

import (
	"fmt"
	"image"
	"image/gif"
	"os"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/recording"
	"github.com/CHE3MZ/termrec/internal/timing"
)

// qualityToColors maps 1-100 to GIF palette size. stdlib has no quality knob.
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
func Render(rec recording.File, outPath string, qualityOverride int, step int) (string, error) {
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
		img := DrawFrame(snap, term.CurX, term.CurY, cfg)
		img = applyWatermark(img, cfg)
		img = flatten(img)
		// delay = next frame's delay (ref/render.js:237), % len for last frame
		nextDelay := rec.Records[(i+1)%len(rec.Records)].Delay
		frames = append(frames, frame{img: img, delay: int(nextDelay)})
	}
	if len(frames) == 0 {
		return "", fmt.Errorf("step filtered out all frames")
	}

	out := &gif.GIF{LoopCount: repeatToLoopCount(cfg.Repeat)}
	for _, fr := range frames {
		d10 := int(fr.delay / 10)
		if d10 < 2 {
			d10 = 2 // min 20ms; viewers ignore 0/1
		}
		if d10 > 65535 {
			d10 = 65535
		}
		// Quantize: use palette with numColors via Floyd-Steinberg? stdlib gif.Encode needs Paletted.
		// Build paletted frame via median-cut-ish: use image palette via gif.Encode with NumColors option is not exposed,
		// so dither manually by encoding through a temp GIF with palette limit.
		pm := image.NewPaletted(fr.img.Bounds(), paletteFor(fr.img, numColors))
		ditherCopy(pm, fr.img)
		out.Image = append(out.Image, pm)
		out.Delay = append(out.Delay, d10)
		// Disposal: leave previous (1 = do not dispose) matches terminal frames
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

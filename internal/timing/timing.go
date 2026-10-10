// Package timing adjusts frame delays. Port of ref/commands/play.js adjustFramesDelays.
package timing

import "github.com/CHE3MZ/termrec/internal/recording"

// Options mirrors play.js options.
type Options struct {
	// FrameDelay <0 means auto. >=0 fixed ms.
	FrameDelayAuto bool
	FrameDelay     float64
	// MaxIdle <0 means auto (no cap).
	MaxIdleAuto bool
	MaxIdle     float64
	SpeedFactor float64
}

// Adjust mutates delays in place.
func Adjust(records []recording.Record, o Options) {
	if o.SpeedFactor == 0 {
		o.SpeedFactor = 1
	}
	for i := range records {
		d := float64(records[i].Delay)
		if !o.FrameDelayAuto {
			d = o.FrameDelay
		} else if !o.MaxIdleAuto && d > o.MaxIdle {
			d = o.MaxIdle
		}
		d *= o.SpeedFactor
		if d < 1 {
			d = 1
		}
		records[i].Delay = int64(d)
	}
}

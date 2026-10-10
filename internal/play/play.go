// Package play replays a recording to stdout. Port of ref/commands/play.js.
package play

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/CHE3MZ/termrec/internal/recording"
	"github.com/CHE3MZ/termrec/internal/timing"
)

// Run replays records with delays. realTiming uses recorded delays uncapped.
func Run(rec recording.File, realTiming bool, speedFactor float64) {
	if speedFactor <= 0 {
		speedFactor = 1
	}
	opts := timing.Options{
		FrameDelayAuto: rec.Config.FrameDelay.IsAuto,
		FrameDelay:     rec.Config.FrameDelay.Value,
		MaxIdleAuto:    rec.Config.MaxIdleTime.IsAuto,
		MaxIdle:        rec.Config.MaxIdleTime.Value,
		SpeedFactor:    speedFactor,
	}
	if realTiming {
		opts.FrameDelayAuto = true
		opts.MaxIdleAuto = true
	}
	timing.Adjust(rec.Records, opts)

	done := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		fmt.Print("\033c")
		close(done)
		os.Exit(0)
	}()

	for _, r := range rec.Records {
		select {
		case <-done:
			return
		default:
		}
		if r.Delay > 0 {
			time.Sleep(time.Duration(float64(r.Delay)*float64(time.Millisecond)))
		}
		fmt.Print(r.Content)
	}
	fmt.Print("\033c")
}

package timing

import (
	"testing"

	"github.com/CHE3MZ/termrec/internal/recording"
)

func recs(delays ...int64) []recording.Record {
	out := make([]recording.Record, len(delays))
	for i, d := range delays {
		out[i] = recording.Record{Delay: d, Content: "x"}
	}
	return out
}

func TestAutoWithCap(t *testing.T) {
	r := recs(100, 5000, 200)
	Adjust(r, Options{FrameDelayAuto: true, MaxIdleAuto: false, MaxIdle: 2000, SpeedFactor: 1})
	if r[0].Delay != 100 || r[1].Delay != 2000 || r[2].Delay != 200 {
		t.Fatalf("got %v", r)
	}
}

func TestFixed(t *testing.T) {
	r := recs(100, 5000)
	Adjust(r, Options{FrameDelayAuto: false, FrameDelay: 150, MaxIdleAuto: true, SpeedFactor: 1})
	if r[0].Delay != 150 || r[1].Delay != 150 {
		t.Fatalf("got %v", r)
	}
}

func TestSpeedFactor(t *testing.T) {
	r := recs(100)
	Adjust(r, Options{FrameDelayAuto: true, MaxIdleAuto: true, SpeedFactor: 2})
	if r[0].Delay != 200 {
		t.Fatalf("got %v", r)
	}
}

func TestAutoNoCap(t *testing.T) {
	r := recs(9999)
	Adjust(r, Options{FrameDelayAuto: true, MaxIdleAuto: true, SpeedFactor: 1})
	if r[0].Delay != 9999 {
		t.Fatalf("got %v", r)
	}
}

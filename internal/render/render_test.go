package render

import "testing"

// Loop count mapping: repeat -1 -> -1 (once), 0 -> 0 (forever), n -> n.
func TestRenderRepeatMapping(t *testing.T) {
	for _, tc := range []struct {
		repeat int
		want   int
	}{
		{-1, -1}, {0, 0}, {3, 3},
	} {
		if got := repeatToLoopCount(tc.repeat); got != tc.want {
			t.Fatalf("repeat %d -> %d want %d", tc.repeat, got, tc.want)
		}
	}
}

// Idle blink chunking: a solid 700ms grace, then ~600ms off/on phases.
// Pauses under ~1s stay single; huge pauses cap out; totals stay exact.
func TestBlinkChunks(t *testing.T) {
	cases := []struct {
		delay int
		want  []int
	}{
		{100, []int{100}},
		{500, []int{500}},
		{800, []int{800}},
		{1050, []int{1050}},
		{1200, []int{700, 500}},
		{1500, []int{700, 400, 400}},
		{2000, []int{700, 433, 433, 434}},
	}
	for _, tc := range cases {
		got := blinkChunks(tc.delay)
		if len(got) != len(tc.want) {
			t.Fatalf("delay %d: %v want %v", tc.delay, got, tc.want)
		}
		sum := 0
		for i, d := range got {
			sum += d
			if d != tc.want[i] {
				t.Fatalf("delay %d: %v want %v", tc.delay, got, tc.want)
			}
		}
		if sum != tc.delay {
			t.Fatalf("delay %d: chunks sum %d", tc.delay, sum)
		}
	}
	if got := blinkChunks(60000); len(got) != maxBlinkPhases {
		t.Fatalf("huge delay: %d phases want cap %d", len(got), maxBlinkPhases)
	} else {
		sum := 0
		for _, d := range got {
			sum += d
		}
		if sum != 60000 {
			t.Fatalf("huge delay: chunks sum %d", sum)
		}
	}
}

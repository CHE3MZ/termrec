package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePauseMS(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
		ok   bool
	}{
		{"2s", 2000, true},
		{"500ms", 500, true},
		{"1500", 1500, true},
		{"1m", 60000, true},
		{"5m", 300000, true},
		{"0", 0, true},
		{"10m", 0, false},      // over the 5min cap
		{"-1s", 0, false},      // negative
		{"abc", 0, false},      // not a duration
		{"", 0, false},         // empty
		{" 750ms ", 750, true}, // surrounding space tolerated
	} {
		got, err := parsePauseMS(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Fatalf("%q: got %d,%v want %d,nil", tc.in, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q: want error, got %d", tc.in, got)
		}
	}
}

func TestNextFreePath(t *testing.T) {
	dir := t.TempDir()
	free := filepath.Join(dir, "render.gif")
	if got := nextFreePath(free); got != free {
		t.Fatalf("free path changed: %q", got)
	}
	if err := os.WriteFile(free, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, want := nextFreePath(free), filepath.Join(dir, "render_1.gif"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if err := os.WriteFile(filepath.Join(dir, "render_1.gif"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, want := nextFreePath(free), filepath.Join(dir, "render_2.gif"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

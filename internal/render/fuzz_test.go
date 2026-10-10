package render

import "testing"

func FuzzTermFeed(f *testing.F) {
	seeds := []string{
		"hello",
		"\x1b[31mred\x1b[0m",
		"\x1b[2J\x1b[H*",
		"line1\nline2\r\n\x1b[1;32mbold",
		"\x1b[2;3Hpos\x1b[2K",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		term := NewTerminal(80, 24)
		term.Feed(s) // must not panic
		_ = term.Snapshot()
	})
}

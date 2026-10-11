package record

import "testing"

func TestContainsHotkey(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
		want bool
	}{
		{"ss3 f2", []byte("\x1bOQ"), true},
		{"csi f2", []byte("ab\x1b[12~cd"), true},
		{"plain", []byte("hello"), false},
		{"partial ss3", []byte("\x1bO"), false},
		{"esc alone", []byte("\x1b[A"), false},
		{"empty", []byte{}, false},
	} {
		if got := containsHotkey(tc.in); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

package theme

import "testing"

func TestParseVariants(t *testing.T) {
	cases := []struct {
		in      string
		r, g, b uint8
		trans   bool
	}{
		{"#fff", 255, 255, 255, false},
		{"#000", 0, 0, 0, false},
		{"#fc4384", 0xfc, 0x43, 0x84, false},
		{"transparent", 0, 0, 0, true},
		{"red", 255, 0, 0, false},
		{"rgb(255, 0, 0)", 255, 0, 0, false},
		{"hsl(0, 100%, 50%)", 255, 0, 0, false},
		{"hsl(120, 100%, 50%)", 0, 255, 0, false},
		{"hsl(240, 100%, 50%)", 0, 0, 255, false},
	}
	for _, c := range cases {
		got, trans, err := Parse(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if trans != c.trans {
			t.Fatalf("%q trans=%v want %v", c.in, trans, c.trans)
		}
		if !trans && (got.R != c.r || got.G != c.g || got.B != c.b) {
			t.Fatalf("%q = %+v want %02x%02x%02x", c.in, got, c.r, c.g, c.b)
		}
	}
}

func TestParseAlpha(t *testing.T) {
	_, trans, err := Parse("#ff000080")
	if err != nil {
		t.Fatal(err)
	}
	if trans {
		t.Fatalf("half alpha is not fully transparent")
	}
	if _, trans, _ := Parse("rgba(255,0,0,0)"); !trans {
		t.Fatalf("zero alpha should be transparent")
	}
}

func TestXterm256(t *testing.T) {
	c := Xterm256(196)
	if c.R != 255 || c.G != 0 || c.B != 0 {
		t.Fatalf("196 should be red: %+v", c)
	}
	g := Xterm256(232)
	if g.R != 8 || g.G != 8 || g.B != 8 {
		t.Fatalf("232 gray: %+v", g)
	}
}

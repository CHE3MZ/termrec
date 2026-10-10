// Package theme parses terminalizer theme colors.
package theme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// Names in ref/config.yml order.
var Names = []string{
	"background", "foreground", "cursor",
	"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
	"brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue",
	"brightMagenta", "brightCyan", "brightWhite",
}

// Defaults from ref/config.yml.
func Defaults() map[string]string {
	return map[string]string{
		"background": "transparent", "foreground": "#afafaf", "cursor": "#c7c7c7",
		"black": "#232628", "red": "#fc4384", "green": "#b3e33b", "yellow": "#ffa727",
		"blue": "#75dff2", "magenta": "#ae89fe", "cyan": "#708387", "white": "#d5d5d0",
		"brightBlack": "#626566", "brightRed": "#ff7fac", "brightGreen": "#c8ed71",
		"brightYellow": "#ebdf86", "brightBlue": "#75dff2", "brightMagenta": "#ae89fe",
		"brightCyan": "#b1c6ca", "brightWhite": "#f9f9f4",
	}
}

// Parse a CSS-ish color: #rgb, #rrggbb, rgb(...), named few, transparent.
// Returns (c, isTransparent, err).
func Parse(s string) (color.RGBA, bool, error) {
	t := strings.TrimSpace(strings.ToLower(s))
	if t == "" || t == "transparent" {
		return color.RGBA{}, true, nil
	}
	named := map[string]string{
		"black": "#000000", "white": "#ffffff", "red": "#ff0000",
		"green": "#00ff00", "blue": "#0000ff", "magenta": "#ff00ff",
		"cyan": "#00ffff", "yellow": "#ffff00",
	}
	if v, ok := named[t]; ok {
		t = v
	}
	if strings.HasPrefix(t, "#") {
		h := strings.TrimPrefix(t, "#")
		if len(h) == 3 {
			h = fmt.Sprintf("%c%c%c%c%c%c", h[0], h[0], h[1], h[1], h[2], h[2])
		}
		if len(h) != 6 {
			return color.RGBA{}, false, fmt.Errorf("bad hex color %q", s)
		}
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return color.RGBA{}, false, err
		}
		return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}, false, nil
	}
	if strings.HasPrefix(t, "rgb(") && strings.HasSuffix(t, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(t, "rgb("), ")")
		parts := strings.Split(inner, ",")
		if len(parts) != 3 {
			return color.RGBA{}, false, fmt.Errorf("bad rgb color %q", s)
		}
		var c color.RGBA
		c.A = 0xff
		for i, p := range parts {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(p), "%")))
			if err != nil {
				return color.RGBA{}, false, err
			}
			switch i {
			case 0:
				c.R = uint8(n)
			case 1:
				c.G = uint8(n)
			case 2:
				c.B = uint8(n)
			}
		}
		return c, false, nil
	}
	return color.RGBA{}, false, fmt.Errorf("unsupported color %q", s)
}

// Map merges user theme over defaults and parses all.
func Map(user map[string]string) (map[string]color.RGBA, map[string]bool) {
	def := Defaults()
	for k, v := range user {
		def[k] = v
	}
	out := map[string]color.RGBA{}
	trans := map[string]bool{}
	for _, k := range Names {
		c, t, err := Parse(def[k])
		if err != nil {
			c = color.RGBA{R: 0xaf, G: 0xaf, B: 0xaf, A: 0xff}
			t = false
		}
		out[k] = c
		trans[k] = t
	}
	return out, trans
}

// Package theme parses terminalizer theme colors.
// Supports Hex (#rgb, #rrggbb, #rrggbbaa), rgb()/rgba(), hsl()/hsla(),
// CSS named colors, and "transparent" (as in ref/config.yml + README).
package theme

import (
	"fmt"
	"image/color"
	"math"
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

// Defaults: Dark (Visual Studio) palette; background stays transparent so
// GIFs render transparent by default.
func Defaults() map[string]string {
	return map[string]string{
		"background": "transparent", "foreground": "#bbbbbb", "cursor": "#c7c7c7",
		"black": "#000000", "red": "#cd3131", "green": "#0dbc79", "yellow": "#e5e510",
		"blue": "#2472c8", "magenta": "#bc3fbc", "cyan": "#11a8cd", "white": "#e5e5e5",
		"brightBlack": "#666666", "brightRed": "#f14c4c", "brightGreen": "#23d18b",
		"brightYellow": "#f5f543", "brightBlue": "#3b8eea", "brightMagenta": "#d670d6",
		"brightCyan": "#29b8db", "brightWhite": "#e5e5e5",
	}
}

// cssNames covers the CSS color keywords (README: Hex, RGB, HSL, Name).
var cssNames = map[string]string{
	"black": "#000000", "silver": "#c0c0c0", "gray": "#808080", "grey": "#808080",
	"white": "#ffffff", "maroon": "#800000", "red": "#ff0000", "purple": "#800080",
	"fuchsia": "#ff00ff", "magenta": "#ff00ff", "green": "#008000", "lime": "#00ff00",
	"olive": "#808000", "yellow": "#ffff00", "navy": "#000080", "blue": "#0000ff",
	"teal": "#008080", "aqua": "#00ffff", "cyan": "#00ffff", "orange": "#ffa500",
	"aliceblue": "#f0f8ff", "antiquewhite": "#faebd7", "aquamarine": "#7fffd4",
	"azure": "#f0ffff", "beige": "#f5f5dc", "bisque": "#ffe4c4", "blanchedalmond": "#ffebcd",
	"blueviolet": "#8a2be2", "brown": "#a52a2a", "burlywood": "#deb887",
	"cadetblue": "#5f9ea0", "chartreuse": "#7fff00", "chocolate": "#d2691e",
	"coral": "#ff7f50", "cornflowerblue": "#6495ed", "cornsilk": "#fff8dc",
	"crimson": "#dc143c", "darkblue": "#00008b", "darkcyan": "#008b8b",
	"darkgoldenrod": "#b8860b", "darkgray": "#a9a9a9", "darkgreen": "#006400",
	"darkgrey": "#a9a9a9", "darkkhaki": "#bdb76b", "darkmagenta": "#8b008b",
	"darkolivegreen": "#556b2f", "darkorange": "#ff8c00", "darkorchid": "#9932cc",
	"darkred": "#8b0000", "darksalmon": "#e9967a", "darkseagreen": "#8fbc8f",
	"darkslateblue": "#483d8b", "darkslategray": "#2f4f4f", "darkslategrey": "#2f4f4f",
	"darkturquoise": "#00ced1", "darkviolet": "#9400d3", "deeppink": "#ff1493",
	"deepskyblue": "#00bfff", "dimgray": "#696969", "dimgrey": "#696969",
	"dodgerblue": "#1e90ff", "firebrick": "#b22222", "floralwhite": "#fffaf0",
	"forestgreen": "#228b22", "gainsboro": "#dcdcdc", "ghostwhite": "#f8f8ff",
	"gold": "#ffd700", "goldenrod": "#daa520", "greenyellow": "#adff2f",
	"honeydew": "#f0fff0", "hotpink": "#ff69b4", "indianred": "#cd5c5c",
	"indigo": "#4b0082", "ivory": "#fffff0", "khaki": "#f0e68c",
	"lavender": "#e6e6fa", "lavenderblush": "#fff0f5", "lawngreen": "#7cfc00",
	"lemonchiffon": "#fffacd", "lightblue": "#add8e6", "lightcoral": "#f08080",
	"lightcyan": "#e0ffff", "lightgoldenrodyellow": "#fafad2", "lightgray": "#d3d3d3",
	"lightgreen": "#90ee90", "lightgrey": "#d3d3d3", "lightpink": "#ffb6c1",
	"lightsalmon": "#ffa07a", "lightseagreen": "#20b2aa", "lightskyblue": "#87cefa",
	"lightslategray": "#778899", "lightslategrey": "#778899", "lightsteelblue": "#b0c4de",
	"lightyellow": "#ffffe0", "limegreen": "#32cd32", "linen": "#faf0e6",
	"mediumaquamarine": "#66cdaa", "mediumblue": "#0000cd", "mediumorchid": "#ba55d3",
	"mediumpurple": "#9370db", "mediumseagreen": "#3cb371", "mediumslateblue": "#7b68ee",
	"mediumspringgreen": "#00fa9a", "mediumturquoise": "#48d1cc", "mediumvioletred": "#c71585",
	"midnightblue": "#191970", "mintcream": "#f5fffa", "mistyrose": "#ffe4e1",
	"moccasin": "#ffe4b5", "navajowhite": "#ffdead", "oldlace": "#fdf5e6",
	"olivedrab": "#6b8e23", "orangered": "#ff4500", "orchid": "#da70d6",
	"palegoldenrod": "#eee8aa", "palegreen": "#98fb98", "paleturquoise": "#afeeee",
	"palevioletred": "#db7093", "papayawhip": "#ffefd5", "peachpuff": "#ffdab9",
	"peru": "#cd853f", "pink": "#ffc0cb", "plum": "#dda0dd", "powderblue": "#b0e0e6",
	"rosybrown": "#bc8f8f", "royalblue": "#4169e1", "saddlebrown": "#8b4513",
	"salmon": "#fa8072", "sandybrown": "#f4a460", "seagreen": "#2e8b57",
	"seashell": "#fff5ee", "sienna": "#a0522d", "skyblue": "#87ceeb",
	"slateblue": "#6a5acd", "slategray": "#708090", "slategrey": "#708090",
	"snow": "#fffafa", "springgreen": "#00ff7f", "steelblue": "#4682b4",
	"tan": "#d2b48c", "thistle": "#d8bfd8", "tomato": "#ff6347",
	"turquoise": "#40e0d0", "violet": "#ee82ee", "wheat": "#f5deb3",
	"whitesmoke": "#f5f5f5", "yellowgreen": "#9acd32",
}

// Parse a CSS-ish color: #rgb, #rrggbb, #rrggbbaa, rgb(), rgba(), hsl(), hsla(), names, transparent.
// Returns (c, isTransparent, err). Transparent yields zero color + true.
func Parse(s string) (color.RGBA, bool, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return color.RGBA{}, true, nil
	}
	low := strings.ToLower(t)
	if low == "transparent" || low == "null" || low == "none" {
		return color.RGBA{}, true, nil
	}
	if v, ok := cssNames[low]; ok {
		t = v
		low = strings.ToLower(t)
	}
	if strings.HasPrefix(low, "#") {
		return parseHex(t)
	}
	if strings.HasPrefix(low, "rgb(") || strings.HasPrefix(low, "rgba(") {
		return parseRGB(t)
	}
	if strings.HasPrefix(low, "hsl(") || strings.HasPrefix(low, "hsla(") {
		return parseHSL(t)
	}
	return color.RGBA{}, false, fmt.Errorf("unsupported color %q", s)
}

func parseHex(s string) (color.RGBA, bool, error) {
	h := strings.TrimSpace(s)
	h = strings.TrimPrefix(strings.TrimPrefix(h, "#"), "#")
	if len(h) == 3 {
		h = fmt.Sprintf("%c%c%c%c%c%c", h[0], h[0], h[1], h[1], h[2], h[2])
	}
	if len(h) == 4 { // #rgba short
		h = fmt.Sprintf("%c%c%c%c%c%c%c%c", h[0], h[0], h[1], h[1], h[2], h[2], h[3], h[3])
	}
	switch len(h) {
	case 6:
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return color.RGBA{}, false, err
		}
		return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}, false, nil
	case 8:
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return color.RGBA{}, false, err
		}
		c := color.RGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}
		if c.A == 0 {
			return color.RGBA{}, true, nil
		}
		return c, false, nil
	}
	return color.RGBA{}, false, fmt.Errorf("bad hex color %q", s)
}

func splitFuncArgs(s string) []string {
	open := strings.Index(s, "(")
	close := strings.LastIndex(s, ")")
	if open < 0 || close < 0 || close <= open {
		return nil
	}
	inner := s[open+1 : close]
	// rgb() allows space or comma or slash-alpha separators
	inner = strings.ReplaceAll(inner, "/", ",")
	parts := strings.Split(inner, ",")
	if len(parts) == 1 {
		parts = strings.Fields(inner)
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func num255(p string) (uint8, error) {
	p = strings.TrimSpace(p)
	if strings.HasSuffix(p, "%") {
		f, err := strconv.ParseFloat(strings.TrimSuffix(p, "%"), 64)
		if err != nil {
			return 0, err
		}
		return uint8(math.Round(f * 255 / 100)), nil
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		f, ferr := strconv.ParseFloat(p, 64)
		if ferr != nil {
			return 0, err
		}
		n = int(math.Round(f))
	}
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	return uint8(n), nil
}

func alpha(p string) (uint8, error) {
	p = strings.TrimSpace(p)
	if strings.HasSuffix(p, "%") {
		f, err := strconv.ParseFloat(strings.TrimSuffix(p, "%"), 64)
		if err != nil {
			return 0, err
		}
		return uint8(math.Round(f * 255 / 100)), nil
	}
	f, err := strconv.ParseFloat(p, 64)
	if err != nil {
		return 0, err
	}
	if f <= 1 {
		return uint8(math.Round(f * 255)), nil
	}
	if f < 0 {
		f = 0
	}
	if f > 255 {
		f = 255
	}
	return uint8(math.Round(f)), nil
}

func parseRGB(s string) (color.RGBA, bool, error) {
	parts := splitFuncArgs(s)
	if len(parts) < 3 {
		return color.RGBA{}, false, fmt.Errorf("bad rgb color %q", s)
	}
	r, err := num255(parts[0])
	if err != nil {
		return color.RGBA{}, false, err
	}
	g, err := num255(parts[1])
	if err != nil {
		return color.RGBA{}, false, err
	}
	b, err := num255(parts[2])
	if err != nil {
		return color.RGBA{}, false, err
	}
	a := uint8(0xff)
	if len(parts) >= 4 && strings.TrimSpace(parts[3]) != "" {
		a, err = alpha(parts[3])
		if err != nil {
			return color.RGBA{}, false, err
		}
	}
	if a == 0 {
		return color.RGBA{}, true, nil
	}
	return color.RGBA{R: r, G: g, B: b, A: a}, false, nil
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	default:
		return p
	}
}

func parseHSL(s string) (color.RGBA, bool, error) {
	parts := splitFuncArgs(s)
	if len(parts) < 3 {
		return color.RGBA{}, false, fmt.Errorf("bad hsl color %q", s)
	}
	hStr := strings.TrimSuffix(strings.TrimSpace(parts[0]), "deg")
	h, err := strconv.ParseFloat(strings.TrimSpace(hStr), 64)
	if err != nil {
		return color.RGBA{}, false, err
	}
	pct := func(p string) (float64, error) {
		p = strings.TrimSpace(p)
		p = strings.TrimSuffix(p, "%")
		return strconv.ParseFloat(p, 64)
	}
	sv, err := pct(parts[1])
	if err != nil {
		return color.RGBA{}, false, err
	}
	lv, err := pct(parts[2])
	if err != nil {
		return color.RGBA{}, false, err
	}
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	h /= 360
	sv /= 100
	lv /= 100
	var r, g, b float64
	if sv == 0 {
		r, g, b = lv, lv, lv
	} else {
		var q float64
		if lv < 0.5 {
			q = lv * (1 + sv)
		} else {
			q = lv + sv - lv*sv
		}
		p := 2*lv - q
		r = hueToRGB(p, q, h+1.0/3)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-1.0/3)
	}
	a := uint8(0xff)
	if len(parts) >= 4 && strings.TrimSpace(parts[3]) != "" {
		a, err = alpha(parts[3])
		if err != nil {
			return color.RGBA{}, false, err
		}
	}
	if a == 0 {
		return color.RGBA{}, true, nil
	}
	return color.RGBA{
		R: uint8(math.Round(r * 255)),
		G: uint8(math.Round(g * 255)),
		B: uint8(math.Round(b * 255)),
		A: a,
	}, false, nil
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

// Xterm256 returns the standard xterm 256-color palette entry.
func Xterm256(n int) color.RGBA {
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	// 0-7 standard, 8-15 bright (xterm values)
	std := [16][3]uint8{
		{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
		{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
		{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
		{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
	if n < 16 {
		c := std[n]
		return color.RGBA{R: c[0], G: c[1], B: c[2], A: 0xff}
	}
	if n < 232 {
		n -= 16
		levels := [6]uint8{0, 95, 135, 175, 215, 255}
		return color.RGBA{R: levels[(n/36)%6], G: levels[(n/6)%6], B: levels[n%6], A: 0xff}
	}
	v := uint8(8 + (n-232)*10)
	return color.RGBA{R: v, G: v, B: v, A: 0xff}
}

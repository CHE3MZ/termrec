package render

import (
	"sync"

	"github.com/CHE3MZ/termrec/internal/config"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

var (
	fontCacheMu sync.Mutex
	fontCache   = map[string]*opentype.Font{}
	faceCache   = map[string]font.Face{}
)

func loadFont(name string, data []byte) (*opentype.Font, error) {
	fontCacheMu.Lock()
	defer fontCacheMu.Unlock()
	if f, ok := fontCache[name]; ok {
		return f, nil
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	fontCache[name] = f
	return f, nil
}

// monoFace returns a vector monospace face at fontSize px (Go Mono).
// Falls back to basicfont when parsing fails (keeps CI green in odd envs).
func monoFace(px float64) font.Face {
	if px <= 0 {
		px = 12
	}
	key := "mono-" + itoa(int(px*10))
	fontCacheMu.Lock()
	if f, ok := faceCache[key]; ok {
		fontCacheMu.Unlock()
		return f
	}
	fontCacheMu.Unlock()
	if ft, err := loadFont("gomono", gomono.TTF); err == nil {
		if face, err := opentype.NewFace(ft, &opentype.FaceOptions{
			Size:    px,
			DPI:     72,
			Hinting: font.HintingFull,
		}); err == nil {
			fontCacheMu.Lock()
			faceCache[key] = face
			fontCacheMu.Unlock()
			return face
		}
	}
	return basicfont.Face7x13
}

// titleFace returns a sans face for titlebars (Arial-like; Go Regular).
func titleFace() font.Face {
	fontCacheMu.Lock()
	if f, ok := faceCache["title-14"]; ok {
		fontCacheMu.Unlock()
		return f
	}
	fontCacheMu.Unlock()
	if ft, err := loadFont("goregular", goregular.TTF); err == nil {
		if face, err := opentype.NewFace(ft, &opentype.FaceOptions{
			Size:    14,
			DPI:     72,
			Hinting: font.HintingFull,
		}); err == nil {
			fontCacheMu.Lock()
			faceCache["title-14"] = face
			fontCacheMu.Unlock()
			return face
		}
	}
	return basicfont.Face7x13
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// cellMetrics measures the terminal cell from the real face.
// advance = "M" width + letterSpacing; height = (ascent+descent) * lineHeight.
func cellMetrics(cfg config.Config) (cellW, cellH, ascent int) {
	px := float64(cfg.FontSize)
	if px <= 0 {
		px = 12
	}
	face := monoFace(px)
	adv := font.MeasureString(face, "M").Ceil()
	if adv <= 0 {
		adv = 7
	}
	cellW = adv + cfg.LetterSpacing
	m := face.Metrics()
	ascent = m.Ascent.Ceil()
	descent := m.Descent.Ceil()
	if ascent <= 0 {
		ascent = 10
	}
	if descent < 0 {
		descent = 0
	}
	base := ascent + descent
	if base <= 0 {
		base = 13
	}
	lh := cfg.LineHeight
	if lh <= 0 {
		lh = 1
	}
	cellH = int(float64(base)*lh + 0.5)
	if cellW < 4 {
		cellW = 4
	}
	if cellH < 6 {
		cellH = 6
	}
	if ascent >= cellH {
		ascent = cellH - 2
	}
	return cellW, cellH, ascent
}

// cellSize keeps the old name for callers.
func cellSize(cfg config.Config) (int, int) {
	w, h, _ := cellMetrics(cfg)
	return w, h
}

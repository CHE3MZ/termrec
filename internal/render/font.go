package render

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	return monoFaceFor("", px)
}

// monoFaceFor resolves cfg.FontFamily (CSS-like list, first available wins)
// against installed system fonts, falling back to embedded Go Mono.
// An empty family uses Go Mono directly.
func monoFaceFor(family string, px float64) font.Face {
	if px <= 0 {
		px = 12
	}
	norm := normalizeFamily(family)
	key := "mono-" + norm + "-" + itoa(int(px*10))
	fontCacheMu.Lock()
	if f, ok := faceCache[key]; ok {
		fontCacheMu.Unlock()
		return f
	}
	fontCacheMu.Unlock()
	var ft *opentype.Font
	if norm != "" {
		if found, err := findSystemFont(family); err == nil && found != nil {
			ft = found
		}
	}
	if ft == nil {
		var err error
		ft, err = loadFont("gomono", gomono.TTF)
		if err != nil {
			return basicfont.Face7x13
		}
	}
	if face, err := opentype.NewFace(ft, &opentype.FaceOptions{
		Size:    px,
		DPI:     72,
		Hinting: font.HintingNone,
	}); err == nil {
		fontCacheMu.Lock()
		faceCache[key] = face
		fontCacheMu.Unlock()
		return face
	}
	return basicfont.Face7x13
}

// normalizeFamily canonicalizes a CSS font list for cache keys.
func normalizeFamily(family string) string {
	parts := strings.Split(family, ",")
	var out []string
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(strings.Trim(p, `"'`)))
		if p == "" {
			continue
		}
		var b strings.Builder
		for _, r := range p {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
		if b.Len() > 0 {
			out = append(out, b.String())
		}
	}
	return strings.Join(out, ",")
}

// familyFileAliases maps normalized family names to likely file stems
// (Windows ships some fonts under short 8.3-ish names, e.g. lucon.ttf).
func familyFileAliases() map[string][]string {
	return map[string][]string{
		"lucidaconsole":  {"lucon", "lucidaconsole"},
		"ubuntumono":     {"ubuntumono", "ubuntu-mono"},
		"dejavusansmono": {"dejavusansmono"},
		"liberationmono": {"liberationmono"},
		"robotomono":     {"robotomono"},
		"notosansmono":   {"notosansmono"},
		"couriernew":     {"cour", "couriernew"},
		"consolas":       {"consola", "consolas"},
		"cascadiamono":   {"cascadiamono"},
		"cascadiacode":   {"cascadiacode"},
		"menlo":          {"menlo"},
		"sfmono":         {"sfmono"},
		"monaco":         {"monaco"},
		"sourcemono":     {"sourcemono", "sourcecodepro"},
		"jetbrainsmono":  {"jetbrainsmono"},
		"firacode":       {"firacode"},
		"hack":           {"hack"},
		"inconsolata":    {"inconsolata"},
	}
}

// systemFontDirs lists OS font directories (plus TERMREC_FONT_DIRS for tests).
func systemFontDirs() []string {
	var dirs []string
	if extra := os.Getenv("TERMREC_FONT_DIRS"); extra != "" {
		dirs = append(dirs, filepath.SplitList(extra)...)
	}
	switch runtime.GOOS {
	case "windows":
		if w := os.Getenv("WINDIR"); w != "" {
			dirs = append(dirs, filepath.Join(w, "Fonts"))
		}
		dirs = append(dirs, `C:\Windows\Fonts`)
		if l := os.Getenv("LOCALAPPDATA"); l != "" {
			dirs = append(dirs, filepath.Join(l, "Microsoft", "Windows", "Fonts"))
		}
	case "darwin":
		dirs = append(dirs,
			"/System/Library/Fonts",
			"/Library/Fonts",
			"/System/Library/Fonts/Supplemental",
		)
		if h, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(h, "Library", "Fonts"))
		}
	default:
		dirs = append(dirs,
			"/usr/share/fonts",
			"/usr/local/share/fonts",
		)
		if h, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(h, ".fonts"), filepath.Join(h, ".local", "share", "fonts"))
		}
	}
	return dirs
}

// findSystemFont parses the CSS list in order and returns the first
// installed, parseable TTF/OTF match. The generic "monospace"/"mono" names
// fall back to common installed monos; a list with no generic and no exact
// match returns ErrNotExist so callers use the embedded Go Mono.
func findSystemFont(family string) (*opentype.Font, error) {
	var wants []string
	generic := false
	for _, p := range strings.Split(family, ",") {
		p = strings.ToLower(strings.TrimSpace(strings.Trim(p, `"'`)))
		if p == "" {
			continue
		}
		var b strings.Builder
		for _, r := range p {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
		if b.Len() > 0 {
			wants = append(wants, b.String())
		}
	}
	if len(wants) == 0 {
		return nil, os.ErrNotExist
	}
	aliases := familyFileAliases()
	// generic monospace last-resort candidates
	monos := []string{"dejavusansmono", "liberationmono", "consola", "consolas", "menlo", "ubuntumono", "robotomono", "notosansmono", "cour", "couriernew", "lucon", "cascadiamono"}
	expanded := make([]string, 0, len(wants)+len(monos))
	for _, w := range wants {
		if w == "monospace" || w == "mono" {
			generic = true
			continue
		}
		if al, ok := aliases[w]; ok {
			expanded = append(expanded, al...)
		} else {
			expanded = append(expanded, w)
		}
	}
	if generic {
		expanded = append(expanded, monos...)
	}
	seenDirs := map[string]struct{}{}
	for _, dir := range systemFontDirs() {
		if _, ok := seenDirs[dir]; ok {
			continue
		}
		seenDirs[dir] = struct{}{}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		// index dir once: normalized base name -> full path
		idx := map[string]string{}
		var files []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := strings.ToLower(e.Name())
			if !(strings.HasSuffix(n, ".ttf") || strings.HasSuffix(n, ".otf")) {
				continue
			}
			base := strings.TrimSuffix(strings.TrimSuffix(n, ".ttf"), ".otf")
			norm := ""
			for _, r := range base {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
					norm += string(r)
				}
			}
			if _, ok := idx[norm]; !ok {
				idx[norm] = filepath.Join(dir, e.Name())
			}
			files = append(files, norm)
		}
		for _, want := range expanded {
			// exact then prefix match (e.g. "consola" hits consola.ttf before consolab)
			candidates := []string{}
			if p, ok := idx[want]; ok {
				candidates = append(candidates, p)
			}
			for _, f := range files {
				if f != want && strings.HasPrefix(f, want) {
					candidates = append(candidates, idx[f])
				}
			}
			for _, path := range candidates {
				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}
				ft, err := opentype.Parse(data)
				if err != nil {
					continue
				}
				fontCacheMu.Lock()
				fontCache["sys:"+path] = ft
				fontCacheMu.Unlock()
				return ft, nil
			}
		}
	}
	return nil, os.ErrNotExist
}

// titleFacePx returns the sans face at the given pixel size.
func titleFacePx(px float64) font.Face {
	if px <= 0 {
		px = 14
	}
	key := "title-" + itoa(int(px*10))
	fontCacheMu.Lock()
	if f, ok := faceCache[key]; ok {
		fontCacheMu.Unlock()
		return f
	}
	fontCacheMu.Unlock()
	if ft, err := loadFont("goregular", goregular.TTF); err == nil {
		if face, err := opentype.NewFace(ft, &opentype.FaceOptions{
			Size:    px,
			DPI:     72,
			Hinting: font.HintingNone,
		}); err == nil {
			fontCacheMu.Lock()
			faceCache[key] = face
			fontCacheMu.Unlock()
			return face
		}
	}
	return basicfont.Face7x13
}

// renderSS is the supersampling factor: frames compose at 2x and are
// downscaled with area averaging for smooth text, shapes and gradients.
const renderSS = 2

// scaledConfig multiplies pixel-based metrics for supersampled compose.
// lineHeight stays a multiplier; only absolute px values scale.
func scaledConfig(cfg config.Config, ss int) config.Config {
	if ss <= 1 {
		return cfg
	}
	cfg.FontSize *= ss
	cfg.LetterSpacing *= ss
	return cfg
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
// The face honors cfg.FontFamily (first installed match, else Go Mono).
func cellMetrics(cfg config.Config) (cellW, cellH, ascent int) {
	px := float64(cfg.FontSize)
	if px <= 0 {
		px = 12
	}
	face := monoFaceFor(cfg.FontFamily, px)
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

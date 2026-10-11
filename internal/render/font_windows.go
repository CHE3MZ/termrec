//go:build windows

package render

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// registryFonts maps normalized Windows font display names
// ("eagernaturalist") to file paths, from HKCU then HKLM. Display names
// are the only reliable key: installed files are often cryptic stems
// (eager__.ttf) that no filename scan can match to a family name.
func registryFonts() map[string]string {
	out := map[string]string{}
	read := func(key registry.Key, name string) {
		k, err := registry.OpenKey(key, name, registry.QUERY_VALUE)
		if err != nil {
			return
		}
		defer k.Close()
		names, err := k.ReadValueNames(0)
		if err != nil {
			return
		}
		for _, vn := range names {
			val, _, err := k.GetStringValue(vn)
			if err != nil || val == "" {
				continue
			}
			base := vn
			if i := strings.LastIndex(base, " ("); i >= 0 {
				base = base[:i] // strip " (TrueType)"/" (OpenType)"
			}
			norm := ""
			for _, r := range strings.ToLower(base) {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
					norm += string(r)
				}
			}
			if norm == "" {
				continue
			}
			if _, ok := out[norm]; ok {
				continue // HKCU (read first) wins
			}
			out[norm] = resolveFontFile(strings.TrimSpace(val))
		}
	}
	const fontsKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`
	read(registry.CURRENT_USER, fontsKey)
	read(registry.LOCAL_MACHINE, fontsKey)
	return out
}

// resolveFontFile turns a registry value into a path: absolute values are
// used as-is, bare filenames are looked up in the per-user then system
// font folders.
func resolveFontFile(v string) string {
	if filepath.IsAbs(v) {
		return v
	}
	if l := os.Getenv("LOCALAPPDATA"); l != "" {
		if p := filepath.Join(l, "Microsoft", "Windows", "Fonts", v); fileExists(p) {
			return p
		}
	}
	if w := os.Getenv("WINDIR"); w != "" {
		if p := filepath.Join(w, "Fonts", v); fileExists(p) {
			return p
		}
	}
	if p := filepath.Join(`C:\Windows\Fonts`, v); fileExists(p) {
		return p
	}
	return v
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

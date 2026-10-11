//go:build !windows

package render

// registryFonts is Windows-only (font display names live in the registry);
// elsewhere the directory scan is the whole lookup.
func registryFonts() map[string]string { return nil }

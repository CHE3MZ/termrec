// Package fonts bundles the font files shipped with termrec.
//
// Files live next to the package so go:embed (which forbids parent paths)
// can reach them. Only the essentials are kept: a single weight per family.
package fonts

import _ "embed"

// GeistMonoMedium is Geist Mono Medium, the default terminal face.
// (Bold is double-struck from this face; no other weights are bundled.)
//
//go:embed geist-mono/GeistMono-Medium.ttf
var GeistMonoMedium []byte

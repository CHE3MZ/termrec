# Changelog

## Unreleased
- Theme: default palette is now Dark (Visual Studio); background stays
  transparent and cursor stays #c7c7c7 (not in the source scheme).
- Render: window traffic-light dots sit 3px below the CSS center (eyeballed).
- Render: shadow glow shifted 2px down (drop-shadow look on any background).
- Render: `-b/--background` flag to flatten onto an opaque page color.
- Render: progress lines to stderr; unique default output `render<epoch>.gif`.
- Fix: `repeat: -1` now plays once (was LoopCount 1 = twice).
- Terminal: double-width CJK/emoji runes occupy two cells (xterm-like wrap).
- Terminal: SGR blink (5/6, shown steady) and strikethrough (9/29, drawn).
- Text: per-rune font fallback chain (family, Go Mono, Go Regular, basicfont).
- Fixtures: shell-prompt records (`$ ` open/close, cursor starts line 1) and calmer delays.
- Fixtures: prompt `$` in white, not magenta.
- Render: idle cursor blink — pauses over 500ms split into alternating
  visible/hidden 500ms phases (capped at 8); hidden cursors and fast typing
  stay single-frame.
- Watermark kept (opt-in, off by default); skips and ignored style keys warn once to stderr.
- Docs: README documents flags and known differences from terminalizer.

## v0.1.0 — initial Go rewrite
- `init/config/record/play/render` (no share/generate, privacy-first).
- PTY record with <5ms coalesce, `.yml` compat with terminalizer.
- Timing parity (`frameDelay/maxIdleTime/speedFactor`), step filter.
- Pure-Go ANSI/SGR emulator + frameBox (null/window/floating/solid) + local watermark + GIF encode.
- DevOps: Taskfile, POSIX sh scripts, ci/render-matrix/release/fuzz workflows.
- Go >= 1.26, builds with 1.27, no CGO, tiny deps only.

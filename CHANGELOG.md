# Changelog

## Unreleased
- Theme: default palette is now Dark (Visual Studio); background stays
  transparent and cursor stays #c7c7c7 (not in the source scheme).
- Render: `--start-pause/--end-pause` hold edge frames (also `startPause`/
  `endPause` config keys, 5min cap); last frame holds its own delay.
- Render: dirty-rect frame encoding + identical-frame delay merge (small GIFs).
- Render: default output `render.gif`, `render_1.gif`, … on collision.
- Render: `-b` accepts a wallpaper image (PNG/JPEG, 60px padding); watermark
  images gain JPEG support too.
- Record: 350ms shell settle, F2 PNG snapshots, `-q/--quiet`; render `--quiet`.
- Record/render print frame counts, durations, and sizes.
- Render: window traffic-light dots sit 3px below the CSS center (eyeballed).
- Render: shadow glow shifted 2px down (drop-shadow look on any background).
- Render: `-b/--background` flag to flatten onto an opaque page color.
- Render: progress lines to stderr; collision-free default output names.
- Fix: `repeat: -1` now plays once (was LoopCount 1 = twice).
- Terminal: double-width CJK/emoji runes occupy two cells (xterm-like wrap).
- Terminal: SGR blink (5/6, shown steady) and strikethrough (9/29, drawn).
- Text: per-rune font fallback chain (family, Go Mono, Go Regular, basicfont).
- Fixtures: shell-prompt records (`$ ` open/close, cursor starts line 1) and calmer delays.
- Fixtures: prompt `$` in white, not magenta.
- Fixtures: new `micro` demo (full-screen editor session: syntax colors,
  256-color comment, insert edit, save message, quit) + `render-micro` task.
  Command, comment, and `echo` are typed per keystroke; repaints stay instant.
- Render: idle cursor blink — 700ms solid grace, then ~600ms off/on phases
  (capped at 8, pauses under ~1s stay single); hidden cursors never split.
- Watermark kept (opt-in, off by default); skips and ignored style keys warn once to stderr.
- Docs: README documents flags and known differences from terminalizer.

## v0.1.0 — initial Go rewrite
- `init/config/record/play/render` (no share/generate, privacy-first).
- PTY record with <5ms coalesce, `.yml` compat with terminalizer.
- Timing parity (`frameDelay/maxIdleTime/speedFactor`), step filter.
- Pure-Go ANSI/SGR emulator + frameBox (null/window/floating/solid) + local watermark + GIF encode.
- DevOps: Taskfile, POSIX sh scripts, ci/render-matrix/release/fuzz workflows.
- Go >= 1.26, builds with 1.27, no CGO, tiny deps only.

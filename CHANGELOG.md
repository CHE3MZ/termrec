# Changelog

## v0.1.0 — initial Go rewrite
- `init/config/record/play/render` (no share/generate, privacy-first).
- PTY record with <5ms coalesce, `.yml` compat with terminalizer.
- Timing parity (`frameDelay/maxIdleTime/speedFactor`), step filter.
- Pure-Go ANSI/SGR emulator + frameBox (null/window/floating/solid) + local watermark + GIF encode.
- DevOps: Taskfile, POSIX sh scripts, ci/render-matrix/release/fuzz workflows.
- Go >= 1.26, builds with 1.27, no CGO, tiny deps only.

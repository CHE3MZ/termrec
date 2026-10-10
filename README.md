# termrec — private terminal recorder (Go rewrite of terminalizer)

Record your terminal, play it back, render animated GIFs. No upload, no web player, no network. Single binary, tiny deps only.

Reference implementation lives in `ref/` (original Node.js + Electron terminalizer). `ref/` is docs-only and git-ignored.

## Install

```sh
go install github.com/CHE3MZ/termrec/cmd/termrec@latest
# or
task build
./dist/termrec --help
```

Requires Go >= 1.26 (CI builds with 1.27). No CGO, no system binaries.

## Usage

```sh
termrec init
termrec config
termrec record demo -c config.yml -d "powershell.exe"
termrec play demo -r -s 1.5
termrec render demo -o demo.gif -q 90 -s 2
```

Flags can come before or after the file (`render demo -o out.gif` works).

## Commands

- `init` — create `%APPDATA%/termrec` (Windows) or `~/.config/termrec` + default `config.yml`.
- `config` — write default `config.yml` into cwd.
- `record <file>` — PTY spawn, save `<file>.yml`. `-c/--config`, `-d/--command`.
- `play <file>` — replay to stdout. `-r/--real-timing`, `-s/--speed-factor`.
- `render <file>` — local GIF render. `-o/--output`, `-q/--quality 1-100`, `-s/--step`.

No `share`, no `generate`. Privacy-first: watermark `http(s)` URLs are ignored, only local PNGs.

## Config

Same keys as `ref/config.yml`: `command, cwd, env, cols, rows, repeat, quality, frameDelay, maxIdleTime, frameBox{type,title,style}, watermark{imagePath,style}, cursorStyle, fontFamily, fontSize, lineHeight, letterSpacing, theme{17 colors}`.

- `cols/rows: auto` → console size, fallback 80x24.
- `frameDelay: auto|<ms>`, `maxIdleTime: auto|<ms>` — same logic as terminalizer `play.js`.
- `frameBox.type: null|window|floating|solid`, `title: null|string`, `style: {boxShadow:none, margin:0px, backgroundColor, border}`.
- `quality 1-100` → GIF palette 256/128/64/32 (stdlib has no quality knob; mapping documented).
- `repeat: -1` play once → GIF LoopCount 1, `0` infinite → 0, `n` → n.
- `fontFamily` is a hint; rendering uses an embedded monospace fallback so CI/headless always works.
- `theme.background: transparent` is flattened to white for GIF (no alpha).

## Recording format

YAML `{config, records[{delay, content}]}` — can play/render old terminalizer `.yml` files.

## Dev (Actions-first)

Host only: `task build`, `task lint`, `task test-short`. Everything else via GitHub Actions.

```sh
task build
task lint
task test-short
task render-sample
busybox sh scripts/ops/go-bugcheck.sh
busybox sh scripts/ops/go-seccheck.sh
```

- `Taskfile.yml` — all tasks call `sh scripts/ops/*.sh`.
- `scripts/ops/*.sh` — POSIX `sh` only (`#!/bin/sh`, `set -eu`), tested with `busybox sh`.
- `.github/workflows/ci.yml` — lint+build+test+smoke render on push.
- `.github/workflows/render-matrix.yml` — golden renders on ubuntu/windows/macos.
- `.github/workflows/release.yml` — matrix binaries on tag `v*`.
- `.github/workflows/fuzz.yml` — weekly term-parser fuzz.

## Dependencies (tiny only)

- `gopkg.in/yaml.v3` — YAML compat (stdlib has none).
- `github.com/creack/pty` — PTY (stdlib `os/exec` cannot do PTY).
- `golang.org/x/image` — bitmap font raster (`basicfont`), no TTF/CGO.
- `golang.org/x/term` — raw mode + console size.
- `golang.org/x/sys` — indirect via above.

No Electron/Chromium/ffmpeg/ImageMagick/vips, no cobra, no uploader SDK.

## Layout

`cmd/termrec`, `internal/config|recording|timing|record|play|render|theme`, `testdata/*.yml`, `scripts/ops/*.sh`.

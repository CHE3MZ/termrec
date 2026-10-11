# termrec

Private terminal recorder. Record your terminal, play it back, render animated GIFs. No upload, no network.

## Install

```sh
go install github.com/CHE3MZ/termrec/cmd/termrec@latest
# or
task build
./dist/termrec --help
```

Requires Go >= 1.26.

## Use

```sh
termrec init
termrec config
termrec record demo -c config.yml -d "powershell.exe"   # F2 saves a PNG snapshot mid-session
termrec play demo -r -s 1.5
termrec render demo -o demo.gif -q 90 -s 2
termrec render demo -b white                             # opaque page background instead of transparency
termrec render demo -b wallpaper.png --start-pause 1s --end-pause 2s
```

`render` flags: `-o/--output` (default `render.gif`, `render_1.gif`, … on
collision; explicit `-o` overwrites), `-q/--quality 1-100`, `-s/--step`
(keep every Nth frame), `-b/--background` (CSS color, `transparent`, or an
image file used as wallpaper with 60px padding — PNG/JPEG, must cover the
page), `--start-pause/--end-pause` (hold edge frames, e.g. `2s`, `500ms`,
max 5min; also `startPause/endPause` config keys), `--quiet`.
Progress and warnings go to stderr; the final path goes to stdout.
Pauses longer than ~1s render the cursor blinking (700ms solid grace, then
~600ms off/on phases, capped), so long idles feel alive; fast typing and
fresh lines hold a steady cursor and never strobe.
The last frame holds its own delay (no wrap to the first record).

`config.yml` stays compatible with terminalizer recordings: old `.yml`
files `play` and `render` as-is.

## Known differences from terminalizer

Deliberate (privacy / headless design):

- No `share`, `generate`, upload, token, or network calls of any kind.
- No `record` share prompt (and no `-k/--skip-sharing` flag).
- GIFs are transparent by default; use `-b white` for the classic opaque look.
- Watermark: local PNG/JPEG files only. Remote URLs, `data:` URLs, and missing /
  undecodable files render without a watermark plus a once-only stderr
  warning. Only `width/height/left/right/top/bottom/opacity` style keys are
  honored (`position` is accepted as-is); anything else warns and is ignored.
- The drop shadow is a dark alpha glow shifted 2px down (reads on any
  background); ref uses a centered `#B3B3B3` glow tuned for its white page.

Approximations (custom engine, no browser):

- Fonts: `fontFamily` is a first-match-wins list scanned against installed
  system fonts — on Windows, registry display names resolve even cryptic
  file stems (`fontFamily: "Eager Naturalist, Monospace"` finds `eager__.ttf`);
  `fontFile` points at an explicit `.ttf/.otf` and wins
  over everything (use it for files that aren't installed). Anything missing
  warns once and falls back per-rune through embedded Go Mono, so rendering works headless
  with zero system fonts. Single weight only: bold is double-struck, italic
  is ignored, titlebar text is always the embedded sans, and proportional
  families will misalign columns (cells use one advance).
- Terminal emulation covers SGR (incl. 256-color/truecolor, blink shown
  steady, strikethrough), cursor moves,
  erase, scroll, alt-screen-as-clear, scroll-regions-as-full, and double-width
  CJK/emoji. No combining-mark rendering, no italic, no mouse /
  bracketed-paste / focus-event sequences.
- `frameBox.style` honors `background/backgroundColor, padding, margin,
  border, boxShadow: none, borderRadius`; other arbitrary CSS keys are ignored.
- GIF `quality` maps to palette size (256/128/64/32) + diffusion instead of
  the encoder's 1-100 scaler; flat theme colors stay bit-exact via anchors.
  Unchanged frames encode as dirty rectangles over the shared palette and
  pixel-identical frames merge (t-rec idle-dedup/`-layers Optimize` parity),
  so typing demos stay small.
- Frame delays clamp to a 20ms minimum (viewers ignore 0/1); `repeat: -1`
  plays once, `0` loops forever, `n` loops n times.

## Develop

```sh
task build          # build dist/termrec
task lint           # bugcheck + seccheck (same scripts CI runs)
task test           # go test ./... (unit tests in-package, integration in tests/)
task render-sample  # smoke render testdata/demo.yml
task render-micro   # complex editor demo testdata/micro.yml
task render-hyperfine  # typed benchmark demo testdata/hyperfine.yml
task render-eager  # system-font showcase testdata/eager.yml (Eager Naturalist)
task render-check   # all fixtures + step/quality variants + whitebg variants
```

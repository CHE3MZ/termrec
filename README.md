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
termrec record demo -c config.yml -d "powershell.exe"
termrec play demo -r -s 1.5
termrec render demo -o demo.gif -q 90 -s 2
termrec render demo -b white   # opaque page background instead of transparency
```

`render` flags: `-o/--output` (default `render<epoch>.gif`, never overwrites),
`-q/--quality 1-100`, `-s/--step` (keep every Nth frame), `-b/--background`
(any CSS color; empty/`transparent` keeps the transparent default).
Progress and warnings go to stderr; the final path goes to stdout.

`config.yml` stays compatible with terminalizer recordings: old `.yml`
files `play` and `render` as-is.

## Known differences from terminalizer

Deliberate (privacy / headless design):

- No `share`, `generate`, upload, token, or network calls of any kind.
- No `record` share prompt (and no `-k/--skip-sharing` flag).
- GIFs are transparent by default; use `-b white` for the classic opaque look.
- Watermark: local PNG files only. Remote URLs, `data:` URLs, and missing /
  undecodable files render without a watermark plus a once-only stderr
  warning. Only `width/height/left/right/top/bottom/opacity` style keys are
  honored (`position` is accepted as-is); anything else warns and is ignored.
- The drop shadow is a dark alpha glow shifted 2px down (reads on any
  background); ref uses a centered `#B3B3B3` glow tuned for its white page.

Approximations (custom engine, no browser):

- Fonts: embedded Go Mono / Go Regular with system-font lookup by filename,
  vs Chromium's full font stack. Layout parity, not pixel parity. Glyphs are
  resolved per rune across a fallback chain (family face, Go Mono, Go Regular,
  basicfont), so mixed-coverage text degrades gracefully instead of tofu —
  but color emoji still has no monochrome source and may box.
- Terminal emulation covers SGR (incl. 256-color/truecolor, blink shown
  steady, strikethrough), cursor moves,
  erase, scroll, alt-screen-as-clear, scroll-regions-as-full, and double-width
  CJK/emoji. No combining-mark rendering, no italic, no mouse /
  bracketed-paste / focus-event sequences.
- `frameBox.style` honors `background/backgroundColor, padding, margin,
  border, boxShadow: none, borderRadius`; other arbitrary CSS keys are ignored.
- GIF `quality` maps to palette size (256/128/64/32) + diffusion instead of
  the encoder's 1-100 scaler; flat theme colors stay bit-exact via anchors.
- Frame delays clamp to a 20ms minimum (viewers ignore 0/1); `repeat: -1`
  plays once, `0` loops forever, `n` loops n times.

## Develop

```sh
task build          # build dist/termrec
task lint           # bugcheck + seccheck (same scripts CI runs)
task test           # go test ./...
task render-sample  # smoke render testdata/demo.yml
task render-check   # all fixtures + step/quality variants + whitebg variants
```

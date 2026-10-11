// Package config loads, normalizes and resolves termrec YAML config.
// Port of ref/utility.js + ref/commands/init.js + ref/commands/config.js.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// AutoInt is an int that can also be the string "auto".
type AutoInt struct {
	IsAuto bool
	Value  int
}

func (a *AutoInt) UnmarshalYAML(value *yaml.Node) error {
	if value.Tag == "!!str" {
		var s string
		if err := value.Decode(&s); err != nil {
			return err
		}
		if strings.ToLower(strings.TrimSpace(s)) == "auto" {
			a.IsAuto = true
			return nil
		}
		return fmt.Errorf("invalid auto value %q, want int or \"auto\"", s)
	}
	var v int
	if err := value.Decode(&v); err != nil {
		return err
	}
	a.IsAuto = false
	a.Value = v
	return nil
}

func (a AutoInt) MarshalYAML() (interface{}, error) {
	if a.IsAuto {
		return "auto", nil
	}
	return a.Value, nil
}

// IntOrAuto is like AutoInt but also accepts float (YAML may decode 2000 as int anyway).
type AutoFloat struct {
	IsAuto bool
	Value  float64
}

func (a *AutoFloat) UnmarshalYAML(value *yaml.Node) error {
	if value.Tag == "!!str" {
		var s string
		if err := value.Decode(&s); err != nil {
			return err
		}
		if strings.ToLower(strings.TrimSpace(s)) == "auto" {
			a.IsAuto = true
			return nil
		}
		return fmt.Errorf("invalid auto value %q", s)
	}
	var v float64
	if err := value.Decode(&v); err != nil {
		return err
	}
	a.IsAuto = false
	a.Value = v
	return nil
}

func (a AutoFloat) MarshalYAML() (interface{}, error) {
	if a.IsAuto {
		return "auto", nil
	}
	if a.Value == float64(int(a.Value)) {
		return int(a.Value), nil
	}
	return a.Value, nil
}

// FrameBox mirrors ref/config.yml frameBox.
type FrameBox struct {
	Type  *string           `yaml:"type"`
	Title *string           `yaml:"title"`
	Style map[string]string `yaml:"style"`
}

// Watermark mirrors ref/config.yml watermark. Only local files are used (no URL fetch, privacy).
type Watermark struct {
	ImagePath *string           `yaml:"imagePath"`
	Style     map[string]string `yaml:"style"`
}

// Config mirrors ref/config.yml top-level keys.
type Config struct {
	Command       *string           `yaml:"command"`
	Cwd           *string           `yaml:"cwd"`
	Env           map[string]string `yaml:"env"`
	Cols          AutoInt           `yaml:"cols"`
	Rows          AutoInt           `yaml:"rows"`
	Repeat        int               `yaml:"repeat"`
	Quality       int               `yaml:"quality"`
	FrameDelay    AutoFloat         `yaml:"frameDelay"`
	MaxIdleTime   AutoFloat         `yaml:"maxIdleTime"`
	StartPause    int               `yaml:"startPause"`
	EndPause      int               `yaml:"endPause"`
	FrameBox      FrameBox          `yaml:"frameBox"`
	Watermark     Watermark         `yaml:"watermark"`
	CursorStyle   string            `yaml:"cursorStyle"`
	FontFamily    string            `yaml:"fontFamily"`
	FontFile      string            `yaml:"fontFile"`
	FontSize      int               `yaml:"fontSize"`
	LineHeight    float64           `yaml:"lineHeight"`
	LetterSpacing int               `yaml:"letterSpacing"`
	Theme         map[string]string `yaml:"theme"`
}

// DefaultYAML is embedded default config, byte-compatible keys with ref/config.yml.
const DefaultYAML = `# Specify a command to be executed
# like /bin/bash -l, ls, or any other commands
# the default is bash for Linux
# or powershell.exe for Windows
command: null

# Specify the current working directory path
# the default is the current working directory path
cwd: null

# Export additional ENV variables
env:
  recording: "true"

# Explicitly set the number of columns
# or use auto to take the current
# number of columns of your shell
cols: auto

# Explicitly set the number of rows
# or use auto to take the current
# number of rows of your shell
rows: auto

# Amount of times to repeat GIF
# If value is -1, play once
# If value is 0, loop indefinitely
# If value is a positive number, loop n times
repeat: 0

# Quality
# 1 - 100
quality: 100

# Delay between frames in ms
# If the value is auto use the actual recording delays
frameDelay: auto

# Maximum delay between frames in ms
# Ignored if the frameDelay is not set to auto
# Set to auto to prevent limiting the max idle time
maxIdleTime: 2000

# Hold the first/last GIF frame in ms: breathing room for demos
# (t-rec start-pause/end-pause parity; CLI flags override these)
startPause: 0
endPause: 0

# The surrounding frame box
# The type can be null, window, floating, or solid
# To hide the title use the value null
# Do not forget to add a backgroundColor style with a null as type
frameBox:
  type: floating
  title: Terminalizer
  style:
    border: 0px black solid

# Add a watermark image to the rendered gif
# Local absolute path only (no URL fetch, privacy-first)
watermark:
  imagePath: null
  style:
    position: absolute
    right: 15px
    bottom: 15px
    width: 100px
    opacity: "0.9"

# Cursor style can be one of
# block, underline, or bar
cursorStyle: block

# Font family
# First installed match wins (system font folders are scanned); anything
# missing falls back per-rune through embedded Geist Mono Medium, so
# rendering works headless with zero system fonts.
# Single weight only: bold is double-struck, italic is ignored, and the
# titlebar always uses the embedded sans.
fontFamily: "Geist Mono, Monaco, Lucida Console, Ubuntu Mono, Monospace"

# Optional absolute path to a .ttf/.otf file: wins over fontFamily above.
# Use this for files that aren't installed system-wide.
fontFile: null

# The size of the font
fontSize: 12

# The height of lines
lineHeight: 1

# The spacing between letters
letterSpacing: 0

# Theme: Dark (Visual Studio) palette; background stays transparent so GIFs
# render transparent by default (set an opaque color for a solid terminal).
theme:
  background: "transparent"
  foreground: "#bbbbbb"
  cursor: "#c7c7c7"
  black: "#000000"
  red: "#cd3131"
  green: "#0dbc79"
  yellow: "#e5e510"
  blue: "#2472c8"
  magenta: "#bc3fbc"
  cyan: "#11a8cd"
  white: "#e5e5e5"
  brightBlack: "#666666"
  brightRed: "#f14c4c"
  brightGreen: "#23d18b"
  brightYellow: "#f5f543"
  brightBlue: "#3b8eea"
  brightMagenta: "#d670d6"
  brightCyan: "#29b8db"
  brightWhite: "#e5e5e5"
`

// Load parses YAML bytes into Config.
func Load(data []byte) (Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// LoadFile loads a YAML config file.
func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Load(data)
}

// GlobalDir returns %APPDATA%/termrec on Windows, else ~/.config/termrec.
// Port of ref/utility.js getGlobalDirectory.
func GlobalDir() string {
	if runtime.GOOS == "windows" {
		if v := os.Getenv("APPDATA"); v != "" {
			return filepath.Join(v, "termrec")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".config", "termrec")
	}
	return filepath.Join(home, ".config", "termrec")
}

// DefaultConfig returns global config if present, else embedded default.
// Port of ref/utility.js getDefaultConfig.
func DefaultConfig() (Config, error) {
	globalPath := filepath.Join(GlobalDir(), "config.yml")
	if st, err := os.Stat(globalPath); err == nil && !st.IsDir() {
		return LoadFile(globalPath)
	}
	return Load([]byte(DefaultYAML))
}

// Normalize fills command/cwd/cols/rows defaults. Port of ref/commands/record.js normalizeConfig.
func Normalize(c *Config, fallbackCols, fallbackRows int) {
	if c.Command == nil || strings.TrimSpace(*c.Command) == "" {
		var cmd string
		if runtime.GOOS == "windows" {
			cmd = "powershell.exe"
		} else {
			cmd = "bash -l"
		}
		c.Command = &cmd
	}
	if c.Cwd == nil || strings.TrimSpace(*c.Cwd) == "" {
		if wd, err := os.Getwd(); err == nil {
			abs, _ := filepath.Abs(wd)
			c.Cwd = &abs
		} else {
			d := "."
			c.Cwd = &d
		}
	} else {
		if abs, err := filepath.Abs(*c.Cwd); err == nil {
			c.Cwd = &abs
		}
	}
	if c.Cols.IsAuto {
		if fallbackCols <= 0 {
			fallbackCols = 80
		}
		c.Cols = AutoInt{Value: fallbackCols}
	}
	if c.Rows.IsAuto {
		if fallbackRows <= 0 {
			fallbackRows = 24
		}
		c.Rows = AutoInt{Value: fallbackRows}
	}
	if c.Env == nil {
		c.Env = map[string]string{}
	}
}

// ResolveFilePath resolves path and appends ext if missing. Port of ref/utility.js resolveFilePath.
func ResolveFilePath(p, ext string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	if strings.ToLower(filepath.Ext(abs)) != "."+strings.ToLower(ext) {
		abs += "." + ext
	}
	return abs
}

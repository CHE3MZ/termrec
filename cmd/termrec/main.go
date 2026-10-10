// Command termrec — private terminal recorder/renderer (no web sharing).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/play"
	"github.com/CHE3MZ/termrec/internal/record"
	"github.com/CHE3MZ/termrec/internal/recording"
	"github.com/CHE3MZ/termrec/internal/render"
)

const version = "0.1.0"

func usage() {
	fmt.Fprintf(os.Stderr, `termrec %s — record your terminal, play back, render GIFs (private, no upload)

Usage:
  termrec <command> [options]

Commands:
  init                  Create global config directory
  config                Generate config.yml in current directory
  record <file>         Record terminal to <file>.yml
  play <file>           Play a recording on your terminal
  render <file>         Render a recording to an animated GIF

Options:
  -h, --help            Show help
  -v, --version         Show version

Run 'termrec <command> --help' for command help.
Hint: use --help to get help about the usage
`, version)
}

func hintAndExit(err error) {
	fmt.Fprintf(os.Stderr, "Error: \n  %v\n\n", err)
	fmt.Fprintln(os.Stderr, "Hint:\n  Use the --help option to get help about the usage")
	os.Exit(1)
}

// splitFlagValue handles --flag=value form.
func splitFlagValue(a string) (string, string, bool) {
	for i := 0; i < len(a); i++ {
		if a[i] == '=' {
			return a[:i], a[i+1:], true
		}
	}
	return a, "", false
}

func hasHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			return true
		}
	}
	return false
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	switch cmd {
	case "-h", "--help", "help":
		usage()
		return
	case "-v", "--version", "version":
		fmt.Println(version)
		return
	case "init":
		cmdInit(os.Args[2:])
	case "config":
		cmdConfig(os.Args[2:])
	case "record":
		cmdRecord(os.Args[2:])
	case "play":
		cmdPlay(os.Args[2:])
	case "render":
		cmdRender(os.Args[2:])
	default:
		hintAndExit(fmt.Errorf("unknown command %q", cmd))
	}
}

func cmdInit(args []string) {
	if hasHelp(args) {
		fmt.Fprintln(os.Stderr, "Usage: termrec init")
		return
	}
	dir := config.GlobalDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		hintAndExit(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(config.DefaultYAML), 0644); err != nil {
		hintAndExit(err)
	}
	fmt.Println("The global config directory is created at")
	fmt.Println(dir)
}

func cmdConfig(args []string) {
	if hasHelp(args) {
		fmt.Fprintln(os.Stderr, "Usage: termrec config")
		return
	}
	if _, err := os.Stat("config.yml"); err == nil {
		fmt.Println("config.yml already exists, overwriting")
	}
	if err := os.WriteFile("config.yml", []byte(config.DefaultYAML), 0644); err != nil {
		hintAndExit(err)
	}
	fmt.Println("Successfully Saved")
	fmt.Println("The config file is saved into the file:")
	fmt.Println("config.yml")
}

func cmdRecord(args []string) {
	if hasHelp(args) {
		fmt.Fprintln(os.Stderr, "Usage: termrec record <recordingFile> [-c config.yml] [-d command]")
		fmt.Fprintln(os.Stderr, "  -c, --config   Overwrite the default configurations")
		fmt.Fprintln(os.Stderr, "  -d, --command  The command to be executed")
		return
	}
	var cfgFile, command string
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, val, hasVal := splitFlagValue(a)
		switch name {
		case "-c", "--config":
			if !hasVal {
				i++
				if i >= len(args) {
					hintAndExit(fmt.Errorf("flag %s needs a value", name))
				}
				val = args[i]
			}
			cfgFile = val
		case "-d", "--command":
			if !hasVal {
				i++
				if i >= len(args) {
					hintAndExit(fmt.Errorf("flag %s needs a value", name))
				}
				val = args[i]
			}
			command = val
		default:
			if len(a) > 0 && a[0] == '-' {
				hintAndExit(fmt.Errorf("unknown flag %q for record", a))
			}
			positional = append(positional, a)
		}
	}
	if len(positional) < 1 {
		hintAndExit(fmt.Errorf("recordingFile is missing\nUsage: termrec record <recordingFile>"))
	}
	var cfg config.Config
	var err error
	if cfgFile != "" {
		cfg, err = config.LoadFile(cfgFile)
		if err != nil {
			hintAndExit(err)
		}
	} else {
		cfg, err = config.DefaultConfig()
		if err != nil {
			hintAndExit(err)
		}
	}
	if _, err := record.Run(cfg, positional[0], command); err != nil {
		hintAndExit(err)
	}
}

func cmdPlay(args []string) {
	if hasHelp(args) {
		fmt.Fprintln(os.Stderr, "Usage: termrec play <recordingFile> [-r] [-s factor]")
		fmt.Fprintln(os.Stderr, "  -r, --real-timing    Use actual recorded delays")
		fmt.Fprintln(os.Stderr, "  -s, --speed-factor   Speed factor (default 1)")
		return
	}
	var realTiming bool
	speed := 1.0
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, val, hasVal := splitFlagValue(a)
		switch name {
		case "-r", "--real-timing":
			realTiming = true
		case "-s", "--speed-factor":
			if !hasVal {
				i++
				if i >= len(args) {
					hintAndExit(fmt.Errorf("flag %s needs a value", name))
				}
				val = args[i]
			}
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				hintAndExit(fmt.Errorf("bad speed factor %q", val))
			}
			speed = f
		default:
			if len(a) > 0 && a[0] == '-' {
				hintAndExit(fmt.Errorf("unknown flag %q for play", a))
			}
			positional = append(positional, a)
		}
	}
	if len(positional) < 1 {
		hintAndExit(fmt.Errorf("recordingFile is missing\nUsage: termrec play <recordingFile>"))
	}
	rec, _, err := recording.LoadFile(positional[0])
	if err != nil {
		hintAndExit(err)
	}
	play.Run(rec, realTiming, speed)
}

func cmdRender(args []string) {
	if hasHelp(args) {
		fmt.Fprintln(os.Stderr, "Usage: termrec render <recordingFile> [-o out.gif] [-q quality] [-s step]")
		fmt.Fprintln(os.Stderr, "  -o, --output   Output file")
		fmt.Fprintln(os.Stderr, "  -q, --quality  Quality 1-100")
		fmt.Fprintln(os.Stderr, "  -s, --step     Step to reduce frames (default 1)")
		return
	}
	var out string
	quality := 0
	step := 1
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, val, hasVal := splitFlagValue(a)
		switch name {
		case "-o", "--output":
			if !hasVal {
				i++
				if i >= len(args) {
					hintAndExit(fmt.Errorf("flag %s needs a value", name))
				}
				val = args[i]
			}
			out = val
		case "-q", "--quality":
			if !hasVal {
				i++
				if i >= len(args) {
					hintAndExit(fmt.Errorf("flag %s needs a value", name))
				}
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil {
				hintAndExit(fmt.Errorf("bad quality %q", val))
			}
			quality = n
		case "-s", "--step":
			if !hasVal {
				i++
				if i >= len(args) {
					hintAndExit(fmt.Errorf("flag %s needs a value", name))
				}
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil {
				hintAndExit(fmt.Errorf("bad step %q", val))
			}
			step = n
		default:
			if len(a) > 0 && a[0] == '-' {
				hintAndExit(fmt.Errorf("unknown flag %q for render", a))
			}
			positional = append(positional, a)
		}
	}
	if len(positional) < 1 {
		hintAndExit(fmt.Errorf("recordingFile is missing\nUsage: termrec render <recordingFile>"))
	}
	rec, _, err := recording.LoadFile(positional[0])
	if err != nil {
		hintAndExit(err)
	}
	outPath := out
	if outPath != "" {
		outPath = config.ResolveFilePath(outPath, "gif")
	} else {
		outPath = config.ResolveFilePath("render", "gif")
	}
	final, err := render.Render(rec, outPath, quality, step)
	if err != nil {
		hintAndExit(err)
	}
	fmt.Println("\nSuccessfully Rendered")
	fmt.Println("The animated GIF image is saved into the file:")
	fmt.Println(final)
}

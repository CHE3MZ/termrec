// Package record spawns a shell in a PTY and saves a .yml recording.
// Port of ref/commands/record.js (minus share prompt).
package record

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/CHE3MZ/termrec/internal/recording"
	"github.com/CHE3MZ/termrec/internal/render"
	"golang.org/x/term"
)

// splitCommand is a minimal shell-like splitter (handles single/double quotes).
func splitCommand(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	esc := false
	for _, r := range s {
		switch {
		case esc:
			cur.WriteRune(r)
			esc = false
		case r == '\\' && quote == 0:
			esc = true
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote = r
		case quote == 0 && (r == ' ' || r == '\t'):
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// settleDelay lets the shell print its first prompt before capture starts
// (t-rec waits 350ms for shell bootstrap the same way). Input during the
// settle is still forwarded, only the capture clock starts after it.
const settleDelay = 350 * time.Millisecond

// f2Seqs are the raw byte sequences treated as the F2 snapshot hotkey
// (xterm SS3 F2, plus the CSI numeric variant some terminals send).
var f2Seqs = [][]byte{
	{0x1b, 'O', 'Q'},
	{0x1b, '[', '1', '2', '~'},
}

// containsHotkey reports whether b holds an F2 snapshot sequence.
func containsHotkey(b []byte) bool {
	for _, s := range f2Seqs {
		if bytes.Contains(b, s) {
			return true
		}
	}
	return false
}

// Run records a session and writes outPath (.yml appended if missing).
// quiet suppresses informational output (errors still print). Pressing F2
// during the session saves a PNG snapshot (<recording>-shot-N.png) of the
// current terminal grid next to the recording.
func Run(cfg config.Config, outPath, commandOverride string, quiet bool) (string, error) {
	if commandOverride != "" {
		cfg.Command = &commandOverride
	}
	// console size fallback
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	fbCols, fbRows := 80, 24
	if err == nil {
		fbCols, fbRows = w, h
	}
	config.Normalize(&cfg, fbCols, fbRows)
	resolved := config.ResolveFilePath(outPath, "yml")

	args := splitCommand(strings.TrimSpace(*cfg.Command))
	if len(args) == 0 {
		return "", fmt.Errorf("empty command")
	}
	// Build env
	env := os.Environ()
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}

	if !quiet {
		fmt.Println("The recording session is started")
		fmt.Println("Press CTRL+D (or type `exit`) to exit and save the recording")
		fmt.Println("Press F2 to save a PNG snapshot next to the recording")
	}
	started := time.Now()

	// Start PTY via creack/pty (Unix; on Windows uses ConPTY when available).
	ptmx, proc, err := startPty(args[0], args[1:], cfg, env)
	if err != nil {
		return "", err
	}
	defer ptmx.Close()

	// Raw mode for stdin so keys pass through.
	oldState, rawErr := term.MakeRaw(int(os.Stdin.Fd()))
	if rawErr == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}
	// Shadow terminal for F2 snapshots: fed with the same PTY output the
	// recording captures, so the snapshot matches what render would draw.
	shadowCols, shadowRows := cfg.Cols.Value, cfg.Rows.Value
	if shadowCols <= 0 {
		shadowCols = 80
	}
	if shadowRows <= 0 {
		shadowRows = 24
	}
	shadow := render.NewTerminal(shadowCols, shadowRows)
	shotBase := strings.TrimSuffix(resolved, ".yml")
	shotN := 0
	dumpShot := func() {
		shotN++
		img := render.DrawFrame(shadow.Snapshot(), shadow.CurX, shadow.CurY, shadow.CursorVisible, cfg)
		path := fmt.Sprintf("%s-shot-%d.png", shotBase, shotN)
		f, ferr := os.Create(path)
		if ferr != nil {
			fmt.Fprintf(os.Stderr, "termrec: warn: snapshot not saved (%v)\n", ferr)
			return
		}
		defer f.Close()
		if ferr := png.Encode(f, img); ferr != nil {
			fmt.Fprintf(os.Stderr, "termrec: warn: snapshot not saved (%v)\n", ferr)
			return
		}
		fmt.Fprintf(os.Stderr, "termrec: snapshot saved: %s\n", path)
	}
	// Bridge stdin -> pty, watching for F2 (forwarded untouched: the app
	// still receives it, the snapshot is a side effect).
	go func() {
		buf := make([]byte, 8192)
		var tail []byte
		for {
			n, rerr := os.Stdin.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				// Sequences may straddle reads: scan tail+chunk, keep 4.
				win := chunk
				if len(tail) > 0 {
					win = append(append([]byte{}, tail...), chunk...)
				}
				if containsHotkey(win) {
					dumpShot()
				}
				if len(win) > 4 {
					tail = append(tail[:0], win[len(win)-4:]...)
				} else {
					tail = append(tail[:0], win...)
				}
				_, _ = ptmx.Write(chunk)
			}
			if rerr != nil {
				return
			}
		}
	}()

	// Settle: let the shell print its first prompt before the capture
	// clock starts, so the opening frame isn't a blank race.
	time.Sleep(settleDelay)

	records := []recording.Record{}
	last := time.Now()
	// Seed first timestamp like ref (lastRecordTimestamp starts at spawn)
	buf := make([]byte, 8192)
	coalesce := func(content []byte, dur time.Duration) {
		ms := dur.Milliseconds()
		if ms < 0 {
			ms = 0
		}
		if ms < 5 && len(records) > 0 {
			records[len(records)-1].Content += string(content)
			return
		}
		records = append(records, recording.Record{Delay: ms, Content: string(content)})
	}
	// Initial zero-delay anchor? ref pushes on first data with duration since spawn.
	for {
		n, rerr := ptmx.Read(buf)
		now := time.Now()
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			_, _ = os.Stdout.Write(chunk)
			shadow.Feed(string(chunk))
			coalesce(chunk, now.Sub(last))
			last = now
		}
		if rerr != nil {
			break
		}
	}
	_, _ = proc.Wait()
	// Restore terminal (deferred) + pause stdin like ref
	if rawErr == nil {
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
	}

	if len(records) == 0 {
		records = append(records, recording.Record{Delay: 100, Content: ""})
	}
	if err := recording.Save(resolved, recording.File{Config: cfg, Records: records}); err != nil {
		return "", err
	}
	if !quiet {
		fmt.Println("Successfully Recorded")
		fmt.Println("The recording data is saved into the file:")
	}
	fmt.Println(resolved)
	if !quiet {
		fmt.Printf("Frames: %d, time: %.1fs", len(records), time.Since(started).Seconds())
		if shotN > 0 {
			fmt.Printf(", snapshots: %d", shotN)
		}
		fmt.Println()
		fmt.Println("You can edit the file and even change the configurations.")
	}
	return resolved, nil
}

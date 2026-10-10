package record

import (
	"os"
	"os/exec"

	"github.com/CHE3MZ/termrec/internal/config"
	"github.com/creack/pty"
)

// startPty spawns command in a PTY sized from config.
func startPty(name string, args []string, cfg config.Config, env []string) (*os.File, *os.Process, error) {
	cmd := exec.Command(name, args...)
	if cfg.Cwd != nil && *cfg.Cwd != "" {
		cmd.Dir = *cfg.Cwd
	}
	cmd.Env = env
	ws := &pty.Winsize{Cols: uint16(cfg.Cols.Value), Rows: uint16(cfg.Rows.Value)}
	if ws.Cols == 0 {
		ws.Cols = 80
	}
	if ws.Rows == 0 {
		ws.Rows = 24
	}
	ptmx, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, nil, err
	}
	return ptmx, cmd.Process, nil
}

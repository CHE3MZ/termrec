// Package recording reads/writes .yml recording files: {config, records[{delay, content}]}.
// Compatible with ref/commands/record.js output.
package recording

import (
	"os"

	"github.com/CHE3MZ/termrec/internal/config"
	"gopkg.in/yaml.v3"
)

// Record is one captured PTY chunk.
type Record struct {
	Delay   int64  `yaml:"delay"`
	Content string `yaml:"content"`
}

// File is the on-disk recording.
type File struct {
	Config  config.Config `yaml:"config"`
	Records []Record      `yaml:"records"`
}

// LoadFile loads a recording (.yml appended if missing).
func LoadFile(path string) (File, string, error) {
	resolved := config.ResolveFilePath(path, "yml")
	data, err := os.ReadFile(resolved)
	if err != nil {
		return File{}, "", err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return File{}, "", err
	}
	return f, resolved, nil
}

// Save writes a recording with a header comment.
func Save(path string, f File) error {
	out, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	header := "# termrec recording (compatible with terminalizer .yml)\n# Feel free to edit config and records.\n"
	return os.WriteFile(path, append([]byte(header), out...), 0644)
}

package recording

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CHE3MZ/termrec/internal/config"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg, err := config.Load([]byte(config.DefaultYAML))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Cols.IsAuto = false
	cfg.Cols.Value = 40
	cfg.Rows.IsAuto = false
	cfg.Rows.Value = 10
	in := File{Config: cfg, Records: []Record{
		{Delay: 120, Content: "hello\r\n"},
		{Delay: 340, Content: "\x1b[31mred\x1b[0m\r\n"},
	}}
	path := filepath.Join(t.TempDir(), "roundtrip.yml")
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	// LoadFile appends .yml if missing; path already has it
	got, resolved, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == "" {
		t.Fatalf("empty resolved path")
	}
	if len(got.Records) != 2 || got.Records[0].Delay != 120 || got.Records[1].Content != "\x1b[31mred\x1b[0m\r\n" {
		t.Fatalf("roundtrip mismatch: %+v", got.Records)
	}
	if got.Config.Quality != in.Config.Quality {
		t.Fatalf("config mismatch")
	}
}

func TestLoadMissingExt(t *testing.T) {
	cfg, _ := config.Load([]byte(config.DefaultYAML))
	path := filepath.Join(t.TempDir(), "noext")
	if err := Save(path+".yml", File{Config: cfg, Records: []Record{{Delay: 5, Content: "x"}}}); err != nil {
		t.Fatal(err)
	}
	// load without extension
	if _, _, err := LoadFile(path); err != nil {
		t.Fatalf("load without ext: %v", err)
	}
	_ = os.Getenv("TMPDIR")
}

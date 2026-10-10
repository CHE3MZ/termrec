package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefault(t *testing.T) {
	c, err := Load([]byte(DefaultYAML))
	if err != nil {
		t.Fatal(err)
	}
	if c.Quality != 100 || c.Repeat != 0 {
		t.Fatalf("bad defaults %+v", c)
	}
	if !c.Cols.IsAuto || !c.Rows.IsAuto {
		t.Fatalf("cols/rows should be auto")
	}
	if !c.FrameDelay.IsAuto {
		t.Fatalf("frameDelay should be auto")
	}
	if c.MaxIdleTime.IsAuto || int(c.MaxIdleTime.Value) != 2000 {
		t.Fatalf("maxIdleTime should be 2000")
	}
	if c.FrameBox.Type == nil || *c.FrameBox.Type != "floating" {
		t.Fatalf("frameBox.type")
	}
}

func TestResolveFilePath(t *testing.T) {
	got := ResolveFilePath("demo", "yml")
	want, _ := filepath.Abs("demo.yml")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = ResolveFilePath("demo.yml", "yml")
	if got != want {
		t.Fatalf("ext dup: %q", got)
	}
}

func TestNormalize(t *testing.T) {
	c, _ := Load([]byte(DefaultYAML))
	Normalize(&c, 80, 24)
	if c.Command == nil || *c.Command == "" {
		t.Fatalf("command default missing")
	}
	if c.Cwd == nil || *c.Cwd == "" {
		t.Fatalf("cwd default missing")
	}
	if c.Cols.Value != 80 || c.Rows.Value != 24 {
		t.Fatalf("cols/rows normalize: %+v %+v", c.Cols, c.Rows)
	}
}

func TestGlobalDir(t *testing.T) {
	d := GlobalDir()
	if d == "" {
		t.Fatalf("empty global dir")
	}
	_ = os.Getenv("APPDATA")
}

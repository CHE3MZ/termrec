package render

import (
	"image/gif"
	"os"
	"testing"

	"github.com/CHE3MZ/termrec/internal/config"
)

func testConfig(frameType string, title *string) config.Config {
	c, _ := config.Load([]byte(config.DefaultYAML))
	c.FrameBox.Type = &frameType
	c.FrameBox.Title = title
	c.Cols.Value = 20
	c.Rows.Value = 5
	c.Cols.IsAuto = false
	c.Rows.IsAuto = false
	return c
}

func decodeGIF(t *testing.T, path string) *gif.GIF {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return g
}

func diff(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

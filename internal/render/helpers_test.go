package render

import "github.com/CHE3MZ/termrec/internal/config"

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

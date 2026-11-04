package imaging

import (
	"bytes"
	"image"
	"os"
	"testing"
)

func TestFork_WebPDecoderRegistered(t *testing.T) {
	data, err := os.ReadFile("testdata/lossy.webp")
	if err != nil {
		t.Fatal("缺 testdata/lossy.webp")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("webp decoder not registered: %v", err)
	}
	t.Logf("webp decoder registered: %s %dx%d", format, cfg.Width, cfg.Height)
}

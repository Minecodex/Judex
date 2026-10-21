package identity

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestPortraitNormalization(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 400, 200))
	img.Set(200, 100, color.NRGBA{R: 200, A: 255})
	var buf bytes.Buffer
	png.Encode(&buf, img)
	out, e := NormalizeAvatar("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(out))
	if e != nil || cfg.Width != 256 || cfg.Height != 256 || format != "png" {
		t.Fatal(cfg, format, e)
	}
	for _, v := range []string{"https://outside/avatar.png", "data:image/svg+xml;base64,PHN2Zz4=", "data:image/png;base64,YWJj"} {
		if _, e = NormalizeAvatar(v); e == nil {
			t.Fatal("accepted invalid raster")
		}
	}
	if out, e = NormalizeAvatar(""); e != nil || out != nil {
		t.Fatal("remove portrait")
	}
}

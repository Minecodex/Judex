package material

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	_ "golang.org/x/image/webp"
)

// Validation is read-only and belongs to the existing fenced preview job. The
// parser has no persistent configuration, network access or writable original.
func validatePreviewPDF(ctx context.Context, raw []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conf := model.NewStatelessConfiguration()
	conf.Offline = true
	conf.Optimize = false
	conf.ValidationMode = model.ValidationRelaxed
	conf.DecodeAllStreams = true
	conf.Limits.MaxInputBytes = int64(len(raw))
	conf.Limits.MaxObjectBytes = 8 << 20
	conf.Limits.MaxStreamBytes = 32 << 20
	conf.Limits.MaxDecodeBytes = 64 << 20
	conf.Limits.MaxImagePixels = 16_000_000
	conf.Limits.MaxImageBytes = 64 << 20
	conf.Limits.MaxObjectCount = 100_000
	return api.Validate(ctx, bytes.NewReader(raw), conf, nil)
}

func validatePreviewImage(raw []byte) error {
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16_000_000 {
		return fmt.Errorf("image preview pixel limit exceeded")
	}
	_, _, err = image.Decode(bytes.NewReader(raw))
	return err
}

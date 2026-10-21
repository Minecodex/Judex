package identity

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	"image/png"
	"strings"

	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"golang.org/x/image/draw"
)

// NormalizeAvatar accepts only bounded raster input and strips all metadata.
// The resulting small PNG is account data, independent of conversation files.
func NormalizeAvatar(data string) ([]byte, error) {
	if data == "" {
		return nil, nil
	}
	bad := func() ([]byte, error) {
		return nil, apierrors.Fields("avatarData", "use a PNG or JPEG up to 512 KB and 4096 × 4096 pixels")
	}
	_, encoded, ok := strings.Cut(data, ",")
	if !ok || !(strings.HasPrefix(data, "data:image/png;base64,") || strings.HasPrefix(data, "data:image/jpeg;base64,")) || len(encoded) > 700000 {
		return bad()
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) > 512<<10 {
		return bad()
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 {
		return bad()
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return bad()
	}
	bounds := img.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	start := image.Pt(bounds.Min.X+(bounds.Dx()-side)/2, bounds.Min.Y+(bounds.Dy()-side)/2)
	crop := image.Rectangle{Min: start, Max: start.Add(image.Pt(side, side))}
	dst := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, crop, draw.Src, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Profiles are visible to signed-in platform users, like the user directory.
// Transport authentication is required; no public object-store URL is issued.
func (s *Service) Avatar(ctx context.Context, user uuid.UUID) ([]byte, error) {
	var out []byte
	if err := s.pool.QueryRow(ctx, `SELECT avatar_png FROM users WHERE id=$1 AND status='active' AND avatar_png IS NOT NULL`, user).Scan(&out); err != nil {
		return nil, apierrors.New(apierrors.NotFound, "avatar unavailable")
	}
	return out, nil
}

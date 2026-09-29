package identity

import (
	"context"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/keys"
	"time"
)

type DeviceReview struct {
	DeviceName   string      `json:"deviceName"`
	Scopes       []string    `json:"scopes"`
	ProjectScope []uuid.UUID `json:"projectScope"`
	State        string      `json:"state"`
	ExpiresAt    time.Time   `json:"expiresAt"`
}

func (s *Service) DeviceReview(ctx context.Context, code string) (DeviceReview, error) {
	var r DeviceReview
	err := s.pool.QueryRow(ctx, `SELECT device_name,requested_scopes,requested_project_scope,status,expires_at FROM device_authorizations WHERE user_code_hash=$1`, keys.Hash(code)).Scan(&r.DeviceName, &r.Scopes, &r.ProjectScope, &r.State, &r.ExpiresAt)
	if err != nil {
		return r, apierrors.New(apierrors.NotFound, "device code not found")
	}
	if s.now().After(r.ExpiresAt) {
		r.State = "expired"
	}
	return r, nil
}

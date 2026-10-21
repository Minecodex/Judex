package collab

import (
	"context"
	"github.com/google/uuid"
)

// CleanupUploads deletes only staging bytes. Registered versions and their
// historical/shared references never participate in garbage collection.
func (s *Service) CleanupUploads(ctx context.Context) error {
	if s.Objects == nil {
		return nil
	}
	rows, e := s.Pool.Query(ctx, `SELECT p.upload_id,p.part,p.object_key FROM upload_parts p JOIN uploads u ON u.id=p.upload_id WHERE u.state IN('completed','cancelled') OR u.expires_at<now() ORDER BY u.created_at LIMIT 100`)
	if e != nil {
		return e
	}
	type part struct {
		upload uuid.UUID
		n      int
		key    string
	}
	parts := []part{}
	for rows.Next() {
		var item part
		if e = rows.Scan(&item.upload, &item.n, &item.key); e != nil {
			rows.Close()
			return e
		}
		parts = append(parts, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, item := range parts {
		if e = s.Objects.Delete(ctx, item.key); e != nil {
			return e
		}
		if _, e = s.Pool.Exec(ctx, `DELETE FROM upload_parts WHERE upload_id=$1 AND part=$2 AND object_key=$3`, item.upload, item.n, item.key); e != nil {
			return e
		}
	}
	return nil
}

package httptransport

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/platform/auth"
)

func liveAuthority(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, p *auth.Principal) bool {
	if p == nil {
		return false
	}
	var valid bool
	query := `SELECT EXISTS(SELECT 1 FROM users u WHERE u.id=$1 AND u.status='active' AND u.auth_version=$2
 AND (($3='web' AND EXISTS(SELECT 1 FROM user_sessions s WHERE s.id=$4 AND s.user_id=u.id AND s.revoked_at IS NULL AND s.expires_at>now()))
 OR ($3='cli' AND EXISTS(SELECT 1 FROM client_grants g WHERE g.id=$5 AND g.user_id=u.id AND g.revoked_at IS NULL AND g.expires_at>now() AND EXISTS(SELECT 1 FROM client_tokens t WHERE t.id=$6 AND t.grant_id=g.id AND t.access_expires_at>now() AND t.revoked_at IS NULL AND t.rotated_at IS NULL)))))`
	return q.QueryRow(ctx, query, p.UserID, p.AuthVersion, string(p.Kind), p.SessionID, p.GrantID, p.AccessTokenID).Scan(&valid) == nil && valid
}

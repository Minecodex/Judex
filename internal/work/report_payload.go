package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"net/url"
	"sort"
)

func reportHash(in ReportInput) string {
	materials := append([]string{}, in.MaterialVersionIDs...)
	sort.Strings(materials)
	raw, _ := json.Marshal(map[string]any{"kind": in.Kind, "text": in.Text, "materials": materials, "codeRefs": in.CodeRefs, "environmentRefs": in.EnvironmentRefs})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func validateCodeRefs(ctx context.Context, tx pgx.Tx, project uuid.UUID, refs []map[string]any) error {
	for _, ref := range refs {
		allowed := map[string]bool{"repositoryId": true, "branch": true, "commit": true, "pullUrl": true, "note": true, "verification": true}
		for key := range ref {
			if !allowed[key] {
				return apierrors.Fields("codeRefs."+key, "unsupported")
			}
		}
		raw, _ := ref["repositoryId"].(string)
		id, err := uuid.Parse(raw)
		if err != nil {
			return apierrors.Fields("codeRefs.repositoryId", "uuid")
		}
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM repository_links WHERE id=$1 AND project_id=$2 AND archived_at IS NULL)`, id, project).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return apierrors.New(apierrors.InvalidReference, "repository outside project")
		}
		if sha, ok := ref["commit"].(string); ok && sha != "" {
			if len(sha) != 40 && len(sha) != 64 {
				return apierrors.Fields("codeRefs.commit", "full SHA required")
			}
			if _, err = hex.DecodeString(sha); err != nil {
				return apierrors.Fields("codeRefs.commit", "hex")
			}
		}
		if raw, ok := ref["pullUrl"].(string); ok && raw != "" {
			u, err := url.Parse(raw)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Host == "" {
				return apierrors.Fields("codeRefs.pullUrl", "http URL without credentials required")
			}
		}
		ref["verification"] = "reported"
	}
	return nil
}

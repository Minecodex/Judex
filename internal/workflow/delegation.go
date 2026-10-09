package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"sort"
)

func delegationMap(nodes []Node) []byte {
	m := map[string][]string{}
	for _, n := range nodes {
		if len(n.DelegationUserIDs) > 0 {
			ids := append([]string{}, n.DelegationUserIDs...)
			sort.Strings(ids)
			m[n.ID] = ids
		}
	}
	raw, _ := json.Marshal(m)
	return raw
}

// Only the project owner can grant or revoke node delegation by publishing.
func validateDelegations(ctx context.Context, tx pgx.Tx, project, definition uuid.UUID, role string, nodesRaw []byte) error {
	var nodes []Node
	if err := json.Unmarshal(nodesRaw, &nodes); err != nil {
		return err
	}
	for _, node := range nodes {
		for _, user := range node.DelegationUserIDs {
			var active bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_members m JOIN users u ON u.id=m.user_id WHERE m.project_id=$1 AND m.user_id=$2 AND m.state='active' AND u.status='active')`, project, user).Scan(&active); err != nil {
				return err
			}
			if !active {
				return apierrors.New(apierrors.InvalidReference, "delegation requires an active project member")
			}
		}
	}
	if role == "owner" {
		return nil
	}
	var oldRaw []byte
	if err := tx.QueryRow(ctx, `SELECT COALESCE(v.nodes_json,'[]'::jsonb) FROM workflow_definitions d LEFT JOIN workflow_versions v ON v.id=d.published_version_id WHERE d.id=$1 AND d.project_id=$2`, definition, project).Scan(&oldRaw); err != nil {
		return err
	}
	var old []Node
	if err := json.Unmarshal(oldRaw, &old); err != nil {
		return err
	}
	if !bytes.Equal(delegationMap(nodes), delegationMap(old)) {
		return apierrors.New(apierrors.Forbidden, "only project owner can change node delegation grants")
	}
	return nil
}

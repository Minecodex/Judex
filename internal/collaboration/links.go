package collaboration

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// ChangeTopicLinks is the common mutation for existing discussions, including
// approved arrangements and human-confirmed suggestions. The caller authorizes
// the command and locks the active project in the same transaction.
func ChangeTopicLinks(ctx context.Context, tx pgx.Tx, project, topic uuid.UUID, expected *int64, links []Link, replace bool, now time.Time) error {
	var version int64
	if err := tx.QueryRow(ctx, `SELECT links_version FROM topics WHERE project_id=$1 AND id=$2 FOR UPDATE`, project, topic).Scan(&version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apierrors.New(apierrors.NotFound, "topic not found")
		}
		return err
	}
	if expected != nil && *expected != version {
		return apierrors.New(apierrors.VersionConflict, "discussion associations changed").WithDetails(map[string]any{"linksVersion": version})
	}
	if err := ValidateLinks(ctx, tx, project, links); err != nil {
		return err
	}
	if replace {
		rows, err := tx.Query(ctx, `SELECT 'plan',id FROM plans WHERE project_id=$1 AND main_topic_id=$2 UNION ALL SELECT 'task',id FROM tasks WHERE project_id=$1 AND main_topic_id=$2`, project, topic)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var kind string
			var id uuid.UUID
			if err = rows.Scan(&kind, &id); err != nil {
				return err
			}
			found := false
			for _, link := range links {
				found = found || link.ObjectType == kind && link.ObjectID == id
			}
			if !found {
				return apierrors.New(apierrors.InvalidReference, "main discussion association is required")
			}
		}
		if err = rows.Err(); err != nil {
			return err
		}
		rows.Close()
		if _, err = tx.Exec(ctx, `DELETE FROM topic_work_links WHERE project_id=$1 AND topic_id=$2`, project, topic); err != nil {
			return err
		}
	}
	var added int64
	for _, link := range links {
		tag, err := tx.Exec(ctx, `INSERT INTO topic_work_links(project_id,topic_id,object_type,object_id,created_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, project, topic, link.ObjectType, link.ObjectID, now)
		if err != nil {
			return err
		}
		added += tag.RowsAffected()
	}
	if !replace && added == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE topics SET links_version=links_version+1 WHERE project_id=$1 AND id=$2`, project, topic); err != nil {
		return err
	}
	_, err := events.AppendProjectEvent(ctx, tx, project, "topic.links.changed", "topic", topic.String(), nil, map[string]any{"linksVersion": version + 1}, now)
	return err
}

// SPDX-License-Identifier: Apache-2.0
package project

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type MemberPreview struct {
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
}
type ProjectSummary struct {
	MemberCount   int             `json:"memberCount"`
	TopicCount    int             `json:"topicCount"`
	MemberPreview []MemberPreview `json:"memberPreview"`
}
type ListOptions struct {
	Query, Ownership string
	Summary          bool
}

func projectReadScope(ctx context.Context) []uuid.UUID {
	if actor := auth.FromContext(ctx); actor != nil && actor.Kind == auth.KindCLI {
		return nonNilProjectScope(actor.ProjectScope)
	}
	return []uuid.UUID{}
}

// ListOverview applies search and ownership before pagination. Summaries are
// fetched for the whole returned page in one query, never one HTTP call per card.
func (s *Service) ListOverview(ctx context.Context, user uuid.UUID, limit int, afterTime *time.Time, afterID *uuid.UUID, options ListOptions) ([]Project, bool, int, error) {
	options.Query = strings.TrimSpace(options.Query)
	if options.Ownership == "" {
		options.Ownership = "all"
	}
	if options.Ownership != "all" && options.Ownership != "owned" && options.Ownership != "joined" {
		return nil, false, 0, apierrors.Fields("ownership", "invalid")
	}
	if len([]rune(options.Query)) > 200 {
		return nil, false, 0, apierrors.Fields("q", "too_long")
	}
	if limit < 1 || limit > 200 {
		return nil, false, 0, apierrors.Fields("limit", "invalid")
	}
	scope := projectReadScope(ctx)
	if actor := auth.FromContext(ctx); actor != nil && actor.Kind == auth.KindCLI && len(actor.ProjectScope) == 0 {
		if options.Summary {
			return nil, false, 0, apierrors.New(apierrors.Forbidden, "project summaries require an explicit project scope")
		}
		if options.Ownership == "joined" {
			return []Project{}, false, 0, nil
		}
		options.Ownership = "owned"
	}
	// strpos is literal, so user-supplied %/_ do not unexpectedly become wildcards.
	filter := ` FROM project_members m JOIN projects p ON p.id=m.project_id
  WHERE m.user_id=$1 AND m.state='active'
   AND (cardinality($2::uuid[])=0 OR p.id=ANY($2))
   AND ($3='' OR strpos(lower(p.title || ' ' || p.description),lower($3))>0)
   AND ($4='all' OR ($4='owned' AND m.role='owner') OR ($4='joined' AND m.role<>'owner'))`
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+filter, user, scope, options.Query, options.Ownership).Scan(&total); err != nil {
		return nil, false, 0, apierrors.New(apierrors.Internal, "project count failed").Wrap(err)
	}
	rows, err := s.pool.Query(ctx, `SELECT p.id,p.title,p.description,p.kind,p.status,p.version,
  p.max_discussion_rounds,p.approval_timeout_seconds,p.default_model_id,p.created_at,p.updated_at,m.role`+filter+`
  AND ($5::timestamptz IS NULL OR (p.created_at,p.id)<($5,$6::uuid))
  ORDER BY p.created_at DESC,p.id DESC LIMIT $7`, user, scope, options.Query, options.Ownership, afterTime, afterID, limit+1)
	if err != nil {
		return nil, false, 0, apierrors.New(apierrors.Internal, "project list failed").Wrap(err)
	}
	out := make([]Project, 0)
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.Kind, &p.Status, &p.Version, &p.MaxDiscussionRounds, &p.ApprovalTimeoutSeconds, &p.DefaultModelID, &p.CreatedAt, &p.UpdatedAt, &p.ViewerRole); err != nil {
			rows.Close()
			return nil, false, 0, apierrors.New(apierrors.Internal, "project scan failed").Wrap(err)
		}
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, false, 0, apierrors.New(apierrors.Internal, "project list failed").Wrap(err)
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	if options.Summary && len(out) > 0 {
		ids := make([]uuid.UUID, len(out))
		byID := make(map[uuid.UUID]*Project, len(out))
		for i := range out {
			ids[i] = out[i].ID
			byID[out[i].ID] = &out[i]
		}
		summaryRows, err := s.pool.Query(ctx, `SELECT p.id,
   (SELECT count(*) FROM project_members m WHERE m.project_id=p.id AND m.state='active'),
   (SELECT count(*) FROM topics t WHERE t.project_id=p.id AND t.kind='discussion'),
   COALESCE((SELECT jsonb_agg(jsonb_build_object('userId',x.user_id,'displayName',x.display_name) ORDER BY x.joined_at,x.user_id)
    FROM (SELECT m.user_id,u.display_name,m.joined_at FROM project_members m JOIN users u ON u.id=m.user_id
     WHERE m.project_id=p.id AND m.state='active' ORDER BY m.joined_at,m.user_id LIMIT 3) x),'[]'::jsonb)
   FROM projects p WHERE p.id=ANY($1)`, ids)
		if err != nil {
			return nil, false, 0, apierrors.New(apierrors.Internal, "project summaries failed").Wrap(err)
		}
		for summaryRows.Next() {
			var id uuid.UUID
			var summary ProjectSummary
			var members []byte
			if err := summaryRows.Scan(&id, &summary.MemberCount, &summary.TopicCount, &members); err != nil {
				summaryRows.Close()
				return nil, false, 0, apierrors.New(apierrors.Internal, "summary scan failed").Wrap(err)
			}
			if err := json.Unmarshal(members, &summary.MemberPreview); err != nil {
				summaryRows.Close()
				return nil, false, 0, apierrors.New(apierrors.Internal, "summary decode failed").Wrap(err)
			}
			byID[id].Summary = &summary
		}
		summaryRows.Close()
		if err := summaryRows.Err(); err != nil {
			return nil, false, 0, apierrors.New(apierrors.Internal, "project summaries failed").Wrap(err)
		}
	}
	return out, more, total, nil
}

type RecentTopic struct {
	TopicID        uuid.UUID `json:"topicId"`
	Title          string    `json:"title"`
	ProjectID      uuid.UUID `json:"projectId"`
	ProjectTitle   string    `json:"projectTitle"`
	LastActivityAt time.Time `json:"lastActivityAt"`
}

// RecentTopics shares the list's active-membership and grant-scope boundary.
func (s *Service) RecentTopics(ctx context.Context, user uuid.UUID, limit int) ([]RecentTopic, error) {
	if limit < 1 || limit > 100 {
		return nil, apierrors.Fields("limit", "invalid")
	}
	if actor := auth.FromContext(ctx); actor != nil && actor.Kind == auth.KindCLI && len(actor.ProjectScope) == 0 {
		return []RecentTopic{}, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT t.id,t.title,p.id,p.title,COALESCE(last_message.created_at,t.created_at) AS activity
  FROM project_members m JOIN projects p ON p.id=m.project_id JOIN topics t ON t.project_id=p.id
  LEFT JOIN LATERAL (SELECT created_at FROM messages WHERE topic_id=t.id ORDER BY seq DESC LIMIT 1) last_message ON true
  WHERE m.user_id=$1 AND m.state='active' AND t.kind='discussion' AND p.status='active'
   AND (cardinality($2::uuid[])=0 OR p.id=ANY($2))
  ORDER BY activity DESC,t.id DESC LIMIT $3`, user, projectReadScope(ctx), limit)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "recent discussions failed").Wrap(err)
	}
	defer rows.Close()
	out := make([]RecentTopic, 0)
	for rows.Next() {
		var topic RecentTopic
		if err := rows.Scan(&topic.TopicID, &topic.Title, &topic.ProjectID, &topic.ProjectTitle, &topic.LastActivityAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "recent discussion scan failed").Wrap(err)
		}
		out = append(out, topic)
	}
	if err := rows.Err(); err != nil {
		return nil, apierrors.New(apierrors.Internal, "recent discussions failed").Wrap(err)
	}
	return out, nil
}

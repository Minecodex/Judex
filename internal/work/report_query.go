package work

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"time"
)

type ReportRecord struct {
	ID                 uuid.UUID       `json:"id"`
	TaskID             uuid.UUID       `json:"taskId"`
	IdentityID         uuid.UUID       `json:"identityId"`
	Kind               string          `json:"reportKind"`
	BindingVersion     int64           `json:"bindingVersion"`
	Text               string          `json:"text"`
	CreatedBy          *uuid.UUID      `json:"createdBy"`
	CreatedAt          time.Time       `json:"createdAt"`
	SubmissionID       *uuid.UUID      `json:"submissionId"`
	MaterialVersionIDs []string        `json:"materialVersionIds"`
	CodeRefs           json.RawMessage `json:"codeRefs"`
	EnvironmentRefs    json.RawMessage `json:"environmentRefs"`
}

func (s *Service) ListReports(ctx context.Context, user, project, task uuid.UUID, reportIDs ...uuid.UUID) ([]ReportRecord, error) {
	if _, err := s.GetTask(ctx, user, project, task); err != nil {
		return nil, err
	}
	var selected *uuid.UUID
	if len(reportIDs) > 0 {
		selected = &reportIDs[0]
	}
	rows, err := paging.Query(ctx, s.pool, `SELECT r.id,r.task_id,r.identity_id,r.report_kind,COALESCE(r.progress_hint,''),r.created_by,r.created_at,r.submission_id,
  ARRAY(SELECT material_version_id::text FROM work_report_materials WHERE report_id=r.id ORDER BY material_version_id),r.code_refs_json,r.environment_refs_json,r.binding_version
  /*keys*/ FROM work_reports r WHERE r.project_id=$1 AND r.task_id=$2 AND ($3::uuid IS NULL OR r.id=$3) /*page*/`, "r.created_at", "r.id", project, task, selected)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReportRecord{}
	for rows.Next() {
		var r ReportRecord
		if err = rows.Scan(&r.ID, &r.TaskID, &r.IdentityID, &r.Kind, &r.Text, &r.CreatedBy, &r.CreatedAt, &r.SubmissionID, &r.MaterialVersionIDs, &r.CodeRefs, &r.EnvironmentRefs, &r.BindingVersion); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

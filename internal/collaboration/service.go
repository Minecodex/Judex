package collaboration

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type Service struct{ Pool *postgres.Pool }

func NewService(pool *postgres.Pool) *Service { return &Service{Pool: pool} }
func Member(ctx context.Context, q Reader, user, project uuid.UUID) error {
	var active bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_members m JOIN users u ON u.id=m.user_id WHERE m.project_id=$1 AND m.user_id=$2 AND m.state='active' AND u.status='active')`, project, user).Scan(&active); err != nil {
		return err
	}
	if !active {
		return apierrors.New(apierrors.NotFound, "project not found")
	}
	return nil
}

type Analysis struct {
	ID            uuid.UUID       `json:"id"`
	TaskID        *uuid.UUID      `json:"taskId"`
	ScopeKind     string          `json:"scopeKind"`
	Output        json.RawMessage `json:"output"`
	BatchID       *uuid.UUID      `json:"batchId"`
	SourceType    string          `json:"sourceType"`
	SourceID      uuid.UUID       `json:"sourceId"`
	State         string          `json:"state"`
	Summary       string          `json:"summary"`
	Disagreements json.RawMessage `json:"disagreements"`
	Basis         json.RawMessage `json:"basis"`
	ErrorCode     *string         `json:"errorCode"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

func (a *Analysis) setScopeKind() {
	a.ScopeKind = "task"
	if a.TaskID == nil {
		a.ScopeKind = "project"
	}
}

type MaterialRef struct {
	VersionID uuid.UUID `json:"versionId"`
	Name      string    `json:"name"`
}
type Activity struct {
	ID                 uuid.UUID     `json:"id"`
	TaskID             uuid.UUID     `json:"taskId"`
	Kind               string        `json:"kind"`
	SourceType         string        `json:"sourceType"`
	Source             string        `json:"source"`
	Text               string        `json:"text"`
	ActorUserID        *uuid.UUID    `json:"actorUserId"`
	ActorName          string        `json:"actorName"`
	IdentityID         *uuid.UUID    `json:"identityId"`
	CreatedAt          time.Time     `json:"createdAt"`
	MaterialVersionIDs []string      `json:"materialVersionIds"`
	Materials          []MaterialRef `json:"materials"`
	TopicIDs           []string      `json:"topicIds"`
	Analysis           *Analysis     `json:"analysis"`
}
type Suggestion struct {
	ID               uuid.UUID  `json:"id"`
	AnalysisID       uuid.UUID  `json:"analysisId"`
	TaskID           uuid.UUID  `json:"taskId"`
	PlanID           *uuid.UUID `json:"planId"`
	Title            string     `json:"title"`
	Reason           string     `json:"reason"`
	ParentTopicID    *uuid.UUID `json:"parentTopicId"`
	ForkAfterSeq     int64      `json:"forkAfterSeq"`
	SuggestedTopicID *uuid.UUID `json:"suggestedTopicId"`
	State            string     `json:"state"`
	Version          int64      `json:"version"`
	ResultTopicID    *uuid.UUID `json:"resultTopicId"`
	SourceRef        SourceRef  `json:"sourceRef"`
	CreatedAt        time.Time  `json:"createdAt"`
}
type ResolveInput struct {
	ExpectedVersion int64      `json:"expectedVersion"`
	Mode            string     `json:"mode"`
	Title           string     `json:"title"`
	TopicID         *uuid.UUID `json:"topicId"`
	Links           []Link     `json:"links"`
}

func nullID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
func (s *Service) GetAnalysis(ctx context.Context, user, project, id uuid.UUID) (Analysis, error) {
	var out Analysis
	if err := Member(ctx, s.Pool, user, project); err != nil {
		return out, err
	}
	err := s.Pool.QueryRow(ctx, `SELECT id,task_id,batch_id,source_type,source_id,state,summary,disagreements,basis,error_code,updated_at,output_json FROM task_analyses WHERE project_id=$1 AND id=$2`, project, id).Scan(&out.ID, &out.TaskID, &out.BatchID, &out.SourceType, &out.SourceID, &out.State, &out.Summary, &out.Disagreements, &out.Basis, &out.ErrorCode, &out.UpdatedAt, &out.Output)
	out.setScopeKind()
	if errors.Is(err, pgx.ErrNoRows) {
		return out, apierrors.New(apierrors.NotFound, "analysis not found")
	}
	return out, err
}

// SubmissionAnalysis resolves the canonical report source where a submission
// backs a work report. It never reveals an analysis outside this project.
func (s *Service) SubmissionAnalysis(ctx context.Context, user, project, submission uuid.UUID) (Analysis, error) {
	if err := Member(ctx, s.Pool, user, project); err != nil {
		return Analysis{}, err
	}
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT a.id FROM submissions s
 LEFT JOIN work_reports r ON r.submission_id=s.id AND r.project_id=s.project_id
 JOIN task_analyses a ON a.project_id=s.project_id AND
 ((a.source_type='submission' AND a.source_id=s.id) OR (a.source_type='report' AND a.source_id=r.id))
 WHERE s.project_id=$1 AND s.id=$2 ORDER BY a.created_at DESC LIMIT 1`, project, submission).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, apierrors.New(apierrors.NotFound, "submission analysis not found")
	}
	if err != nil {
		return Analysis{}, err
	}
	return s.GetAnalysis(ctx, user, project, id)
}

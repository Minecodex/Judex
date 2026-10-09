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
	TaskID        uuid.UUID       `json:"taskId"`
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
	err := s.Pool.QueryRow(ctx, `SELECT id,task_id,batch_id,source_type,source_id,state,summary,disagreements,basis,error_code,updated_at FROM task_analyses WHERE project_id=$1 AND id=$2`, project, id).Scan(&out.ID, &out.TaskID, &out.BatchID, &out.SourceType, &out.SourceID, &out.State, &out.Summary, &out.Disagreements, &out.Basis, &out.ErrorCode, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, apierrors.New(apierrors.NotFound, "analysis not found")
	}
	return out, err
}

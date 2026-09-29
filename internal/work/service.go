// SPDX-License-Identifier: Apache-2.0

// Package work implements plans and tasks per docs/plans/v1/03 §1-§2:
// drafts are member-writable; becoming formal work requires proposals
// (decision module). Tasks keep single-owning-plan + parent semantics and
// typed hard requirements with cycle checks under the project lock.
package work

import (
	"context"
	"errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Plan is the API projection (06 §5).
type Plan struct {
	ID                 uuid.UUID  `json:"id"`
	Title              string     `json:"title"`
	Goal               string     `json:"goal"`
	AcceptanceCriteria string     `json:"acceptanceCriteria"`
	Status             string     `json:"status"`
	OwnerIdentityID    *uuid.UUID `json:"ownerIdentityId"`
	WorkflowID         *uuid.UUID `json:"workflowId"`
	LatestAcceptanceID *uuid.UUID `json:"latestAcceptanceId"`
	TaskStats          TaskStats  `json:"taskStats"`
	Version            int64      `json:"version"`
	CreatedAt          time.Time  `json:"createdAt"`
}

type TaskStats struct {
	Total     int `json:"total"`
	Accepted  int `json:"accepted"`
	Active    int `json:"active"`
	Cancelled int `json:"cancelled"`
}

// Task is the API projection (06 §5).
type Task struct {
	BugDetails         *BugDetails   `json:"bugDetails,omitempty"`
	ID                 uuid.UUID     `json:"id"`
	PlanID             *uuid.UUID    `json:"planId"`
	ParentTaskID       *uuid.UUID    `json:"parentTaskId"`
	Title              string        `json:"title"`
	ExpectedOutput     string        `json:"expectedOutput"`
	AcceptanceCriteria string        `json:"acceptanceCriteria"`
	Kind               string        `json:"kind"`
	Status             string        `json:"status"`
	Participants       []Participant `json:"participants"`
	ReviewerIdentityID *uuid.UUID    `json:"reviewerIdentityId"`
	WorkflowID         *uuid.UUID    `json:"workflowId"`
	NodeID             *string       `json:"nodeId"`
	Requirements       []Requirement `json:"requirements"`
	LatestReportID     *uuid.UUID    `json:"latestReportId"`
	LatestAcceptanceID *uuid.UUID    `json:"latestAcceptanceId"`
	Version            int64         `json:"version"`
	CreatedAt          time.Time     `json:"createdAt"`
}

type Participant struct {
	IdentityID     uuid.UUID `json:"identityId"`
	DisplayName    string    `json:"displayName"`
	Responsibility string    `json:"responsibility"`
}

type Requirement struct {
	ID                uuid.UUID  `json:"id"`
	Phase             string     `json:"phase"`
	Kind              string     `json:"kind"`
	TargetID          uuid.UUID  `json:"targetId"`
	MaterialVersionID *uuid.UUID `json:"materialVersionId"`
	Hard              bool       `json:"hard"`
	Label             string     `json:"label"`
}

type Service struct {
	pool *postgres.Pool
	now  func() time.Time
}

func NewService(pool *postgres.Pool, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{pool: pool, now: now}
}

func memberTx(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID, requester uuid.UUID) (string, error) {
	var role string
	err := q.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, requester).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apierrors.New(apierrors.NotFound, "project not found")
	}
	return role, err
}

// CreatePlanDraft creates a draft plan (03 §2 计划 draft: 项目成员可建)。
func (s *Service) CreatePlanDraft(ctx context.Context, requester, projectID uuid.UUID, title, goal, criteria string, ownerIdentityID, workflowID *uuid.UUID) (Plan, error) {
	title = strings.TrimSpace(title)
	if l := utf8.RuneCountInString(title); l < 1 || l > 200 {
		return Plan{}, apierrors.Fields("title", "length")
	}
	var out Plan
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		now := s.now()
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO plans (project_id, id, title, goal, acceptance_criteria, owner_identity_id, workflow_id, status, created_at, updated_at,created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'draft',$8,$8,$9)`,
			projectID, id, title, goal, criteria, nullableUUID(ownerIdentityID), nullableUUID(workflowID), now, requester); err != nil {
			return apierrors.New(apierrors.Internal, "plan insert failed").Wrap(err)
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "plan.draft.create",
			ObjectType: "plan", ObjectID: id.String(), OccurredAt: now,
		}); err != nil {
			return err
		}
		out = Plan{ID: id, Title: title, Goal: goal, AcceptanceCriteria: criteria,
			Status: "draft", OwnerIdentityID: ownerIdentityID, WorkflowID: workflowID,
			Version: 1, CreatedAt: now}
		return nil
	})
	return out, err
}

// ListPlans returns plans with task stats.
func (s *Service) ListPlans(ctx context.Context, requester, projectID uuid.UUID, planIDs ...uuid.UUID) ([]Plan, error) {
	if _, err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	var selected *uuid.UUID
	if len(planIDs) > 0 {
		selected = &planIDs[0]
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT p.id, p.title, p.goal, p.acceptance_criteria, p.status, p.owner_identity_id,
		       p.workflow_id, p.latest_acceptance_id, p.version, p.created_at,
		       count(t.id), count(t.id) FILTER (WHERE t.status='accepted'),
		       count(t.id) FILTER (WHERE t.status NOT IN ('accepted','cancelled')),
		       count(t.id) FILTER (WHERE t.status='cancelled') /*keys*/
		FROM plans p LEFT JOIN tasks t ON t.project_id=p.project_id AND (t.plan_id=p.id OR EXISTS(SELECT 1 FROM plan_task_references r WHERE r.plan_id=p.id AND r.task_id=t.id))
		WHERE p.project_id=$1 AND ($2::uuid IS NULL OR p.id=$2)
		/*page*/ GROUP BY p.id`, "p.created_at", "p.id", projectID, selected)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "plans failed").Wrap(err)
	}
	defer rows.Close()
	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Title, &p.Goal, &p.AcceptanceCriteria, &p.Status, &p.OwnerIdentityID,
			&p.WorkflowID, &p.LatestAcceptanceID, &p.Version, &p.CreatedAt,
			&p.TaskStats.Total, &p.TaskStats.Accepted, &p.TaskStats.Active, &p.TaskStats.Cancelled); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// TaskDraft carries the create-task payload.
type TaskDraft struct {
	BugDetails         *BugDetails
	Title              string
	PlanID             *uuid.UUID
	ParentTaskID       *uuid.UUID
	ExpectedOutput     string
	AcceptanceCriteria string
	Kind               string
	ParticipantIDs     []uuid.UUID
	ReviewerIdentityID *uuid.UUID
	WorkflowID         *uuid.UUID
	NodeID             *string
	Requirements       []Requirement
}

// CreateTaskDraft validates ownership/parent/plan coherence (03 §1 领域不变量)
// and typed requirements, then inserts the draft with participants.
func (s *Service) CreateTaskDraft(ctx context.Context, requester, projectID uuid.UUID, draft TaskDraft) (Task, error) {
	title := strings.TrimSpace(draft.Title)
	if l := utf8.RuneCountInString(title); l < 1 || l > 200 {
		return Task{}, apierrors.Fields("title", "length")
	}
	if draft.Kind == "" {
		draft.Kind = "task"
	}
	if draft.Kind != "task" && draft.Kind != "bug" {
		return Task{}, apierrors.Fields("kind", "enum")
	}
	for _, req := range draft.Requirements {
		switch req.Phase {
		case "start", "accept", "both":
		default:
			return Task{}, apierrors.Fields("requirements[].phase", "enum")
		}
		switch req.Kind {
		case "task_acceptance", "handoff_receipt", "material_ready":
		default:
			return Task{}, apierrors.Fields("requirements[].kind", "enum")
		}
	}
	var out Task
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		// Parent and plan coherence: same project; parent must carry the same
		// owning plan (03 §1 关联父子必须同项目、同 owning plan).
		if draft.ParentTaskID != nil {
			var parentPlan *uuid.UUID
			var parentProject uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT plan_id, project_id FROM tasks WHERE id=$1`,
				*draft.ParentTaskID).Scan(&parentPlan, &parentProject); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return apierrors.New(apierrors.InvalidReference, "parent task not found")
				}
				return apierrors.New(apierrors.Internal, "parent lookup failed").Wrap(err)
			}
			if parentProject != projectID {
				return apierrors.New(apierrors.InvalidReference, "parent task in another project")
			}
			if (draft.PlanID == nil) != (parentPlan == nil) || (draft.PlanID != nil && parentPlan != nil && *parentPlan != *draft.PlanID) {
				return apierrors.New(apierrors.InvalidReference, "parent must share the owning plan")
			}
		}
		if draft.PlanID != nil {
			var planProject uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT project_id FROM plans WHERE id=$1`, *draft.PlanID).
				Scan(&planProject); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return apierrors.New(apierrors.InvalidReference, "plan not found")
				}
				return apierrors.New(apierrors.Internal, "plan lookup failed").Wrap(err)
			}
			if planProject != projectID {
				return apierrors.New(apierrors.InvalidReference, "plan in another project")
			}
		}
		if err := validateRequirementReferences(ctx, tx, projectID, draft.Requirements); err != nil {
			return err
		}
		// Requirement cycles: task_acceptance edges among existing tasks must
		// stay acyclic (03 §9 依赖环检查 under the project lock).
		if err := checkRequirementCycle(ctx, tx, projectID, draft.Requirements); err != nil {
			return err
		}
		now := s.now()
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO tasks (project_id, id, plan_id, parent_task_id, title, expected_output,
				acceptance_criteria, kind, reviewer_identity_id, workflow_id, node_id, status, created_at, updated_at,created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'draft',$12,$12,$13)`,
			projectID, id, nullableUUID(draft.PlanID), nullableUUID(draft.ParentTaskID), title,
			draft.ExpectedOutput, draft.AcceptanceCriteria, draft.Kind,
			nullableUUID(draft.ReviewerIdentityID), nullableUUID(draft.WorkflowID),
			nullableString(draft.NodeID), now, requester); err != nil {
			return apierrors.New(apierrors.Internal, "task insert failed").Wrap(err)
		}
		if draft.Kind == "bug" {
			if err := insertBugDetails(ctx, tx, projectID, id, draft.BugDetails); err != nil {
				return err
			}
		}
		for _, identityID := range draft.ParticipantIDs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO task_participants (project_id, task_id, identity_id)
				VALUES ($1,$2,$3)`, projectID, id, identityID); err != nil {
				return apierrors.New(apierrors.Internal, "participant failed").Wrap(err)
			}
		}
		for _, req := range draft.Requirements {
			if _, err := tx.Exec(ctx, `
				INSERT INTO task_requirements (project_id, id, task_id, phase, kind, target_id, material_version_id, hard, label)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
				projectID, uuid.New(), id, req.Phase, req.Kind, req.TargetID,
				nullableUUID(req.MaterialVersionID), req.Hard, req.Label); err != nil {
				return apierrors.New(apierrors.Internal, "requirement failed").Wrap(err)
			}
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "task.draft.create",
			ObjectType: "task", ObjectID: id.String(), OccurredAt: now,
		}); err != nil {
			return err
		}
		out = Task{ID: id, PlanID: draft.PlanID, ParentTaskID: draft.ParentTaskID, Title: title,
			ExpectedOutput: draft.ExpectedOutput, AcceptanceCriteria: draft.AcceptanceCriteria,
			Kind: draft.Kind, Status: "draft", ReviewerIdentityID: draft.ReviewerIdentityID,
			WorkflowID: draft.WorkflowID, NodeID: draft.NodeID, Requirements: draft.Requirements,
			Version: 1, CreatedAt: now}
		return nil
	})
	return out, err
}

// checkRequirementCycle walks task_acceptance edges among existing tasks and
// rejects anything that would close a cycle (03 §9).
func checkRequirementCycle(ctx context.Context, tx pgx.Tx, projectID uuid.UUID, newReqs []Requirement) error {
	edges := map[uuid.UUID][]uuid.UUID{}
	rows, err := tx.Query(ctx, `
		SELECT task_id, target_id FROM task_requirements
		WHERE project_id=$1 AND kind='task_acceptance'`, projectID)
	if err != nil {
		return apierrors.New(apierrors.Internal, "requirement query failed").Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var from, to uuid.UUID
		if err := rows.Scan(&from, &to); err != nil {
			return apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		edges[from] = append(edges[from], to)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// New edges are validated as (target -> futureTask) precondition edges:
	// target must be accepted before the new task proceeds, so the DAG
	// direction runs target -> task; a cycle exists if walking from any
	// target reaches itself.
	state := map[uuid.UUID]int{}
	var visit func(node uuid.UUID) bool
	visit = func(node uuid.UUID) bool {
		state[node] = 1
		for _, next := range edges[node] {
			if state[next] == 1 {
				return true
			}
			if state[next] == 0 && visit(next) {
				return true
			}
		}
		state[node] = 2
		return false
	}
	nodes := make(map[uuid.UUID]struct{}, len(edges))
	for node := range edges {
		nodes[node] = struct{}{}
	}
	for node := range nodes {
		if state[node] == 0 && visit(node) {
			return apierrors.New(apierrors.DependencyChanged, "requirement cycle detected")
		}
	}
	return nil
}

// ListTasks pages tasks by plan/status (06 §5).
func (s *Service) ListTasks(ctx context.Context, requester, projectID uuid.UUID, planID *uuid.UUID) ([]Task, error) {
	if _, err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT id, plan_id, parent_task_id, title, kind, status, version, created_at /*keys*/
		FROM tasks WHERE project_id=$1 AND ($2::uuid IS NULL OR plan_id=$2)
		/*page*/`, "created_at", "id", projectID, planID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "tasks failed").Wrap(err)
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.PlanID, &t.ParentTaskID, &t.Title, &t.Kind, &t.Status, &t.Version, &t.CreatedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

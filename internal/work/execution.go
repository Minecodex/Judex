// SPDX-License-Identifier: Apache-2.0

package work

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Blocker is one unmet hard requirement (06 §5 blockers).
type Blocker struct {
	Phase      string `json:"phase"`
	ObjectType string `json:"objectType"`
	ObjectID   string `json:"objectId"`
	Reason     string `json:"reason"`
}

// RequirementsFor loads a task's typed requirements with satisfaction.
type dbQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Service) RequirementsFor(ctx context.Context, q dbQuery, projectID, taskID uuid.UUID) ([]Requirement, []Blocker, error) {
	rows, err := q.Query(ctx, `
		SELECT id, phase, kind, target_id, material_version_id, hard, label
		FROM task_requirements WHERE project_id=$1 AND task_id=$2`, projectID, taskID)
	if err != nil {
		return nil, nil, apierrors.New(apierrors.Internal, "requirements failed").Wrap(err)
	}
	defer rows.Close()
	var reqs []Requirement
	for rows.Next() {
		var r Requirement
		if err := rows.Scan(&r.ID, &r.Phase, &r.Kind, &r.TargetID, &r.MaterialVersionID, &r.Hard, &r.Label); err != nil {
			return nil, nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		reqs = append(reqs, r)
	}
	blockers, err := s.evaluateBlockers(ctx, q, reqs)
	return reqs, blockers, err
}

// evaluateBlockers computes hard precondition satisfaction (03 §2):
// task_acceptance targets must be accepted; handoff_receipt needs an
// accepted source version; material_ready needs a ready version.
func (s *Service) evaluateBlockers(ctx context.Context, q dbQuery, reqs []Requirement) ([]Blocker, error) {
	var blockers []Blocker
	for _, req := range reqs {
		if !req.Hard {
			continue
		}
		switch req.Kind {
		case "task_acceptance":
			var status string
			err := q.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, req.TargetID).Scan(&status)
			if errors.Is(err, pgx.ErrNoRows) {
				blockers = append(blockers, Blocker{Phase: req.Phase, ObjectType: "task", ObjectID: req.TargetID.String(), Reason: "前置任务不存在"})
				continue
			}
			if err != nil {
				return nil, apierrors.New(apierrors.Internal, "task lookup failed").Wrap(err)
			}
			if status != "accepted" {
				blockers = append(blockers, Blocker{Phase: req.Phase, ObjectType: "task", ObjectID: req.TargetID.String(), Reason: "前置任务未验收（" + status + "）"})
			}
		case "handoff_receipt":
			var count int
			if err := q.QueryRow(ctx, `
				SELECT count(*) FROM source_versions v
				JOIN handoff_sources hs ON hs.id=v.source_id
				WHERE v.state='accepted' AND hs.source_task_id=$1`, req.TargetID).Scan(&count); err != nil {
				return nil, apierrors.New(apierrors.Internal, "handoff lookup failed").Wrap(err)
			}
			if count == 0 {
				blockers = append(blockers, Blocker{Phase: req.Phase, ObjectType: "handoff", ObjectID: req.TargetID.String(), Reason: "来源交接未被接收"})
			}
		case "material_ready":
			var state string
			err := q.QueryRow(ctx, `SELECT state FROM material_versions WHERE id=$1`, req.TargetID).Scan(&state)
			if errors.Is(err, pgx.ErrNoRows) {
				blockers = append(blockers, Blocker{Phase: req.Phase, ObjectType: "material", ObjectID: req.TargetID.String(), Reason: "材料版本不存在"})
				continue
			}
			if err != nil {
				return nil, apierrors.New(apierrors.Internal, "material lookup failed").Wrap(err)
			}
			if state != "ready" {
				blockers = append(blockers, Blocker{Phase: req.Phase, ObjectType: "material", ObjectID: req.TargetID.String(), Reason: "材料未就绪"})
			}
		}
	}
	return blockers, nil
}

// Start moves a task ready/rework -> working when the caller is a current
// participant and start-phase hard requirements hold (03 §2 开始任务).
func (s *Service) Start(ctx context.Context, requester, projectID, taskID uuid.UUID, identityID *uuid.UUID, expectedVersion int64) (Task, error) {
	var out Task
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var (
			status      string
			participant bool
		)
		if err := tx.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			taskID, projectID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "task not found")
			}
			return apierrors.New(apierrors.Internal, "task lookup failed").Wrap(err)
		}
		if status != "ready" && status != "rework" {
			return apierrors.New(apierrors.InvalidTransition, "task is "+status)
		}
		if identityID != nil {
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS(SELECT 1 FROM task_participants WHERE task_id=$1 AND identity_id=$2)`,
				taskID, *identityID).Scan(&participant); err != nil {
				return apierrors.New(apierrors.Internal, "participant lookup failed").Wrap(err)
			}
			if !participant {
				return apierrors.New(apierrors.Forbidden, "只有当前任务参与者可以开始")
			}
		}
		reqs, blockers, err := s.RequirementsFor(ctx, tx, projectID, taskID)
		if err != nil {
			return err
		}
		var startBlockers []Blocker
		for _, b := range blockers {
			if b.Phase == "start" || b.Phase == "both" {
				startBlockers = append(startBlockers, b)
			}
		}
		if len(startBlockers) > 0 {
			return apierrors.New(apierrors.RequirementUnmet, "开始前置未满足").
				WithDetails(map[string]any{"blockers": startBlockers})
		}
		tag, err := tx.Exec(ctx, `
			UPDATE tasks SET status='working', version=version+1, updated_at=$3
			WHERE id=$1 AND project_id=$2 AND version=$4`,
			taskID, projectID, s.now(), expectedVersion)
		if err != nil {
			return apierrors.New(apierrors.Internal, "start failed").Wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "task version conflict")
		}
		out = Task{ID: taskID, Status: "working", Requirements: reqs}
		return nil
	})
	return out, err
}

// MapNode / MapEdge compose the execution map (06 §5).
type MapNode struct {
	TaskID   string    `json:"taskId"`
	Title    string    `json:"title"`
	Status   string    `json:"status"`
	Column   int       `json:"column"`
	Blockers []Blocker `json:"blockers"`
}

type MapEdge struct {
	FromTaskID string `json:"fromTaskId"`
	ToTaskID   string `json:"toTaskId"`
	Phase      string `json:"phase"`
	Kind       string `json:"kind"`
}

type ExecutionMap struct {
	PlanID *uuid.UUID `json:"planId"`
	Nodes  []MapNode  `json:"nodes"`
	Edges  []MapEdge  `json:"edges"`
}

// ExecutionMap builds the left-to-right execution graph (08 §8): columns
// follow hard precondition depth; advisory edges display only; parent links
// are ownership, never ordering.
func (s *Service) ExecutionMap(ctx context.Context, requester, projectID uuid.UUID, planID *uuid.UUID) (ExecutionMap, error) {
	if _, err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return ExecutionMap{}, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, plan_id, parent_task_id, title, status FROM tasks
		WHERE project_id=$1 AND status<>'draft' AND status<>'cancelled'
		  AND ($2::uuid IS NULL OR plan_id=$2)`, projectID, planID)
	if err != nil {
		return ExecutionMap{}, apierrors.New(apierrors.Internal, "map failed").Wrap(err)
	}
	defer rows.Close()
	type node struct {
		id     string
		parent *string
		title  string
		status string
	}
	nodes := map[string]node{}
	var order []string
	for rows.Next() {
		var n node
		if err := rows.Scan(&n.id, new(uuid.UUID), &n.parent, &n.title, &n.status); err != nil {
			return ExecutionMap{}, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		nodes[n.id] = n
		order = append(order, n.id)
	}
	reqRows, err := s.pool.Query(ctx, `
		SELECT task_id, target_id, phase FROM task_requirements
		WHERE project_id=$1 AND kind='task_acceptance' AND hard`, projectID)
	if err != nil {
		return ExecutionMap{}, apierrors.New(apierrors.Internal, "edges failed").Wrap(err)
	}
	defer reqRows.Close()
	edges := map[string][]string{}
	edgeMeta := map[string]MapEdge{}
	for reqRows.Next() {
		var taskID, targetID string
		var phase string
		if err := reqRows.Scan(&taskID, &targetID, &phase); err != nil {
			return ExecutionMap{}, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		edges[taskID] = append(edges[taskID], targetID)
		edgeMeta[taskID+"|"+targetID] = MapEdge{FromTaskID: targetID, ToTaskID: taskID, Phase: phase, Kind: "hard"}
	}
	// Longest-path column assignment (targets left, dependents right).
	column := map[string]int{}
	var depth func(id string, seen map[string]bool) int
	depth = func(id string, seen map[string]bool) int {
		if c, ok := column[id]; ok {
			return c
		}
		if seen[id] {
			return 0
		}
		seen[id] = true
		max := 0
		for _, target := range edges[id] {
			if d := depth(target, seen) + 1; d > max {
				max = d
			}
		}
		column[id] = max
		return max
	}
	out := ExecutionMap{PlanID: planID}
	for _, id := range order {
		n := nodes[id]
		node := MapNode{TaskID: id, Title: n.title, Status: n.status, Column: depth(id, map[string]bool{})}
		_, blockers, err := s.RequirementsFor(ctx, poolAsQuery{s.pool}, projectID, parseUUID(id))
		if err == nil {
			node.Blockers = blockers
		}
		out.Nodes = append(out.Nodes, node)
		if n.parent != nil {
			out.Edges = append(out.Edges, MapEdge{FromTaskID: *n.parent, ToTaskID: id, Phase: "both", Kind: "parent"})
		}
	}
	for _, meta := range edgeMeta {
		out.Edges = append(out.Edges, meta)
	}
	return out, nil
}

type poolAsQuery struct{ *postgres.Pool }

func (p poolAsQuery) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.Pool.Query(ctx, sql, args...)
}
func (p poolAsQuery) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.Pool.QueryRow(ctx, sql, args...)
}

func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

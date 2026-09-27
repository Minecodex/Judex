// SPDX-License-Identifier: Apache-2.0

package work

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// BugDetails carries the bug payload (03 §8).
type BugDetails struct {
	SourceTaskID       *uuid.UUID
	ObservedReleaseRef string
	Environment        string
	Steps              string
	Expected           string
	Actual             string
	Severity           string
}

// CreateBug drafts a bug task with its details; linking to an accepted task
// proposes a reopen request rather than un-accepting (03 §8).
func (s *Service) CreateBug(ctx context.Context, requester, projectID uuid.UUID, title string, details BugDetails, planID *uuid.UUID) (Task, error) {
	if strings.TrimSpace(title) == "" {
		return Task{}, apierrors.Fields("title", "required")
	}
	switch details.Severity {
	case "low", "medium", "high", "critical":
	default:
		details.Severity = "medium"
	}
	var out Task
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		now := s.now()
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO tasks (project_id, id, plan_id, title, expected_output, acceptance_criteria,
				kind, status, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,'bug','draft',$7,$7)`,
			projectID, id, nullableUUID(planID), title, details.Expected, details.Actual, now); err != nil {
			return apierrors.New(apierrors.Internal, "bug insert failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO bug_details (project_id, task_id, source_task_id, observed_release_ref,
				environment, steps, expected, actual, severity)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			projectID, id, nullableUUID(details.SourceTaskID), details.ObservedReleaseRef,
			details.Environment, details.Steps, details.Expected, details.Actual, details.Severity); err != nil {
			return apierrors.New(apierrors.Internal, "bug details failed").Wrap(err)
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "bug.create",
			ObjectType: "task", ObjectID: id.String(), OccurredAt: now,
		}); err != nil {
			return err
		}
		out = Task{ID: id, Title: title, Kind: "bug", Status: "draft", CreatedAt: now}
		return nil
	})
	return out, err
}

// ReleaseReport is an environment/version fact report (03 §8).
type ReleaseReport struct {
	ID                uuid.UUID        `json:"id"`
	VersionLabel      string           `json:"versionLabel"`
	Environment       string           `json:"environment"`
	URL               string           `json:"url"`
	Status            string           `json:"status"`
	RepositoryCommits []map[string]any `json:"repositoryCommits"`
	ReportedBy        uuid.UUID        `json:"reportedBy"`
	CreatedAt         time.Time        `json:"createdAt"`
}

// ReportRelease records a deployment fact; it never triggers deployment
// itself (03 §8 是上报事实，不是部署命令).
func (s *Service) ReportRelease(ctx context.Context, requester, projectID uuid.UUID, versionLabel, environment, url, status string, repositoryCommits []map[string]any) (ReleaseReport, error) {
	if strings.TrimSpace(versionLabel) == "" || strings.TrimSpace(environment) == "" {
		return ReleaseReport{}, apierrors.Fields("versionLabel/environment", "required")
	}
	switch status {
	case "success", "partial", "failed":
	default:
		return ReleaseReport{}, apierrors.Fields("status", "enum")
	}
	var out ReleaseReport
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		now := s.now()
		id := uuid.New()
		commits, _ := jsonMarshal(repositoryCommits)
		if _, err := tx.Exec(ctx, `
			INSERT INTO release_reports (project_id, id, version_label, environment, url, status,
				repository_commits_json, reported_by, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)`,
			projectID, id, versionLabel, environment, nullableText(url), status, commits, requester, now); err != nil {
			return apierrors.New(apierrors.Internal, "release insert failed").Wrap(err)
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "release.report",
			ObjectType: "release_report", ObjectID: id.String(), OccurredAt: now,
		}); err != nil {
			return err
		}
		out = ReleaseReport{ID: id, VersionLabel: versionLabel, Environment: environment,
			URL: url, Status: status, RepositoryCommits: repositoryCommits,
			ReportedBy: requester, CreatedAt: now}
		return nil
	})
	return out, err
}

// ListReleases returns release reports newest first.
func (s *Service) ListReleases(ctx context.Context, requester, projectID uuid.UUID) ([]ReleaseReport, error) {
	if _, err := memberTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, version_label, environment, url, status, repository_commits_json, reported_by, created_at
		FROM release_reports WHERE project_id=$1 ORDER BY created_at DESC LIMIT 50`, projectID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "releases failed").Wrap(err)
	}
	defer rows.Close()
	var out []ReleaseReport
	for rows.Next() {
		var r ReleaseReport
		var url *string
		var commits []byte
		if err := rows.Scan(&r.ID, &r.VersionLabel, &r.Environment, &url, &r.Status, &commits, &r.ReportedBy, &r.CreatedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		if url != nil {
			r.URL = *url
		}
		_ = jsonUnmarshal(commits, &r.RepositoryCommits)
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateFixPropagation drafts independent fix tasks for user-chosen target
// releases (03 §8 用户明确选择目标，独立验收).
func (s *Service) CreateFixPropagation(ctx context.Context, requester, projectID, bugTaskID uuid.UUID, targetReleaseRefs []string) ([]uuid.UUID, error) {
	if len(targetReleaseRefs) == 0 {
		return nil, apierrors.Fields("targetReleaseRefs", "required")
	}
	var created []uuid.UUID
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var bugKind string
		if err := tx.QueryRow(ctx, `SELECT kind FROM tasks WHERE id=$1 AND project_id=$2`,
			bugTaskID, projectID).Scan(&bugKind); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.InvalidReference, "bug task not found")
			}
			return apierrors.New(apierrors.Internal, "bug lookup failed").Wrap(err)
		}
		if bugKind != "bug" {
			return apierrors.New(apierrors.InvalidReference, "fix propagation starts from a bug task")
		}
		now := s.now()
		for _, ref := range targetReleaseRefs {
			propagationID := uuid.New()
			targetTask := uuid.New()
			if _, err := tx.Exec(ctx, `
				INSERT INTO tasks (project_id, id, title, kind, status, created_at, updated_at)
				VALUES ($1,$2,$3,'task','draft',$4,$4)`,
				projectID, targetTask, "修复传播 → "+ref, now); err != nil {
				return apierrors.New(apierrors.Internal, "target task failed").Wrap(err)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO fix_propagations (project_id, id, bug_task_id, target_release_ref, target_task_id, state)
				VALUES ($1,$2,$3,$4,$5,'proposed')`,
				projectID, propagationID, bugTaskID, ref, targetTask); err != nil {
				return apierrors.New(apierrors.Internal, "propagation failed").Wrap(err)
			}
			created = append(created, targetTask)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "task.changed", "task", bugTaskID.String(), nil,
			map[string]any{"change": "fix_propagation", "targets": targetReleaseRefs}, now); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "fix.propagation.create",
			ObjectType: "task", ObjectID: bugTaskID.String(), OccurredAt: now,
		})
	})
	return created, err
}

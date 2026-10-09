package work

import (
	"context"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *Service) GetTask(ctx context.Context, user, project, task uuid.UUID) (Task, error) {
	out := Task{Participants: []Participant{}, Requirements: []Requirement{}}
	if _, err := memberTx(ctx, s.pool, project, user); err != nil {
		return out, err
	}
	err := s.pool.QueryRow(ctx, `SELECT id,plan_id,parent_task_id,title,expected_output,acceptance_criteria,kind,status,reviewer_identity_id,workflow_id,node_id,latest_report_id,latest_acceptance_id,version,created_at,main_topic_id FROM tasks WHERE id=$1 AND project_id=$2`, task, project).Scan(&out.ID, &out.PlanID, &out.ParentTaskID, &out.Title, &out.ExpectedOutput, &out.AcceptanceCriteria, &out.Kind, &out.Status, &out.ReviewerIdentityID, &out.WorkflowID, &out.NodeID, &out.LatestReportID, &out.LatestAcceptanceID, &out.Version, &out.CreatedAt, &out.MainTopicID)
	if err != nil {
		return out, apierrors.New(apierrors.NotFound, "task not found")
	}
	if out.Kind == "bug" {
		d := &BugDetails{}
		if err = s.pool.QueryRow(ctx, `SELECT source_task_id,COALESCE(observed_release_ref,''),environment,steps,expected,actual,severity FROM bug_details WHERE project_id=$1 AND task_id=$2`, project, task).Scan(&d.SourceTaskID, &d.ObservedReleaseRef, &d.Environment, &d.Steps, &d.Expected, &d.Actual, &d.Severity); err != nil {
			return out, err
		}
		out.BugDetails = d
	}
	rows, err := s.pool.Query(ctx, `SELECT p.identity_id,COALESCE(u.display_name,''),p.responsibility FROM task_participants p JOIN agent_identities i ON i.id=p.identity_id LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version LEFT JOIN users u ON u.id=b.user_id WHERE p.project_id=$1 AND p.task_id=$2 ORDER BY p.identity_id`, project, task)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var p Participant
		if err = rows.Scan(&p.IdentityID, &p.DisplayName, &p.Responsibility); err != nil {
			rows.Close()
			return out, err
		}
		out.Participants = append(out.Participants, p)
	}
	rows.Close()
	out.Requirements, _, err = s.RequirementsFor(ctx, poolAsQuery{s.pool}, project, task)
	if err == nil {
		all := []Task{out}
		err = s.decorateTasks(ctx, user, project, all)
		out = all[0]
	}
	return out, err
}

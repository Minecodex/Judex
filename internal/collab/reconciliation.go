package collab

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *Service) ownRun(ctx context.Context, group, run uuid.UUID) error {
	if _, e := s.Access(ctx, group); e != nil {
		return e
	}
	p, e := actor(ctx)
	if e != nil {
		return e
	}
	var own bool
	e = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_runs run JOIN role_agents r ON r.id=run.role_agent_id WHERE run.id=$1 AND r.group_id=$2 AND r.user_id=$3)`, run, group, p.UserID).Scan(&own)
	if e != nil {
		return e
	}
	if !own {
		return missing()
	}
	return nil
}
func (s *Service) ToolCalls(ctx context.Context, group, run uuid.UUID) ([]json.RawMessage, error) {
	if e := s.ownRun(ctx, group, run); e != nil {
		return nil, e
	}
	return s.list(ctx, `SELECT t.call_id AS "callId",t.name,t.args,t.state,t.result,t.replay_authorized AS "replayAuthorized",run.fence AS version FROM role_tool_calls t JOIN role_runs run ON run.id=t.run_id WHERE t.run_id=$1 ORDER BY t.created_at`, run)
}

type ReconcileInput struct {
	ExpectedVersion int64           `json:"expectedVersion"`
	Resolution      string          `json:"resolution"`
	Reason          string          `json:"reason"`
	Result          json.RawMessage `json:"result"`
}

func (s *Service) Reconcile(ctx context.Context, group, run uuid.UUID, call string, in ReconcileInput) error {
	if e := s.requireHuman(ctx); e != nil {
		return e
	}
	if e := validText(in.Reason, "reason", 4000, true); e != nil {
		return e
	}
	if in.Resolution != "completed" && in.Resolution != "retry" {
		return apierrors.Fields("resolution", "invalid")
	}
	if len(in.Result) > 16000 {
		return apierrors.Fields("result", "length")
	}
	return s.write(ctx, func(ctx context.Context) error {
		if e := s.ownRun(ctx, group, run); e != nil {
			return e
		}
		if e := s.lockConversation(ctx, group); e != nil {
			return e
		}
		p, _ := actor(ctx)
		var version int64
		var paused bool
		e := s.Pool.QueryRow(ctx, `SELECT r.fence,r.state='paused' AND d.state='paused' FROM role_runs r JOIN discussions d ON d.id=r.discussion_id WHERE r.id=$1 FOR UPDATE OF r`, run).Scan(&version, &paused)
		if e != nil {
			return e
		}
		if version != in.ExpectedVersion {
			return conflict()
		}
		if !paused {
			return apierrors.New(apierrors.InvalidTransition, "pause the discussion before reconciling effects")
		}
		var state, name string
		if e = s.Pool.QueryRow(ctx, `SELECT state,name FROM role_tool_calls WHERE run_id=$1 AND call_id=$2 FOR UPDATE`, run, call).Scan(&state, &name); e != nil {
			return missing()
		}
		if state != "started" && state != "unknown" {
			return apierrors.New(apierrors.InvalidTransition, "only an uncertain effect needs reconciliation")
		}
		audit, _ := json.Marshal(map[string]any{"userId": p.UserID, "reason": in.Reason, "resolution": in.Resolution})
		if in.Resolution == "completed" {
			result := defaultJSON(in.Result)
			if _, e = s.Pool.Exec(ctx, `UPDATE role_tool_calls SET state='completed',result=$3,reconciliation=$4,replay_authorized=false WHERE run_id=$1 AND call_id=$2`, run, call, result, audit); e != nil {
				return e
			}
			if e = s.knowledge(ctx, group, "tool", run.String()+":"+call, map[string]any{"name": name, "result": result}); e != nil {
				return e
			}
		} else {
			if _, e = s.Pool.Exec(ctx, `UPDATE role_tool_calls SET replay_authorized=true,reconciliation=$3 WHERE run_id=$1 AND call_id=$2`, run, call, audit); e != nil {
				return e
			}
		}
		_, e = s.Pool.Exec(ctx, `UPDATE role_runs SET fence=fence+1 WHERE id=$1`, run)
		return e
	})
}

package collab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// AdvancePath counts a closed causal path once and removes its closed segment.
// Role identities, rather than human accounts or role names, are the vertices.
func AdvancePath(path []uuid.UUID, target uuid.UUID) ([]uuid.UUID, bool) {
	for i, id := range path {
		if id == target {
			return append([]uuid.UUID{}, path[:i+1]...), true
		}
	}
	return append(append([]uuid.UUID{}, path...), target), false
}
func (s *Service) openDiscussion(ctx context.Context, group uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	e := s.Pool.QueryRow(ctx, `SELECT id FROM discussions WHERE group_id=$1 AND state IN('active','paused') FOR UPDATE`, group).Scan(&id)
	if e == nil {
		return id, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return id, e
	}
	id = uuid.New()
	_, e = s.Pool.Exec(ctx, `INSERT INTO discussions(id,group_id,state,allowance) SELECT $1,g.id,'active',p.loop_limit FROM groups g JOIN projects p ON p.id=g.project_id WHERE g.id=$2`, id, group)
	return id, e
}
func (s *Service) deliver(ctx context.Context, group, message, target uuid.UUID, pub *publication) error {
	var discussion uuid.UUID
	path := []uuid.UUID{}
	if pub == nil {
		var e error
		discussion, e = s.openDiscussion(ctx, group)
		if e != nil {
			return e
		}
	} else {
		discussion = pub.DiscussionID
		path = pub.Path
		if target == pub.RoleID {
			return nil
		}
	}
	// The conversation lock serializes routing, including competing branches.
	var exists bool
	if e := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM deliveries WHERE discussion_id=$1 AND role_agent_id=$2 AND trigger_message_id=$3)`, discussion, target, message).Scan(&exists); e != nil {
		return e
	}
	if exists {
		return nil
	}
	next, closed := AdvancePath(path, target)
	var loops, allowance int
	var state string
	if e := s.Pool.QueryRow(ctx, `SELECT loop_count,allowance,state FROM discussions WHERE id=$1`, discussion).Scan(&loops, &allowance, &state); e != nil {
		return e
	}
	deferred := closed && (state != "active" || loops >= allowance)
	id := uuid.New()
	if _, e := s.Pool.Exec(ctx, `INSERT INTO deliveries(id,discussion_id,role_agent_id,trigger_message_id,causal_path,closure_pending) VALUES($1,$2,$3,$4,$5,$6)`, id, discussion, target, message, next, deferred); e != nil {
		return e
	}
	if closed && !deferred {
		if _, e := s.Pool.Exec(ctx, `UPDATE discussions SET loop_count=loop_count+1,version=version+1 WHERE id=$1`, discussion); e != nil {
			return e
		}
	}
	if _, e := s.Pool.Exec(ctx, `INSERT INTO role_runs(id,delivery_id,role_agent_id,discussion_id) VALUES($1,$2,$3,$4)`, uuid.New(), id, target, discussion); e != nil {
		return e
	}
	payload, _ := json.Marshal(map[string]any{"deliveryId": id})
	if _, e := s.Pool.Exec(ctx, `INSERT INTO background_jobs(id,kind,payload,unique_key,run_after,created_at,updated_at,max_attempts) VALUES($1,'role.delivery',$2,$3,now(),now(),now(),8) ON CONFLICT(kind,unique_key) WHERE unique_key IS NOT NULL DO NOTHING`, uuid.New(), payload, id.String()); e != nil {
		return e
	}
	if e := s.Pool.QueryRow(ctx, `SELECT loop_count,allowance,state FROM discussions WHERE id=$1`, discussion).Scan(&loops, &allowance, &state); e != nil {
		return e
	}
	if state == "active" && loops >= allowance {
		return s.pauseDiscussion(ctx, group, discussion, "loop_limit")
	}
	return s.event(ctx, group, "discussion.changed", map[string]any{"discussionId": discussion})
}
func (s *Service) pauseDiscussion(ctx context.Context, group, id uuid.UUID, reason string) error {
	tag, e := s.Pool.Exec(ctx, `UPDATE discussions SET state='paused',pause_reason=$2,version=version+1 WHERE id=$1 AND state='active'`, id, reason)
	if e != nil || tag.RowsAffected() == 0 {
		return e
	}
	var version int64
	if e = s.Pool.QueryRow(ctx, `SELECT version FROM discussions WHERE id=$1`, id).Scan(&version); e != nil {
		return e
	}
	rows, e := s.Pool.Query(ctx, `SELECT DISTINCT r.user_id FROM deliveries d JOIN role_agents r ON r.id=d.role_agent_id JOIN conversation_members cm ON cm.conversation_id=r.group_id AND cm.user_id=r.user_id AND cm.active WHERE d.discussion_id=$1 ORDER BY r.user_id`, id)
	if e != nil {
		return e
	}
	users := []uuid.UUID{}
	for rows.Next() {
		var user uuid.UUID
		if e = rows.Scan(&user); e != nil {
			rows.Close()
			return e
		}
		users = append(users, user)
	}
	rows.Close()
	for _, user := range users {
		if e = s.notice(ctx, user, &group, nil, "discussion.paused", map[string]any{"discussionId": id, "reason": reason}, fmt.Sprintf("discussion:%s:%d", id, version)); e != nil {
			return e
		}
	}
	return s.event(ctx, group, "discussion.paused", map[string]any{"discussionId": id, "reason": reason})
}
func (s *Service) checkPublication(ctx context.Context, pub *publication) error {
	var valid bool
	e := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_runs run JOIN role_agents r ON r.id=run.role_agent_id JOIN discussions d ON d.id=run.discussion_id
 JOIN conversation_members cm ON cm.conversation_id=r.group_id AND cm.user_id=r.user_id AND cm.active JOIN project_members pm ON pm.project_id=(SELECT project_id FROM groups WHERE id=r.group_id) AND pm.user_id=r.user_id AND pm.state='active'
 WHERE run.id=$1 AND run.role_agent_id=$2 AND run.discussion_id=$3 AND run.fence=$4 AND run.state='running' AND run.lease_until>now()
 AND run.budget_started_at+interval '30 minutes'>clock_timestamp()
 AND r.active AND EXISTS(SELECT 1 FROM users u WHERE u.id=r.user_id AND u.status='active') AND d.state='active')`, pub.RunID, pub.RoleID, pub.DiscussionID, pub.Fence).Scan(&valid)
	if e != nil {
		return e
	}
	if !valid {
		return apierrors.New(apierrors.InvalidTransition, "run paused, terminated, expired, revoked or lease lost; publication rejected")
	}
	return nil
}
func (s *Service) Discussions(ctx context.Context, group uuid.UUID) ([]json.RawMessage, error) {
	a, e := s.Access(ctx, group)
	if e != nil {
		return nil, e
	}
	if a.Kind != "group" {
		return []json.RawMessage{}, nil
	}
	return s.list(ctx, `SELECT id,state,loop_count AS "loopCount",allowance,pause_reason AS "pauseReason",version,started_at AS "startedAt",ended_at AS "endedAt",
 COALESCE((SELECT jsonb_agg(jsonb_build_object('messageId',h.message_id,'userId',h.user_id,'roleAgentId',x.role_agent_id) ORDER BY h.message_id,h.user_id) FROM human_waits h JOIN deliveries x ON x.id=h.delivery_id WHERE x.discussion_id=d.id AND x.state IN('running','waiting_human') AND NOT h.resolved),'[]'::jsonb) AS "waitingFor",
 COALESCE((SELECT jsonb_agg(jsonb_build_object('action',action,'userId',user_id,'feedback',feedback,'allowance',allowance,'loopCount',loop_count,'createdAt',created_at) ORDER BY created_at) FROM discussion_controls WHERE discussion_id=d.id),'[]'::jsonb) AS controls FROM discussions d WHERE group_id=$1 ORDER BY started_at DESC LIMIT 30`, group)
}
func (s *Service) Control(ctx context.Context, group, id uuid.UUID, version int64, action, feedback string) error {
	if e := s.requireHuman(ctx); e != nil {
		return e
	}
	if action != "continue" && action != "terminate" {
		return apierrors.Fields("action", "invalid")
	}
	if len(feedback) > 16000 {
		return apierrors.Fields("feedback", "length")
	}
	return s.write(ctx, func(ctx context.Context) error {
		a, e := s.Access(ctx, group)
		if e != nil {
			return e
		}
		if a.ProjectID == nil {
			return missing()
		}
		if e = s.lockConversation(ctx, group); e != nil {
			return e
		}
		p, _ := actor(ctx)
		var state string
		var current int64
		var loops, allowance int
		if e = s.Pool.QueryRow(ctx, `SELECT state,version,loop_count,allowance FROM discussions WHERE id=$1 AND group_id=$2 FOR UPDATE`, id, group).Scan(&state, &current, &loops, &allowance); e != nil {
			return missing()
		}
		if current != version {
			return conflict()
		}
		if state != "active" && state != "paused" {
			return apierrors.New(apierrors.InvalidTransition, "discussion has ended")
		}
		if action == "continue" {
			if state != "paused" {
				return apierrors.New(apierrors.InvalidTransition, "only paused discussion may continue")
			}
			var limit int
			if e = s.Pool.QueryRow(ctx, `SELECT loop_limit FROM projects WHERE id=$1`, a.ProjectID).Scan(&limit); e != nil {
				return e
			}
			allowance += limit
			if _, e = s.Pool.Exec(ctx, `UPDATE discussions SET state='active',pause_reason=NULL,allowance=$2,version=version+1 WHERE id=$1`, id, allowance); e != nil {
				return e
			}
			// Count queued closures exactly once, within the newly granted allowance.
			// This also covers multiple closed mentions in the message that paused us.
			counted, err := s.Pool.Exec(ctx, `WITH ready AS(SELECT id FROM deliveries WHERE discussion_id=$1 AND closure_pending AND state IN('pending','running') ORDER BY created_at,id LIMIT $2 FOR UPDATE) UPDATE deliveries SET closure_pending=false WHERE id IN(SELECT id FROM ready)`, id, max(0, allowance-loops))
			if err != nil {
				return err
			}
			if n := counted.RowsAffected(); n > 0 {
				loops += int(n)
				if _, e = s.Pool.Exec(ctx, `UPDATE discussions SET loop_count=$2,version=version+1 WHERE id=$1`, id, loops); e != nil {
					return e
				}
			}
			// Retain checkpoints; extend technical budgets as a separate audited continuation.
			if _, e = s.Pool.Exec(ctx, `UPDATE role_runs SET state='pending',budget_model_calls=0,budget_tokens=0,budget_epoch=budget_epoch+1,budget_started_at=now(),fence=fence+1,lease_owner=NULL,lease_until=NULL,updated_at=now() WHERE discussion_id=$1 AND state IN('pending','running','paused')`, id); e != nil {
				return e
			}
			if _, e = s.Pool.Exec(ctx, `UPDATE background_jobs SET state='queued',attempt=0,run_after=now(),lease_owner=NULL,lease_until=NULL,fencing_token=fencing_token+1 WHERE kind='role.delivery' AND unique_key IN(SELECT id::text FROM deliveries WHERE discussion_id=$1 AND state IN('pending','running')) AND state<>'running'`, id); e != nil {
				return e
			}
			if loops >= allowance {
				if e = s.pauseDiscussion(ctx, group, id, "loop_limit"); e != nil {
					return e
				}
			}
		} else {
			if _, e = s.Pool.Exec(ctx, `UPDATE discussions SET state='terminated',ended_at=now(),version=version+1 WHERE id=$1`, id); e != nil {
				return e
			}
			if _, e = s.Pool.Exec(ctx, `UPDATE deliveries SET state='cancelled' WHERE discussion_id=$1 AND state IN('pending','running','waiting_human')`, id); e != nil {
				return e
			}
			if _, e = s.Pool.Exec(ctx, `UPDATE role_runs SET state='cancelled',fence=fence+1,updated_at=now() WHERE discussion_id=$1 AND state IN('pending','running','paused')`, id); e != nil {
				return e
			}
		}
		if _, e = s.Pool.Exec(ctx, `INSERT INTO discussion_controls(id,discussion_id,user_id,action,feedback,allowance,loop_count) VALUES($1,$2,$3,$4,$5,$6,$7)`, uuid.New(), id, p.UserID, action, feedback, allowance, loops); e != nil {
			return e
		}
		body := "AI discussion " + action
		if feedback != "" {
			body += "\n" + feedback
		}
		if _, e = s.appendMessage(ctx, group, p.UserID, "system", MessageInput{Body: body}, nil); e != nil {
			return e
		}
		return s.event(ctx, group, "discussion.changed", map[string]any{"discussionId": id, "action": action})
	})
}
func (s *Service) Settle(ctx context.Context, group uuid.UUID) error {
	return s.write(ctx, func(ctx context.Context) error {
		if e := s.lockConversation(ctx, group); e != nil {
			return e
		}
		tag, e := s.Pool.Exec(ctx, `UPDATE discussions d SET state='settled',ended_at=now(),version=version+1 WHERE d.group_id=$1 AND d.state IN('active','paused') AND NOT EXISTS(SELECT 1 FROM deliveries x WHERE x.discussion_id=d.id AND x.state IN('pending','running','waiting_human'))`, group)
		if e != nil || tag.RowsAffected() == 0 {
			return e
		}
		return s.event(ctx, group, "discussion.settled", map[string]any{})
	})
}
func (s *Service) Runs(ctx context.Context, group uuid.UUID) ([]json.RawMessage, error) {
	if _, e := s.Access(ctx, group); e != nil {
		return nil, e
	}
	return s.list(ctx, `SELECT run.id,run.fence AS version,run.role_agent_id AS "roleAgentId",run.discussion_id AS "discussionId",run.state,run.snapshot_version AS "snapshotVersion",run.model_calls AS "modelCalls",run.tokens,run.error_code AS "errorCode",run.error_message AS "errorMessage",run.created_at AS "createdAt",run.updated_at AS "updatedAt" FROM role_runs run JOIN role_agents r ON r.id=run.role_agent_id WHERE r.group_id=$1 ORDER BY run.created_at DESC LIMIT 100`, group)
}

// Runtime executes as the owning human through an internal principal. Network callers
// cannot construct this publication: every result must still match a live run fence.
type Publication struct {
	GroupID, UserID, RoleID, RunID, DiscussionID, DeliveryID uuid.UUID
	Fence                                                    int64
	Path                                                     []uuid.UUID
}

func (s *Service) Publish(ctx context.Context, p Publication, in MessageInput) (uuid.UUID, error) {
	ctx = auth.WithPrincipal(ctx, &auth.Principal{Kind: auth.KindWeb, UserID: p.UserID})
	var id uuid.UUID
	err := s.write(ctx, func(ctx context.Context) error {
		if _, e := s.Access(ctx, p.GroupID); e != nil {
			return e
		}
		var e error
		id, e = s.appendMessage(ctx, p.GroupID, p.UserID, "agent", in, &publication{RoleID: p.RoleID, RunID: p.RunID, DiscussionID: p.DiscussionID, DeliveryID: p.DeliveryID, Fence: p.Fence, Path: p.Path})
		return e
	})
	return id, err
}
func (s *Service) Pause(ctx context.Context, group, discussion uuid.UUID, reason string) error {
	return s.write(ctx, func(ctx context.Context) error {
		if e := s.lockConversation(ctx, group); e != nil {
			return e
		}
		return s.pauseDiscussion(ctx, group, discussion, reason)
	})
}
func (s *Service) RuntimeClock() time.Time { return time.Now().UTC() }

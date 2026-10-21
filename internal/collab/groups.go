package collab

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"sort"
)

// The parent projection is permission filtered, including its identifier.
const groupColumns = `g.id,g.project_id AS "projectId",g.name,g.purpose,g.archived,g.version,g.workflow_ids AS "workflowIds",
 CASE WHEN EXISTS(SELECT 1 FROM conversation_members a WHERE a.conversation_id=g.parent_id AND a.user_id=$2 AND a.active) THEN g.parent_id ELSE NULL END AS "parentId",
 g.depth,c.message_seq AS "messageSeq",c.event_seq AS "eventSeq",GREATEST(0,c.message_seq-cm.read_seq) AS unread,
 (SELECT body FROM messages WHERE conversation_id=g.id ORDER BY seq DESC LIMIT 1) AS preview,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('userId',m.user_id,'displayName',u.display_name,'avatarVersion',CASE WHEN u.avatar_png IS NULL THEN 0 ELSE u.avatar_version END,'version',m.version,'roles',
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',r.id,'positionId',r.position_id,'name',p.name,'version',r.version)) FROM role_agents r JOIN positions p ON p.id=r.position_id WHERE r.group_id=g.id AND r.user_id=m.user_id AND r.active),'[]'::jsonb)) ORDER BY m.joined_at,m.user_id)
 FROM conversation_members m JOIN users u ON u.id=m.user_id WHERE m.conversation_id=g.id AND m.active),'[]'::jsonb) AS members`

func (s *Service) Groups(ctx context.Context, project uuid.UUID) ([]json.RawMessage, error) {
	if err := s.projectAccess(ctx, project, false); err != nil {
		return nil, err
	}
	p, _ := actor(ctx)
	return s.list(ctx, `SELECT `+groupColumns+` FROM groups g JOIN conversations c ON c.id=g.id JOIN conversation_members cm ON cm.conversation_id=g.id AND cm.user_id=$2 AND cm.active WHERE g.project_id=$1 ORDER BY g.created_at`, project, p.UserID)
}
func (s *Service) Group(ctx context.Context, id uuid.UUID) (json.RawMessage, error) {
	access, err := s.Access(ctx, id)
	if err != nil {
		return nil, err
	}
	if access.Kind != "group" {
		return nil, missing()
	}
	p, _ := actor(ctx)
	return s.one(ctx, `SELECT `+groupColumns+` FROM groups g JOIN conversations c ON c.id=g.id JOIN conversation_members cm ON cm.conversation_id=g.id AND cm.user_id=$2 WHERE g.id=$1`, id, p.UserID)
}
func (s *Service) SaveGroup(ctx context.Context, project, id uuid.UUID, in GroupInput) (json.RawMessage, error) {
	if err := s.requireHuman(ctx); err != nil {
		return nil, err
	}
	if err := validText(in.Name, "name", 120, true); err != nil {
		return nil, err
	}
	if err := validText(in.Purpose, "purpose", 16000, false); err != nil {
		return nil, err
	}
	err := s.write(ctx, func(ctx context.Context) error {
		if err := s.projectAccess(ctx, project, false); err != nil {
			return err
		}
		p, _ := actor(ctx)
		if _, err := s.Pool.Exec(ctx, `SELECT id FROM projects WHERE id=$1 FOR UPDATE`, project); err != nil {
			return err
		}
		for _, w := range in.WorkflowIDs {
			var valid bool
			if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workflows WHERE id=$1 AND project_id=$2)`, w, project).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return apierrors.Fields("workflowIds", "not in project")
			}
		}
		creating := id == uuid.Nil
		if creating {
			depth := 1
			if in.ParentID != nil {
				a, e := s.Access(ctx, *in.ParentID)
				if e != nil {
					return e
				}
				if a.ProjectID == nil || *a.ProjectID != project {
					return missing()
				}
				if e = s.Pool.QueryRow(ctx, `SELECT depth+1 FROM groups WHERE id=$1`, in.ParentID).Scan(&depth); e != nil {
					return e
				}
				if depth > 3 {
					return apierrors.Fields("parentId", "maximum depth is three")
				}
			}
			id = uuid.New()
			if _, e := s.Pool.Exec(ctx, `INSERT INTO conversations(id,kind) VALUES($1,'group')`, id); e != nil {
				return e
			}
			if _, e := s.Pool.Exec(ctx, `INSERT INTO groups(id,project_id,parent_id,depth,name,purpose,workflow_ids,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, project, in.ParentID, depth, in.Name, in.Purpose, nonnil(in.WorkflowIDs), p.UserID); e != nil {
				return e
			}
			if _, e := s.Pool.Exec(ctx, `INSERT INTO context_heads(group_id) VALUES($1)`, id); e != nil {
				return e
			}
		} else {
			a, e := s.Access(ctx, id)
			if e != nil {
				return e
			}
			if a.ProjectID == nil || *a.ProjectID != project {
				return missing()
			}
			var parent *uuid.UUID
			if e = s.Pool.QueryRow(ctx, `SELECT parent_id FROM groups WHERE id=$1`, id).Scan(&parent); e != nil {
				return e
			}
			// parentId is a creation-only field; hidden ancestors cannot be guessed in writes.
			if in.ParentID != nil {
				return apierrors.Fields("parentId", "immutable after creation")
			}
			tag, e := s.Pool.Exec(ctx, `UPDATE groups SET name=$2,purpose=$3,workflow_ids=$4,version=version+1 WHERE id=$1 AND version=$5`, id, in.Name, in.Purpose, nonnil(in.WorkflowIDs), in.ExpectedVersion)
			if e != nil {
				return e
			}
			if tag.RowsAffected() == 0 {
				return conflict()
			}
		}
		if e := s.lockConversation(ctx, id); e != nil {
			return e
		}
		if in.Members != nil || creating {
			if e := s.setGroupMembers(ctx, project, id, in.Members, p.UserID); e != nil {
				return e
			}
		}
		return s.event(ctx, id, "group.changed", map[string]any{"groupId": id})
	})
	if err != nil {
		return nil, err
	}
	return s.Group(ctx, id)
}
func (s *Service) setGroupMembers(ctx context.Context, project, id uuid.UUID, members []MemberRoles, self uuid.UUID) error {
	sort.Slice(members, func(i, j int) bool { return members[i].UserID.String() < members[j].UserID.String() })
	seen := map[uuid.UUID]bool{}
	for _, m := range members {
		if seen[m.UserID] {
			return apierrors.Fields("members", "duplicate")
		}
		seen[m.UserID] = true
	}
	if !seen[self] {
		return apierrors.Fields("members", "current member must remain")
	}
	if len(seen) < 2 {
		return apierrors.Fields("members", "at least two members required")
	}
	old := []uuid.UUID{}
	rows, e := s.Pool.Query(ctx, `SELECT user_id FROM conversation_members WHERE conversation_id=$1 AND active ORDER BY user_id`, id)
	if e != nil {
		return e
	}
	for rows.Next() {
		var user uuid.UUID
		if e = rows.Scan(&user); e != nil {
			rows.Close()
			return e
		}
		old = append(old, user)
	}
	rows.Close()
	for _, user := range old {
		if !seen[user] {
			if _, e = s.Pool.Exec(ctx, `UPDATE conversation_members SET active=false,version=version+1 WHERE conversation_id=$1 AND user_id=$2`, id, user); e != nil {
				return e
			}
			if e = s.revokeRoles(ctx, id, user); e != nil {
				return e
			}
			if e = s.userEvent(ctx, user, nil, "access.changed", map[string]any{}); e != nil {
				return e
			}
		}
	}
	for _, m := range members {
		var valid bool
		if e = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM project_members m JOIN users u ON u.id=m.user_id WHERE project_id=$1 AND user_id=$2 AND m.state='active' AND u.status='active')`, project, m.UserID).Scan(&valid); e != nil {
			return e
		}
		if !valid {
			return apierrors.Fields("members", "active project member required")
		}
		if _, e = s.Pool.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id) VALUES($1,$2) ON CONFLICT(conversation_id,user_id) DO UPDATE SET active=true,version=conversation_members.version+1`, id, m.UserID); e != nil {
			return e
		}
		if _, e = s.Pool.Exec(ctx, `UPDATE role_agents SET active=false,version=version+1 WHERE group_id=$1 AND user_id=$2 AND NOT(position_id=ANY($3::uuid[]))`, id, m.UserID, nonnil(m.PositionIDs)); e != nil {
			return e
		}
		for _, position := range m.PositionIDs {
			if e = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM positions WHERE id=$1 AND project_id=$2)`, position, project).Scan(&valid); e != nil {
				return e
			}
			if !valid {
				return apierrors.Fields("positionIds", "not in project")
			}
			if _, e = s.Pool.Exec(ctx, `INSERT INTO role_agents(id,group_id,user_id,position_id) VALUES($1,$2,$3,$4) ON CONFLICT(group_id,user_id,position_id) DO UPDATE SET active=true,version=role_agents.version+1`, uuid.New(), id, m.UserID, position); e != nil {
				return e
			}
		}
		already := false
		for _, user := range old {
			if user == m.UserID {
				already = true
			}
		}
		if !already {
			if e = s.notice(ctx, m.UserID, &id, &project, "group.added", map[string]any{"groupId": id}, "group-add:"+id.String()+":"+uuid.NewString()); e != nil {
				return e
			}
		}
	}
	return s.cancelInactiveRoles(ctx, id)
}
func (s *Service) revokeRoles(ctx context.Context, group, user uuid.UUID) error {
	if e := s.lockConversation(ctx, group); e != nil {
		return e
	}
	if _, e := s.Pool.Exec(ctx, `UPDATE role_agents SET active=false,version=version+1 WHERE group_id=$1 AND user_id=$2`, group, user); e != nil {
		return e
	}
	if e := s.cancelInactiveRoles(ctx, group); e != nil {
		return e
	}
	if _, e := s.Pool.Exec(ctx, `UPDATE human_waits h SET resolved=true FROM deliveries d JOIN discussions dc ON dc.id=d.discussion_id WHERE h.delivery_id=d.id AND dc.group_id=$1 AND h.user_id=$2`, group, user); e != nil {
		return e
	}
	if _, e := s.Pool.Exec(ctx, `UPDATE deliveries d SET state='done' FROM discussions dc WHERE d.discussion_id=dc.id AND dc.group_id=$1 AND d.state='waiting_human' AND NOT EXISTS(SELECT 1 FROM human_waits h WHERE h.delivery_id=d.id AND NOT h.resolved)`, group); e != nil {
		return e
	}
	return s.Settle(ctx, group)
}

func (s *Service) cancelInactiveRoles(ctx context.Context, group uuid.UUID) error {
	if _, e := s.Pool.Exec(ctx, `UPDATE deliveries d SET state='cancelled' FROM role_agents r WHERE d.role_agent_id=r.id AND r.group_id=$1 AND NOT r.active AND d.state IN ('pending','running','waiting_human')`, group); e != nil {
		return e
	}
	if _, e := s.Pool.Exec(ctx, `UPDATE role_runs run SET state='cancelled',fence=fence+1,updated_at=now() FROM role_agents r WHERE run.role_agent_id=r.id AND r.group_id=$1 AND NOT r.active AND run.state IN ('pending','running','paused')`, group); e != nil {
		return e
	}
	return s.Settle(ctx, group)
}
func (s *Service) Archive(ctx context.Context, id uuid.UUID, version int64, archived bool) error {
	if e := s.requireHuman(ctx); e != nil {
		return e
	}
	return s.write(ctx, func(ctx context.Context) error {
		a, e := s.Access(ctx, id)
		if e != nil {
			return e
		}
		if a.ProjectID == nil {
			return missing()
		}
		if e = s.projectAccess(ctx, *a.ProjectID, true); e != nil {
			return e
		}
		targets, e := s.Pool.Query(ctx, `WITH RECURSIVE subtree AS(SELECT id FROM groups WHERE id=$1 UNION ALL SELECT g.id FROM groups g JOIN subtree p ON g.parent_id=p.id) SELECT id FROM subtree ORDER BY id`, id)
		if e != nil {
			return e
		}
		locks := []uuid.UUID{}
		for targets.Next() {
			var target uuid.UUID
			if e = targets.Scan(&target); e != nil {
				targets.Close()
				return e
			}
			locks = append(locks, target)
		}
		targets.Close()
		if e = s.lockConversations(ctx, locks); e != nil {
			return e
		}

		var current int64
		if e = s.Pool.QueryRow(ctx, `SELECT version FROM groups WHERE id=$1 FOR UPDATE`, id).Scan(&current); e != nil {
			return e
		}
		if current != version {
			return conflict()
		}
		rows, e := s.Pool.Query(ctx, `WITH RECURSIVE subtree AS(SELECT id FROM groups WHERE id=$1 UNION ALL SELECT g.id FROM groups g JOIN subtree p ON g.parent_id=p.id) UPDATE groups SET archived=$2,version=version+1 WHERE id IN(SELECT id FROM subtree) RETURNING id`, id, archived)
		if e != nil {
			return e
		}
		ids := []uuid.UUID{}
		for rows.Next() {
			var target uuid.UUID
			if e = rows.Scan(&target); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, target)
		}
		rows.Close()
		for _, target := range ids {
			if e = s.event(ctx, target, "group.archived", map[string]any{"archived": archived}); e != nil {
				return e
			}
		}
		return nil
	})
}

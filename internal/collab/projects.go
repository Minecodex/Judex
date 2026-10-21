package collab

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/workflow"
)

const projectColumns = `p.id,p.name,p.description,p.owner_id AS "ownerId",p.default_model_id AS "defaultModelId",p.loop_limit AS "loopLimit",p.version,p.created_at AS "createdAt",m.role,
 (SELECT count(*) FROM project_members pm WHERE pm.project_id=p.id AND pm.state='active') AS "memberCount"`

func (s *Service) Projects(ctx context.Context) ([]json.RawMessage, error) {
	p, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	// An empty device grant can discover projects before narrowing its scope.
	// Once narrowed, even the project metadata must stay within that scope.
	discovery := p.Kind != auth.KindCLI || len(p.ProjectScope) == 0
	return s.list(ctx, `SELECT `+projectColumns+` FROM projects p JOIN project_members m ON m.project_id=p.id WHERE m.user_id=$1 AND m.state='active' AND ($2::boolean OR p.id=ANY($3::uuid[])) ORDER BY p.created_at DESC`, p.UserID, discovery, nonnil(p.ProjectScope))
}
func (s *Service) Project(ctx context.Context, id uuid.UUID) (json.RawMessage, error) {
	if err := s.projectAccess(ctx, id, false); err != nil {
		return nil, err
	}
	p, _ := actor(ctx)
	return s.one(ctx, `SELECT `+projectColumns+` FROM projects p JOIN project_members m ON m.project_id=p.id WHERE p.id=$1 AND m.user_id=$2`, id, p.UserID)
}
func (s *Service) SaveProject(ctx context.Context, id uuid.UUID, in ProjectInput) (json.RawMessage, error) {
	if err := s.requireHuman(ctx); err != nil {
		return nil, err
	}
	if err := validText(in.Name, "name", 120, true); err != nil {
		return nil, err
	}
	if err := validText(in.Description, "description", 32000, false); err != nil {
		return nil, err
	}
	if in.LoopLimit == 0 {
		in.LoopLimit = 10
	}
	if in.LoopLimit < 1 || in.LoopLimit > 100 {
		return nil, apierrors.Fields("loopLimit", "range")
	}
	err := s.write(ctx, func(ctx context.Context) error {
		p, _ := actor(ctx)
		if in.DefaultModelID != nil {
			var enabled bool
			if e := s.Pool.QueryRow(ctx, `SELECT enabled FROM model_catalog WHERE id=$1`, in.DefaultModelID).Scan(&enabled); e != nil || !enabled {
				return apierrors.Fields("defaultModelId", "unavailable")
			}
		}
		if id == uuid.Nil {
			id = uuid.New()
			if _, e := s.Pool.Exec(ctx, `INSERT INTO projects(id,name,description,owner_id,default_model_id,loop_limit) VALUES($1,$2,$3,$4,$5,$6)`, id, in.Name, in.Description, p.UserID, in.DefaultModelID, in.LoopLimit); e != nil {
				return e
			}
			if _, e := s.Pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role) VALUES($1,$2,'owner')`, id, p.UserID); e != nil {
				return e
			}
			return s.projectEvent(ctx, id, "project", id)
		}
		if e := s.projectAccess(ctx, id, true); e != nil {
			return e
		}
		tag, e := s.Pool.Exec(ctx, `UPDATE projects SET name=$2,description=$3,default_model_id=$4,loop_limit=$5,version=version+1,updated_at=now() WHERE id=$1 AND version=$6`, id, in.Name, in.Description, in.DefaultModelID, in.LoopLimit, in.ExpectedVersion)
		if e == nil && tag.RowsAffected() == 0 {
			return conflict()
		}
		if e != nil {
			return e
		}
		return s.projectEvent(ctx, id, "project", id)
	})
	if err != nil {
		return nil, err
	}
	return s.Project(ctx, id)
}
func (s *Service) SearchUsers(ctx context.Context, term string) ([]json.RawMessage, error) {
	if _, err := actor(ctx); err != nil {
		return nil, err
	}
	term = strings.TrimSpace(term)
	if len(term) < 1 {
		return []json.RawMessage{}, nil
	}
	return s.list(ctx, `SELECT id,display_name AS "displayName",CASE WHEN avatar_png IS NULL THEN 0 ELSE avatar_version END AS "avatarVersion",email_display AS email FROM users WHERE status='active' AND (display_name ILIKE $1 OR email_normalized ILIKE $1) ORDER BY display_name,id LIMIT 50`, "%"+term+"%")
}
func (s *Service) Members(ctx context.Context, id uuid.UUID) ([]json.RawMessage, error) {
	if err := s.projectAccess(ctx, id, false); err != nil {
		return nil, err
	}
	return s.list(ctx, `SELECT m.user_id AS "userId",u.display_name AS "displayName",CASE WHEN u.avatar_png IS NULL THEN 0 ELSE u.avatar_version END AS "avatarVersion",u.email_display AS email,m.role,m.version FROM project_members m JOIN users u ON u.id=m.user_id WHERE m.project_id=$1 AND m.state='active' ORDER BY m.created_at,u.id`, id)
}

type ProjectMemberInput struct {
	UserID          uuid.UUID `json:"userId"`
	Role            string    `json:"role"`
	ExpectedVersion int64     `json:"expectedVersion"`
}

func (s *Service) SaveMember(ctx context.Context, project uuid.UUID, in ProjectMemberInput, remove bool) error {
	if err := s.requireHuman(ctx); err != nil {
		return err
	}
	return s.write(ctx, func(ctx context.Context) error {
		if err := s.projectAccess(ctx, project, true); err != nil {
			return err
		}
		var owner uuid.UUID
		if err := s.Pool.QueryRow(ctx, `SELECT owner_id FROM projects WHERE id=$1 FOR UPDATE`, project).Scan(&owner); err != nil {
			return err
		}
		if owner == in.UserID {
			return apierrors.Fields("userId", "owner cannot be removed or demoted")
		}
		if remove {
			groups, e := s.Pool.Query(ctx, `SELECT g.id FROM groups g JOIN conversation_members cm ON cm.conversation_id=g.id WHERE g.project_id=$1 AND cm.user_id=$2 AND cm.active ORDER BY g.id`, project, in.UserID)
			if e != nil {
				return e
			}
			locks := []uuid.UUID{}
			for groups.Next() {
				var id uuid.UUID
				if e = groups.Scan(&id); e != nil {
					groups.Close()
					return e
				}
				locks = append(locks, id)
			}
			groups.Close()
			if e = s.lockConversations(ctx, locks); e != nil {
				return e
			}

			tag, err := s.Pool.Exec(ctx, `UPDATE project_members SET state='removed',version=version+1 WHERE project_id=$1 AND user_id=$2 AND version=$3 AND state='active'`, project, in.UserID, in.ExpectedVersion)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return conflict()
			}
			rows, err := s.Pool.Query(ctx, `UPDATE conversation_members cm SET active=false,version=cm.version+1 FROM groups g WHERE cm.conversation_id=g.id AND g.project_id=$1 AND cm.user_id=$2 RETURNING cm.conversation_id`, project, in.UserID)
			if err != nil {
				return err
			}
			ids := []uuid.UUID{}
			for rows.Next() {
				var id uuid.UUID
				if err = rows.Scan(&id); err != nil {
					rows.Close()
					return err
				}
				ids = append(ids, id)
			}
			rows.Close()
			for _, id := range ids {
				if err = s.revokeRoles(ctx, id, in.UserID); err != nil {
					return err
				}
				if err = s.event(ctx, id, "membership.changed", map[string]any{"userId": in.UserID}); err != nil {
					return err
				}
			}
			if err = s.projectEvent(ctx, project, "members", in.UserID); err != nil {
				return err
			}
			return s.userEvent(ctx, in.UserID, nil, "access.changed", map[string]any{})
		}
		if in.Role != "admin" && in.Role != "member" {
			return apierrors.Fields("role", "invalid")
		}
		var valid bool
		if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')`, in.UserID).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return apierrors.Fields("userId", "active registered user required")
		}
		tag, err := s.Pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT(project_id,user_id) DO UPDATE SET role=$3,state='active',version=project_members.version+1 WHERE project_members.version=$4 OR (project_members.state='removed' AND $4=0)`, project, in.UserID, in.Role, in.ExpectedVersion)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return conflict()
		}
		if err = s.projectEvent(ctx, project, "members", in.UserID); err != nil {
			return err
		}
		return s.notice(ctx, in.UserID, nil, &project, "project.added", map[string]any{"projectId": project}, "project-add:"+project.String()+":"+uuid.NewString())
	})
}

type TemplateInput struct {
	Graph           *workflow.Graph `json:"graph,omitempty"`
	Name            string          `json:"name"`
	Prompt          string          `json:"prompt"`
	Content         string          `json:"content"`
	ModelID         *uuid.UUID      `json:"modelId"`
	ExpectedVersion int64           `json:"expectedVersion"`
}

func (s *Service) Templates(ctx context.Context, project uuid.UUID, kind string) ([]json.RawMessage, error) {
	if err := s.projectAccess(ctx, project, false); err != nil {
		return nil, err
	}
	if kind == "positions" {
		return s.list(ctx, `SELECT p.id,p.name,p.version,p.preset_id AS "presetId",p.current_version AS revision,v.prompt,v.model_id AS "modelId" FROM positions p JOIN position_versions v ON v.position_id=p.id AND v.revision=p.current_version WHERE p.project_id=$1 ORDER BY p.created_at`, project)
	}
	return s.list(ctx, `SELECT w.id,w.name,w.version,w.preset_id AS "presetId",w.current_version AS revision,v.content,v.graph FROM workflows w JOIN workflow_versions v ON v.workflow_id=w.id AND v.revision=w.current_version WHERE w.project_id=$1 ORDER BY w.created_at`, project)
}
func (s *Service) TemplateVersions(ctx context.Context, project, id uuid.UUID, kind string) ([]json.RawMessage, error) {
	if err := s.projectAccess(ctx, project, false); err != nil {
		return nil, err
	}
	if kind == "positions" {
		return s.list(ctx, `SELECT v.revision,v.prompt,v.model_id AS "modelId",v.created_at AS "createdAt" FROM position_versions v JOIN positions p ON p.id=v.position_id WHERE p.project_id=$1 AND p.id=$2 ORDER BY v.revision DESC`, project, id)
	}
	return s.list(ctx, `SELECT v.revision,v.content,v.graph,v.created_at AS "createdAt" FROM workflow_versions v JOIN workflows w ON w.id=v.workflow_id WHERE w.project_id=$1 AND w.id=$2 ORDER BY v.revision DESC`, project, id)
}
func (s *Service) SaveTemplate(ctx context.Context, project, id uuid.UUID, kind string, in TemplateInput) (uuid.UUID, error) {
	if e := s.requireHuman(ctx); e != nil {
		return id, e
	}
	if e := validText(in.Name, "name", 120, true); e != nil {
		return id, e
	}
	if len(in.Prompt)+len(in.Content) > 32000 {
		return id, apierrors.Fields("prompt", "length")
	}
	if in.Graph != nil {
		if e := in.Graph.Validate(); e != nil {
			return id, apierrors.Fields("graph", e.Error())
		}
	}
	err := s.write(ctx, func(ctx context.Context) error {
		if e := s.projectAccess(ctx, project, true); e != nil {
			return e
		}
		if kind == "positions" && in.ModelID != nil {
			var enabled bool
			if e := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM model_catalog WHERE id=$1 AND enabled)`, in.ModelID).Scan(&enabled); e != nil {
				return e
			}
			if !enabled {
				return apierrors.Fields("modelId", "enabled model required")
			}
		}
		table, col := "positions", "position_id"
		if kind == "workflows" {
			table, col = "workflows", "workflow_id"
		}
		revision := 1
		if id == uuid.Nil {
			id = uuid.New()
			if _, e := s.Pool.Exec(ctx, `INSERT INTO `+table+`(id,project_id,name) VALUES($1,$2,$3)`, id, project, in.Name); e != nil {
				return e
			}
		} else {
			e := s.Pool.QueryRow(ctx, `UPDATE `+table+` SET name=$3,current_version=current_version+1,version=version+1 WHERE project_id=$1 AND id=$2 AND version=$4 RETURNING current_version`, project, id, in.Name, in.ExpectedVersion).Scan(&revision)
			if e != nil {
				return conflict()
			}
		}
		if kind == "positions" {
			if _, e := s.Pool.Exec(ctx, `INSERT INTO position_versions(`+col+`,revision,prompt,model_id) VALUES($1,$2,$3,$4)`, id, revision, in.Prompt, in.ModelID); e != nil {
				return e
			}
		} else if _, e := s.Pool.Exec(ctx, `INSERT INTO workflow_versions(workflow_id,revision,content,graph) VALUES($1,$2,$3,COALESCE($4::jsonb,(SELECT graph FROM workflow_versions WHERE workflow_id=$1 AND revision=$2-1),'{"nodes":[],"edges":[]}'::jsonb))`, id, revision, in.Content, in.Graph); e != nil {
			return e
		}
		return s.projectEvent(ctx, project, kind, id)
	})
	return id, err
}
func (s *Service) PersonalPrompts(ctx context.Context, project uuid.UUID) ([]json.RawMessage, error) {
	if e := s.projectAccess(ctx, project, false); e != nil {
		return nil, e
	}
	p, _ := actor(ctx)
	return s.list(ctx, `SELECT position_id AS "positionId",prompt,version FROM personal_prompts WHERE project_id=$1 AND user_id=$2`, project, p.UserID)
}
func (s *Service) SavePrompt(ctx context.Context, project, position uuid.UUID, prompt string, version int64) error {
	return s.write(ctx, func(ctx context.Context) error {
		if e := s.projectAccess(ctx, project, false); e != nil {
			return e
		}
		if len(prompt) > 16000 {
			return apierrors.Fields("prompt", "length")
		}
		if version < 0 {
			return apierrors.Fields("expectedVersion", "non-negative")
		}
		p, _ := actor(ctx)
		var valid bool
		if e := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM positions WHERE project_id=$1 AND id=$2)`, project, position).Scan(&valid); e != nil {
			return e
		}
		if !valid {
			return missing()
		}
		var changed int64
		var e error
		if version == 0 {
			tag, err := s.Pool.Exec(ctx, `INSERT INTO personal_prompts(project_id,user_id,position_id,prompt) VALUES($1,$2,$3,$4) ON CONFLICT(project_id,user_id,position_id) DO NOTHING`, project, p.UserID, position, prompt)
			e, changed = err, tag.RowsAffected()
		} else {
			tag, err := s.Pool.Exec(ctx, `UPDATE personal_prompts SET prompt=$4,version=version+1 WHERE project_id=$1 AND user_id=$2 AND position_id=$3 AND version=$5`, project, p.UserID, position, prompt, version)
			e, changed = err, tag.RowsAffected()
		}
		if e == nil && changed == 0 {
			return conflict()
		}
		if e != nil {
			return e
		}
		return s.userEventProject(ctx, p.UserID, nil, &project, "personal-prompt.changed", map[string]any{"projectId": project, "positionId": position})
	})
}

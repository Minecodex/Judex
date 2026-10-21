package collab

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *Service) CheckOperation(ctx context.Context, op string, project, conversation, group uuid.UUID, raw []byte) error {
	manage := map[string]bool{"updateProject": true, "saveProjectMember": true, "removeProjectMember": true, "createPosition": true, "updatePosition": true, "createWorkflow": true, "updateWorkflow": true, "importTemplatePresets": true, "createWorkflowDraft": true, "getWorkflowDraft": true}
	if project != uuid.Nil {
		if e := s.projectAccess(ctx, project, manage[op]); e != nil {
			return e
		}
	}
	if conversation != uuid.Nil {
		if _, e := s.Access(ctx, conversation); e != nil {
			return e
		}
	}
	if group != uuid.Nil {
		a, e := s.Access(ctx, group)
		if e != nil {
			return e
		}
		if op == "archiveGroup" {
			if a.ProjectID == nil {
				return missing()
			}
			if e = s.projectAccess(ctx, *a.ProjectID, true); e != nil {
				return e
			}
		}
	}
	if op == "shareGroupContent" {
		var in ShareInput
		if e := json.Unmarshal(raw, &in); e != nil {
			return apierrors.Fields("body", "invalid JSON")
		}
		if _, e := s.Access(ctx, in.TargetGroupID); e != nil {
			return e
		}
	}
	if op == "postMessage" || op == "pushGroup" {
		var in MessageInput
		if e := json.Unmarshal(raw, &in); e != nil {
			return apierrors.Fields("body", "invalid JSON")
		}
		return authorizeMessage(ctx, in, op == "pushGroup")
	}
	return nil
}

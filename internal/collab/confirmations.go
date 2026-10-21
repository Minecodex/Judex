package collab

import (
	"context"
	"strings"

	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// A decision is an immutable human response, never an execution permission.
type Confirmation struct {
	Title string `json:"title"`
}
type Decision struct {
	Choice          string `json:"choice"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

func (s *Service) validateConfirmation(ctx context.Context, conversation, user uuid.UUID, kind string, in MessageInput, pub *publication) error {
	requested := false
	for _, mention := range in.Mentions {
		if mention.RequiresResponse {
			if mention.Kind != "human" {
				return apierrors.Fields("mentions.requiresResponse", "only human recipients can respond")
			}
			requested = true
		}
	}
	if in.Confirmation != nil {
		if kind != "group" || !requested {
			return apierrors.Fields("confirmation", "a group human response request is required")
		}
		if e := validText(in.Confirmation.Title, "confirmation.title", 240, true); e != nil {
			return e
		}
	}
	if in.Decision == nil {
		return nil
	}
	if pub != nil || kind != "group" || in.ReplyTo == nil || in.Confirmation != nil {
		return apierrors.Fields("decision", "must be a human reply to a group confirmation")
	}
	if in.Decision.ExpectedVersion != 1 {
		return conflict()
	}
	switch in.Decision.Choice {
	case "agree", "changes":
	case "comment":
		if strings.TrimSpace(in.Body) == "" {
			return apierrors.Fields("body", "feedback required")
		}
	default:
		return apierrors.Fields("decision.choice", "invalid choice")
	}
	var allowed, answered, closed bool
	e := s.Pool.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM message_mentions WHERE message_id=m.id AND kind='human' AND target_id=$3 AND requires_response),
 EXISTS(SELECT 1 FROM confirmation_responses WHERE request_message_id=m.id AND user_id=$3),
 COALESCE((SELECT state='terminated' FROM discussions WHERE id=m.discussion_id),false)
 FROM messages m WHERE m.id=$1 AND m.conversation_id=$2`, in.ReplyTo, conversation, user).Scan(&allowed, &answered, &closed)
	if e != nil || !allowed {
		return missing()
	}
	if answered || closed {
		return conflict()
	}
	return nil
}

func (s *Service) recordConfirmationResponse(ctx context.Context, user, message uuid.UUID, in MessageInput, pub *publication) error {
	if pub != nil || in.ReplyTo == nil {
		return nil
	}
	choice := "comment"
	if in.Decision != nil {
		choice = in.Decision.Choice
	}
	_, e := s.Pool.Exec(ctx, `INSERT INTO confirmation_responses(request_message_id,user_id,response_message_id,choice)
 SELECT $1,$2,$3,$4 WHERE EXISTS(SELECT 1 FROM message_mentions WHERE message_id=$1 AND kind='human' AND target_id=$2 AND requires_response)
 ON CONFLICT(request_message_id,user_id) DO NOTHING`, in.ReplyTo, user, message, choice)
	return e
}

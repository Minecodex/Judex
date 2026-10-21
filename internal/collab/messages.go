package collab

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

const messageColumns = `m.id,m.seq,m.body,m.author_user_id AS "authorUserId",u.display_name AS "authorName",CASE WHEN u.avatar_png IS NULL THEN 0 ELSE u.avatar_version END AS "avatarVersion",m.role_agent_id AS "roleAgentId",p.name AS "positionName",m.source,m.origin,m.reply_to AS "replyTo",m.discussion_id AS "discussionId",m.created_at AS "createdAt",
 CASE WHEN EXISTS(SELECT 1 FROM message_mentions WHERE message_id=m.id AND kind='human' AND requires_response) THEN jsonb_build_object('title',m.confirmation_title,'version',1,'closed',COALESCE((SELECT state='terminated' FROM discussions WHERE id=m.discussion_id),false),'responses',COALESCE((SELECT jsonb_agg(jsonb_build_object('userId',cr.user_id,'choice',cr.choice,'messageId',cr.response_message_id,'body',rm.body,'createdAt',cr.created_at)) FROM confirmation_responses cr JOIN messages rm ON rm.id=cr.response_message_id WHERE cr.request_message_id=m.id),'[]'::jsonb)) ELSE NULL END AS confirmation,
 (SELECT jsonb_build_object('choice',choice,'requestMessageId',request_message_id) FROM confirmation_responses WHERE response_message_id=m.id) AS decision,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('kind',kind,'targetId',target_id,'requiresResponse',requires_response)) FROM message_mentions WHERE message_id=m.id),'[]'::jsonb) AS mentions,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',v.id,'fileId',v.file_id,'revision',v.revision,'name',v.name,'mime',v.mime,'size',v.size)) FROM message_files mf JOIN file_versions v ON v.id=mf.version_id JOIN files f ON f.id=v.file_id WHERE mf.message_id=m.id),'[]'::jsonb) AS files`
const messageJoins = ` FROM messages m JOIN users u ON u.id=m.author_user_id LEFT JOIN role_agents r ON r.id=m.role_agent_id LEFT JOIN positions p ON p.id=r.position_id `

func (s *Service) Messages(ctx context.Context, id uuid.UUID, after, before int64, search string) ([]json.RawMessage, error) {
	if _, e := s.Access(ctx, id); e != nil {
		return nil, e
	}
	return s.list(ctx, `SELECT `+messageColumns+messageJoins+` WHERE m.conversation_id=$1 AND m.seq>$2 AND ($3::bigint=0 OR m.seq<$3) AND ($4='' OR m.body ILIKE '%'||$4||'%') ORDER BY m.seq DESC LIMIT 100`, id, after, before, search)
}
func (s *Service) Message(ctx context.Context, id, mid uuid.UUID) (json.RawMessage, error) {
	if _, e := s.Access(ctx, id); e != nil {
		return nil, e
	}
	return s.one(ctx, `SELECT `+messageColumns+messageJoins+` WHERE m.conversation_id=$1 AND m.id=$2`, id, mid)
}

type publication struct {
	RoleID, RunID, DiscussionID uuid.UUID
	Fence                       int64
	Path                        []uuid.UUID
	DeliveryID                  uuid.UUID
}

func (s *Service) Post(ctx context.Context, id uuid.UUID, in MessageInput, push bool) (json.RawMessage, error) {
	if err := authorizeMessage(ctx, in, push); err != nil {
		return nil, err
	}
	var mid uuid.UUID
	warnings := []string{}
	err := s.write(ctx, func(ctx context.Context) error {
		access, e := s.Access(ctx, id)
		if e != nil {
			return e
		}
		if push && access.Kind != "group" {
			return apierrors.Fields("conversationId", "push requires group")
		}
		p, _ := actor(ctx)
		source := "web"
		if p.Kind == auth.KindCLI {
			source = "cli"
		}
		if push && in.RecordOnly {
			in.suppressTriggers = true
		}
		mid, e = s.appendMessage(ctx, id, p.UserID, source, in, nil)
		if e != nil {
			return e
		}
		if push && !in.RecordOnly {
			rows, e := s.Pool.Query(ctx, `SELECT id FROM role_agents WHERE group_id=$1 AND user_id=$2 AND active ORDER BY id`, id, p.UserID)
			if e != nil {
				return e
			}
			ids := []uuid.UUID{}
			for rows.Next() {
				var rid uuid.UUID
				if e = rows.Scan(&rid); e != nil {
					rows.Close()
					return e
				}
				ids = append(ids, rid)
			}
			rows.Close()
			if len(ids) == 0 {
				warnings = append(warnings, "NO_OWN_ROLE: content saved; no AI was triggered")
			}
			for _, rid := range ids {
				if e = s.deliver(ctx, id, mid, rid, nil); e != nil {
					return e
				}
			}
		}
		if access.Kind == "group" {
			return s.Settle(ctx, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	raw, err := s.Message(ctx, id, mid)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"message": raw, "warnings": warnings})
}

// Check effects as well as endpoints. This also runs before idempotent replay;
// a response created by a broader grant cannot confer authority on a narrower one.
func authorizeMessage(ctx context.Context, in MessageInput, push bool) error {
	p, err := actor(ctx)
	if err != nil {
		return err
	}
	scope := auth.ScopeMessagesWrite
	if push {
		scope = auth.ScopeAgentRequest
	}
	if !p.HasScope(scope) {
		return apierrors.New(apierrors.Forbidden, "grant lacks scope: "+scope)
	}
	// recordOnly suppresses triggers only on local push, never on message send.
	if !push && !in.suppressTriggers {
		for _, mention := range in.Mentions {
			if mention.Kind == "role_agent" && !p.HasScope(auth.ScopeAgentRequest) {
				return apierrors.New(apierrors.Forbidden, "grant lacks scope: "+auth.ScopeAgentRequest)
			}
		}
	}
	return nil
}
func (s *Service) appendMessage(ctx context.Context, id, user uuid.UUID, source string, in MessageInput, pub *publication) (uuid.UUID, error) {
	if e := validText(in.Body, "body", 64<<10, len(in.FileVersionIDs) == 0); e != nil {
		return uuid.Nil, e
	}
	if len(in.Mentions) > 100 || len(in.FileVersionIDs) > 20 {
		return uuid.Nil, apierrors.Fields("mentions", "too many references")
	}
	if e := s.lockConversation(ctx, id); e != nil {
		return uuid.Nil, e
	}
	if pub != nil {
		if e := s.checkPublication(ctx, pub); e != nil {
			return uuid.Nil, e
		}
	}
	var kind string
	if e := s.Pool.QueryRow(ctx, `SELECT kind FROM conversations WHERE id=$1`, id).Scan(&kind); e != nil {
		return uuid.Nil, e
	}
	if kind == "direct" && len(in.Mentions) > 0 {
		return uuid.Nil, apierrors.Fields("mentions", "direct conversations are human only")
	}
	if e := s.validateConfirmation(ctx, id, user, kind, in, pub); e != nil {
		return uuid.Nil, e
	}
	if in.ReplyTo != nil {
		var valid bool
		if e := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE id=$1 AND conversation_id=$2)`, in.ReplyTo, id).Scan(&valid); e != nil {
			return uuid.Nil, e
		}
		if !valid {
			return uuid.Nil, apierrors.Fields("replyTo", "not in conversation")
		}
	}
	for _, m := range in.Mentions {
		var valid bool
		var e error
		if m.Kind == "human" {
			e = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND active)`, id, m.TargetID).Scan(&valid)
		} else if m.Kind == "role_agent" {
			e = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_agents r JOIN conversation_members cm ON cm.conversation_id=r.group_id AND cm.user_id=r.user_id AND cm.active WHERE r.group_id=$1 AND r.id=$2 AND r.active)`, id, m.TargetID).Scan(&valid)
		} else {
			return uuid.Nil, apierrors.Fields("mentions.kind", "invalid")
		}
		if e != nil {
			return uuid.Nil, e
		}
		if !valid {
			return uuid.Nil, apierrors.Fields("mentions.targetId", "not in conversation")
		}
	}
	for _, vid := range in.FileVersionIDs {
		if e := s.fileAccess(ctx, id, vid); e != nil {
			return uuid.Nil, e
		}
	}
	mid := uuid.New()
	var seq int64
	if e := s.Pool.QueryRow(ctx, `UPDATE conversations SET message_seq=message_seq+1 WHERE id=$1 RETURNING message_seq`, id).Scan(&seq); e != nil {
		return mid, e
	}
	var role, run, discussion *uuid.UUID
	if pub != nil {
		role = &pub.RoleID
		run = &pub.RunID
		discussion = &pub.DiscussionID
	}
	if _, e := s.Pool.Exec(ctx, `INSERT INTO messages(id,conversation_id,seq,author_user_id,role_agent_id,source,body,reply_to,discussion_id,run_id,origin) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, mid, id, seq, user, role, source, in.Body, in.ReplyTo, discussion, run, defaultJSON(in.origin)); e != nil {
		return mid, e
	}
	if in.Confirmation != nil {
		if _, e := s.Pool.Exec(ctx, `UPDATE messages SET confirmation_title=$2 WHERE id=$1`, mid, in.Confirmation.Title); e != nil {
			return mid, e
		}
	}
	if e := s.recordConfirmationResponse(ctx, user, mid, in, pub); e != nil {
		return mid, e
	}
	for _, vid := range in.FileVersionIDs {
		if _, e := s.Pool.Exec(ctx, `INSERT INTO message_files(message_id,version_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, mid, vid); e != nil {
			return mid, e
		}
	}
	for _, m := range in.Mentions {
		if _, e := s.Pool.Exec(ctx, `INSERT INTO message_mentions(message_id,kind,target_id,requires_response) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, mid, m.Kind, m.TargetID, m.RequiresResponse); e != nil {
			return mid, e
		}
		if m.Kind == "human" {
			if e := s.notice(ctx, m.TargetID, &id, nil, "message.mention", map[string]any{"conversationId": id, "messageId": mid}, "mention:"+mid.String()); e != nil {
				return mid, e
			}
			if pub != nil && m.RequiresResponse {
				if _, e := s.Pool.Exec(ctx, `INSERT INTO human_waits(delivery_id,message_id,user_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, pub.DeliveryID, mid, m.TargetID); e != nil {
					return mid, e
				}
			}
		} else if !in.suppressTriggers && (pub == nil || m.TargetID != pub.RoleID) {
			if e := s.deliver(ctx, id, mid, m.TargetID, pub); e != nil {
				return mid, e
			}
		}
	}
	// A human response releases only matching waits; waking a role still requires a structured mention.
	if pub == nil && in.ReplyTo != nil {
		if _, e := s.Pool.Exec(ctx, `UPDATE human_waits SET resolved=true WHERE message_id=$1 AND user_id=$2`, in.ReplyTo, user); e != nil {
			return mid, e
		}
		if _, e := s.Pool.Exec(ctx, `UPDATE deliveries d SET state='done' WHERE d.state='waiting_human' AND NOT EXISTS(SELECT 1 FROM human_waits h WHERE h.delivery_id=d.id AND NOT h.resolved) AND d.discussion_id IN(SELECT id FROM discussions WHERE group_id=$1)`, id); e != nil {
			return mid, e
		}
	}
	if kind == "group" {
		raw, e := s.one(ctx, `SELECT `+messageColumns+messageJoins+` WHERE m.id=$1`, mid)
		if e != nil {
			return mid, e
		}
		if e = s.knowledge(ctx, id, "message", mid.String(), raw); e != nil {
			return mid, e
		}
	}
	if e := s.event(ctx, id, "message.created", map[string]any{"messageId": mid, "seq": seq}); e != nil {
		return mid, e
	}
	return mid, nil
}
func (s *Service) Read(ctx context.Context, id uuid.UUID, seq int64) error {
	return s.write(ctx, func(ctx context.Context) error {
		if _, e := s.Access(ctx, id); e != nil {
			return e
		}
		p, _ := actor(ctx)
		var max int64
		if e := s.Pool.QueryRow(ctx, `SELECT message_seq FROM conversations WHERE id=$1`, id).Scan(&max); e != nil {
			return e
		}
		if seq < 0 || seq > max {
			return apierrors.Fields("seq", "range")
		}
		_, e := s.Pool.Exec(ctx, `UPDATE conversation_members SET read_seq=GREATEST(read_seq,$3) WHERE conversation_id=$1 AND user_id=$2`, id, p.UserID, seq)
		if e != nil {
			return e
		}
		return s.userEvent(ctx, p.UserID, &id, "conversation.read", map[string]any{"conversationId": id, "seq": seq})
	})
}
func (s *Service) Direct(ctx context.Context, other uuid.UUID) (json.RawMessage, error) {
	if e := s.requireHuman(ctx); e != nil {
		return nil, e
	}
	p, _ := actor(ctx)
	if p.UserID == other {
		return nil, apierrors.Fields("userId", "different user required")
	}
	ids := []string{p.UserID.String(), other.String()}
	sort.Strings(ids)
	var id uuid.UUID
	err := s.write(ctx, func(ctx context.Context) error {
		if _, e := s.Pool.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, ids[0]+":"+ids[1]); e != nil {
			return e
		}
		var valid bool
		if e := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')`, other).Scan(&valid); e != nil {
			return e
		}
		if !valid {
			return missing()
		}
		e := s.Pool.QueryRow(ctx, `SELECT conversation_id FROM direct_pairs WHERE low_user_id=$1 AND high_user_id=$2`, ids[0], ids[1]).Scan(&id)
		if e == nil {
			return nil
		}
		id = uuid.New()
		if _, e = s.Pool.Exec(ctx, `INSERT INTO conversations(id,kind) VALUES($1,'direct')`, id); e != nil {
			return e
		}
		if _, e = s.Pool.Exec(ctx, `INSERT INTO direct_pairs(low_user_id,high_user_id,conversation_id) VALUES($1,$2,$3)`, ids[0], ids[1], id); e != nil {
			return e
		}
		_, e = s.Pool.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id) VALUES($1,$2),($1,$3)`, id, p.UserID, other)
		return e
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"id": id})
}
func (s *Service) Directs(ctx context.Context) ([]json.RawMessage, error) {
	p, e := actor(ctx)
	if e != nil {
		return nil, e
	}
	return s.list(ctx, `SELECT c.id,u.id AS "userId",u.display_name AS "displayName",CASE WHEN u.avatar_png IS NULL THEN 0 ELSE u.avatar_version END AS "avatarVersion",c.message_seq AS "messageSeq",GREATEST(0,c.message_seq-cm.read_seq) AS unread,
 (SELECT body FROM messages WHERE conversation_id=c.id ORDER BY seq DESC LIMIT 1) AS preview FROM conversations c JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$1 AND cm.active JOIN conversation_members peer ON peer.conversation_id=c.id AND peer.user_id<>$1 JOIN users u ON u.id=peer.user_id WHERE c.kind='direct' ORDER BY c.created_at DESC`, p.UserID)
}

func defaultJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return value
}

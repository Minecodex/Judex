package collab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type Snapshot struct {
	Version         int64             `json:"version"`
	SourceSeq       int64             `json:"sourceSeq"`
	CoveredThrough  int64             `json:"coveredThrough"`
	Summary         string            `json:"summary"`
	Entries         []json.RawMessage `json:"entries"`
	EstimatedTokens int64             `json:"estimatedTokens"`
}
type Summarize func(context.Context, string, int64) (string, error)

// Tokens deliberately overestimates ordinary text, and is shared by every role.
func Tokens(value string) int64 { return int64((len([]byte(value)) + 1) / 2) }
func (s *Service) ContextBudget(ctx context.Context, group uuid.UUID) (int64, error) {
	var budget int64
	e := s.Pool.QueryRow(ctx, `WITH effective AS (
 SELECT COALESCE(pv.model_id,p.default_model_id) AS model_id,
 octet_length(p.description)+octet_length(g.purpose)+octet_length(COALESCE(pv.prompt,''))+octet_length(COALESCE(pp.prompt,'')) AS guidance
 FROM groups g JOIN projects p ON p.id=g.project_id LEFT JOIN role_agents r ON r.group_id=g.id AND r.active
 LEFT JOIN positions pos ON pos.id=r.position_id LEFT JOIN position_versions pv ON pv.position_id=pos.id AND pv.revision=pos.current_version
 LEFT JOIN personal_prompts pp ON pp.project_id=p.id AND pp.user_id=r.user_id AND pp.position_id=pos.id WHERE g.id=$1
 ), candidates AS (
 SELECT mc.id,mc.limits FROM effective e JOIN model_catalog mc ON mc.id=e.model_id AND mc.enabled
 UNION SELECT alternate.id,alternate.limits FROM effective e JOIN model_catalog mc ON mc.id=e.model_id AND mc.enabled
 CROSS JOIN LATERAL jsonb_array_elements_text(mc.quota_fallback_ids) fallback(id)
 JOIN model_catalog alternate ON alternate.id::text=fallback.id AND alternate.enabled
 ) SELECT GREATEST(1024,COALESCE((SELECT min((limits->>'maxInputTokens')::bigint-COALESCE((limits->>'maxOutputTokens')::bigint,8192)) FROM candidates),119808)-COALESCE((SELECT max(guidance) FROM effective),0)-8192)`, group).Scan(&budget)
	return budget, e
}

// Snapshot is shared infrastructure, never a coordinating role. Its lease is in
// PostgreSQL; no database transaction/connection is held during model I/O.
func (s *Service) Snapshot(ctx context.Context, group uuid.UUID, summarize Summarize) (Snapshot, error) {
	result := Snapshot{}
	if a, e := s.Access(ctx, group); e != nil {
		return result, e
	} else if a.Kind != "group" {
		return result, missing()
	}
	budget, e := s.ContextBudget(ctx, group)
	if e != nil {
		return result, e
	}
	owner := uuid.NewString()
	var source, headVersion, covered int64
	var summary string
	var fresh bool
	e = s.write(ctx, func(ctx context.Context) error {
		var compressOwner *string
		var until *time.Time
		if e := s.Pool.QueryRow(ctx, `SELECT h.version,h.covered_through,h.summary,h.compress_owner,h.compress_until,c.knowledge_seq FROM context_heads h JOIN conversations c ON c.id=h.group_id WHERE h.group_id=$1 FOR UPDATE OF h`, group).Scan(&headVersion, &covered, &summary, &compressOwner, &until, &source); e != nil {
			return e
		}
		e := s.Pool.QueryRow(ctx, `SELECT version,source_seq,covered_through,summary,entries,estimated_tokens FROM context_snapshots WHERE group_id=$1 AND version=$2`, group, headVersion).Scan(&result.Version, &result.SourceSeq, &result.CoveredThrough, &result.Summary, &result.Entries, &result.EstimatedTokens)
		if e == nil && result.SourceSeq == source && result.EstimatedTokens <= budget*7/10 {
			fresh = true
			return nil
		}
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if compressOwner != nil && until != nil && until.After(time.Now()) {
			return job.ErrBusy
		}
		_, e = s.Pool.Exec(ctx, `UPDATE context_heads SET compress_owner=$2,compress_until=now()+interval '5 minutes' WHERE group_id=$1`, group, owner)
		return e
	})
	if e != nil || fresh {
		return result, e
	}
	defer s.Pool.Exec(context.WithoutCancel(ctx), `UPDATE context_heads SET compress_owner=NULL,compress_until=NULL WHERE group_id=$1 AND compress_owner=$2`, group, owner)
	entries, e := s.list(ctx, `SELECT seq,kind,ref_id AS "refId",content FROM knowledge_entries WHERE group_id=$1 AND seq>$2 AND seq<=$3 ORDER BY seq`, group, covered, source)
	if e != nil {
		return result, e
	}
	tokenSize := func(summary string, entries []json.RawMessage) int64 {
		n := Tokens(summary)
		for _, entry := range entries {
			n += Tokens(string(entry))
		}
		return n
	}
	for i, raw := range entries {
		if Tokens(string(raw)) > budget/4 {
			var entry map[string]any
			if json.Unmarshal(raw, &entry) != nil {
				continue
			}
			if entry["kind"] == "file" || entry["kind"] == "tool" || entry["kind"] == "file_preview" {
				limit := int(max(int64(256), min(int64(8000), budget/8)))
				excerpt := []rune(string(raw))
				if len(excerpt) > limit {
					excerpt = excerpt[:limit]
				}
				entry["content"] = map[string]any{"indexExcerpt": string(excerpt), "originalBytes": len(raw), "readOriginal": map[string]any{"kind": entry["kind"], "refId": entry["refId"]}, "notice": "Index excerpt only; read_context provides the complete original in pages. Never treat omitted content as absent evidence."}
				entries[i], _ = json.Marshal(entry)
			}
		}
	}
	estimated := tokenSize(summary, entries)
	if estimated > budget*7/10 {
		if summarize == nil {
			return result, apierrors.New(apierrors.BudgetExhausted, "shared context requires compression before the next model call")
		}
		keepStart := len(entries)
		keptTokens := int64(0)
		for keepStart > 0 {
			n := Tokens(string(entries[keepStart-1]))
			if keptTokens+n > budget/5 {
				break
			}
			keepStart--
			keptTokens += n
		}
		if keepStart == 0 && len(entries) > 0 {
			keepStart = 1
			keptTokens -= Tokens(string(entries[0]))
		}
		old := entries[:keepStart]
		retained := entries[keepStart:]
		var input strings.Builder
		input.WriteString("Previous public summary:\n" + summary + "\nEarlier public entries (keep references, authors, unanswered questions, disagreements, mention relationships and file versions):\n")
		for _, entry := range old {
			input.Write(entry)
			input.WriteByte('\n')
		}
		condensed, err := summarize(ctx, input.String(), max(int64(128), budget/2-keptTokens))
		if err != nil || strings.TrimSpace(condensed) == "" {
			if err == nil {
				err = fmt.Errorf("empty summary")
			}
			_, _ = s.Pool.Exec(context.WithoutCancel(ctx), `UPDATE context_heads SET error_message='Shared context compression failed; original entries retained' WHERE group_id=$1`, group)
			return result, apierrors.New(apierrors.DependencyDown, "shared context compression failed; original entries retained").Wrap(err)
		}
		// An unbounded response is a failure, never silently truncated knowledge.
		if tokenSize(condensed, retained) > budget/2 {
			return result, apierrors.New(apierrors.BudgetExhausted, "summary did not fit the shared context budget")
		}
		// A configuration change can require recompressing only the previous
		// summary. Its coverage must remain intact even with no new entries.
		if len(old) > 0 {
			var last struct {
				Seq int64 `json:"seq"`
			}
			if e = json.Unmarshal(old[len(old)-1], &last); e != nil {
				return result, e
			}
			covered = last.Seq
		}
		summary = condensed
		entries = retained
		estimated = tokenSize(summary, entries)
	}
	result = Snapshot{Version: headVersion + 1, SourceSeq: source, CoveredThrough: covered, Summary: summary, Entries: entries, EstimatedTokens: estimated}
	e = s.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error {
		var current string
		if e := s.Pool.QueryRow(ctx, `SELECT COALESCE(compress_owner,'') FROM context_heads WHERE group_id=$1 FOR UPDATE`, group).Scan(&current); e != nil {
			return e
		}
		if current != owner {
			return job.ErrBusy
		}
		raw, _ := json.Marshal(entries)
		if _, e := s.Pool.Exec(ctx, `INSERT INTO context_snapshots(group_id,version,source_seq,covered_through,summary,entries,estimated_tokens) VALUES($1,$2,$3,$4,$5,$6,$7)`, group, result.Version, source, covered, summary, raw, estimated); e != nil {
			return e
		}
		_, e := s.Pool.Exec(ctx, `UPDATE context_heads SET version=$2,covered_through=$3,summary=$4,error_message=NULL WHERE group_id=$1`, group, result.Version, covered, summary)
		return e
	})
	return result, e
}

// Raw references stay readable even after compression, within the same boundary.
func (s *Service) ReadReference(ctx context.Context, group uuid.UUID, kind, ref string, offset, limit int) (json.RawMessage, error) {
	if _, e := s.Access(ctx, group); e != nil {
		return nil, e
	}
	if offset < 0 {
		return nil, apierrors.Fields("offset", "range")
	}
	if limit < 1 || limit > 16000 {
		limit = 8000
	}
	return s.one(ctx, `SELECT seq,kind,ref_id AS "refId",substring(content::text FROM $4+1 FOR $5) AS content,length(content::text) AS length FROM knowledge_entries WHERE group_id=$1 AND kind=$2 AND ref_id=$3`, group, kind, ref, offset, limit)
}

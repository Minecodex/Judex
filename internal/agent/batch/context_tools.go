package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/material"
	"strings"
)

func (e *Executor) contextTools(env *tools.Env, project uuid.UUID) {
	env.Publish = func(ctx context.Context, run string, args map[string]any, raw []byte) (map[string]any, error) {
		runID, err := uuid.Parse(run)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		var publication material.Publication
		if err = json.Unmarshal(encoded, &publication); err != nil {
			return nil, err
		}
		tx, err := e.beginEffect(ctx, project)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)
		service := material.NewService(nil, e.Objects, material.DefaultLimits(), e.Clock)
		version, err := service.PublishInTx(ctx, tx, project, runID, publication, raw)
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return map[string]any{"materialId": version.MaterialID, "materialVersionId": version.ID, "revision": version.Revision, "sha256": version.SHA256, "state": version.State, "entrypoint": version.Entrypoint}, nil
	}

	env.ReadContext = func(ctx context.Context, current, source, call string, offset, limit int) (map[string]any, error) {
		if offset < 0 || limit < 1 || limit > 16000 {
			return nil, fmt.Errorf("context range invalid")
		}
		var raw []byte
		err := e.Pool.QueryRow(ctx, `SELECT t.result_json FROM tool_calls t JOIN agent_runs old ON old.id=t.run_id JOIN agent_runs active ON active.id=$1 AND active.project_id=old.project_id AND active.session_id=old.session_id AND active.identity_id=old.identity_id AND active.binding_version IS NOT DISTINCT FROM old.binding_version JOIN agent_identities i ON i.id=active.identity_id WHERE old.id=$2 AND old.project_id=$3 AND t.tool_call_id=$4 AND t.state IN ('succeeded','failed') AND active.state='running' AND (i.kind='coordinator' OR i.current_binding_version=active.binding_version)`, current, source, project, call).Scan(&raw)
		if err != nil {
			return nil, fmt.Errorf("tool result unavailable to this session and binding")
		}
		chars := []rune(string(raw))
		if offset > len(chars) {
			return nil, fmt.Errorf("offset beyond result")
		}
		end := offset + limit
		if end > len(chars) {
			end = len(chars)
		}
		return map[string]any{"runId": source, "toolCallId": call, "offset": offset, "nextOffset": end, "total": len(chars), "hasMore": end < len(chars), "content": string(chars[offset:end])}, nil
	}
	env.RecordAnalysis = func(ctx context.Context, run string, args map[string]any) (map[string]any, error) {
		// Explicit public fields only; neither prompt nor debugging context is accepted.
		summary, _ := args["summary"].(string)
		scope, _ := args["applicability"].(string)
		refs, _ := args["sourceRefs"].([]any)
		if strings.TrimSpace(summary) == "" || len(refs) == 0 {
			return nil, fmt.Errorf("summary and sources required")
		}
		raw, err := json.Marshal(map[string]any{"runId": run, "refs": refs, "disagreements": args["disagreements"]})
		if err != nil {
			return nil, err
		}
		id := uuid.New()
		tx, err := e.beginEffect(ctx, project)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)
		tag, err := tx.Exec(ctx, `INSERT INTO knowledge_candidates(project_id,id,text,source_refs,state,applicability,created_at) SELECT $1,$2,$3,$4,'unverified',$5,now() WHERE EXISTS(SELECT 1 FROM agent_runs WHERE id=$6 AND project_id=$1 AND state='running')`, project, id, summary, raw, scope, run)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, fmt.Errorf("run not active")
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "state": "unverified", "sourceRefs": refs, "applicability": scope}, err
	}
	env.QueryKnowledge = func(ctx context.Context, query, cursor string, limit int) (map[string]any, error) {
		var after *uuid.UUID
		if cursor != "" {
			id, err := uuid.Parse(cursor)
			if err != nil {
				return nil, fmt.Errorf("invalid knowledge cursor")
			}
			after = &id
		}
		if limit < 1 || limit > 50 {
			limit = 20
		}
		rows, err := e.Pool.Query(ctx, `SELECT id,text,source_refs,state,applicability FROM knowledge_candidates WHERE project_id=$1 AND state<>'retracted' AND (strpos(lower(text),lower($2))>0 OR strpos(lower(applicability),lower($2))>0) AND ($3::uuid IS NULL OR id>$3) ORDER BY id LIMIT $4`, project, query, after, limit+1)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id uuid.UUID
			var text, state, scope string
			var raw []byte
			if err = rows.Scan(&id, &text, &raw, &state, &scope); err != nil {
				return nil, err
			}
			var refs any
			if err = json.Unmarshal(raw, &refs); err != nil {
				return nil, err
			}
			items = append(items, map[string]any{"id": id, "summary": text, "sourceRefs": refs, "state": state, "applicability": scope})
		}
		if err = rows.Err(); err != nil {
			return nil, err
		}
		var next any
		if len(items) > limit {
			items = items[:limit]
			next = items[len(items)-1]["id"]
		}
		return map[string]any{"items": items, "nextCursor": next}, nil
	}
}

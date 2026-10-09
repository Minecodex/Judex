package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"github.com/kakj-go/Judex/internal/work"
	"net/url"
	"strconv"
)

func (e *Executor) queryWorkPage(ctx context.Context, project string, args map[string]any) (map[string]any, error) {
	kind, _ := args["objectType"].(string)
	table := map[string]string{"task": "tasks", "plan": "plans", "proposal": "proposals"}[kind]
	if table == "" {
		return nil, fmt.Errorf("objectType unsupported")
	}
	cursor, _ := args["cursor"].(string)
	query, _ := args["query"].(string)
	rawID, _ := args["objectId"].(string)
	var id *uuid.UUID
	if rawID != "" {
		value, err := uuid.Parse(rawID)
		if err != nil {
			return nil, err
		}
		id = &value
	}
	limit := 20
	if n, ok := args["limit"].(float64); ok {
		limit = int(n)
	}
	ctx, err := paging.Parse(ctx, "tool:query_work:"+project+":"+kind, url.Values{"cursor": {cursor}, "limit": {strconv.Itoa(limit)}, "query": {query}, "objectId": {rawID}})
	if err != nil {
		return nil, err
	}
	label := "t.title"
	fields := `'id',t.id,'title',t.title,'status',t.status,'version',t.version`
	switch kind {
	case "task":
		fields += `,'discardedAt',t.discarded_at,'executionException',(SELECT jsonb_build_object('id',x.id,'previousStatus',x.previous_status,'reason',x.reason,'actorUserId',x.actor_user_id,'createdAt',x.created_at,'waivers',x.waivers_json) FROM task_execution_exceptions x WHERE x.project_id=t.project_id AND x.task_id=t.id AND x.restored_at IS NULL)`
		fields += `,'expectedOutput',t.expected_output,'acceptanceCriteria',t.acceptance_criteria,'planId',t.plan_id,'reviewerIdentityId',t.reviewer_identity_id,'workflowId',t.workflow_id,'nodeId',t.node_id,'latestReportId',t.latest_report_id,'latestAcceptanceId',t.latest_acceptance_id`
	case "plan":
		fields += `,'discardedAt',t.discarded_at`
		fields += `,'goal',t.goal,'acceptanceCriteria',t.acceptance_criteria,'ownerIdentityId',t.owner_identity_id,'workflowId',t.workflow_id`
	case "proposal":
		label = "t.kind"
		fields = `'id',t.id,'kind',t.kind,'status',t.status,'version',t.version,'reason',t.reason,'reviewId',t.current_review_id`
	}
	sql := `SELECT jsonb_build_object(` + fields + `) /*keys*/ FROM ` + table + ` t WHERE t.project_id=$1 AND ($2::uuid IS NULL OR t.id=$2) AND ($3='' OR strpos(lower(` + label + `),lower($3))>0) /*page*/`
	if kind == "task" || kind == "plan" {
		sql = `SELECT jsonb_build_object(` + fields + `) /*keys*/ FROM ` + table + ` t WHERE t.project_id=$1 AND ($2::uuid IS NULL OR t.id=$2) AND (t.discarded_at IS NULL OR $2::uuid IS NOT NULL) AND ($3='' OR strpos(lower(` + label + `),lower($3))>0) /*page*/`
	}
	rows, err := paging.Query(ctx, e.Pool, sql, "t.created_at", "t.id", project, id, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var value map[string]any
		if err = json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if kind == "task" {
		for _, item := range items {
			task, err := uuid.Parse(fmt.Sprint(item["id"]))
			if err != nil {
				return nil, err
			}
			facts, err := work.ReadTaskExecutionFacts(ctx, e.Pool, uuid.MustParse(project), task)
			if err != nil {
				return nil, err
			}
			item["effectiveRequirements"] = facts.Requirements
			item["blockers"] = facts.Blockers
		}
	}
	return map[string]any{"items": items, "nextCursor": paging.Next(ctx)}, nil
}

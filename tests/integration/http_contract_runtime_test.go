package integrationtest_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"time"
)

func (s *contractScenario) runtime(pool *postgres.Pool, initial map[string]any) {
	c, p, ctx := s.call, s.prefix, context.Background()
	runPath := p + "/runs/" + initial["runId"].(string)
	run := c("GET", runPath, nil)
	c("GET", p+"/events", nil)
	c("GET", runPath+"/events", nil)
	var version int64
	// Controlled runtime fixture for the stopped-batch HTTP contract. This does
	// not claim a model exhausted a real quota or completed a business analysis.
	if err := pool.QueryRow(ctx, `UPDATE discussion_batches SET state='waiting_human' WHERE id=$1 RETURNING version`, initial["batchId"]).Scan(&version); err != nil {
		s.t.Fatal(err)
	}
	c("POST", p+"/batches/"+initial["batchId"].(string)+"/extensions", map[string]any{"extraRounds": 1, "expectedVersion": version, "reason": "Controlled stopped batch schema"})
	c("POST", runPath+"/cancel", map[string]any{"expectedVersion": run["version"], "reason": "Cancel owned contract run"})
	activities := c("GET", s.taskPath+"/activity", nil)["items"].([]any)
	var analysis map[string]any
	for _, raw := range activities {
		if value, ok := raw.(map[string]any)["analysis"].(map[string]any); ok {
			analysis = value
			break
		}
	}
	if analysis == nil {
		s.t.Fatal("report analysis missing")
	}
	path := p + "/task-analyses/" + analysis["id"].(string)
	c("GET", path, nil)
	c("POST", path+"/retry", map[string]any{})
	// The deterministic structured-output path creates the discussion fixture
	// through the same validated recorder/finisher used by the model runner.
	project := uuid.MustParse(p[len("/projects/"):])
	batch := uuid.MustParse(analysis["batchId"].(string))
	if err := pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `UPDATE agent_runs SET state='running' WHERE batch_id=$1 AND parent_run_id IS NULL RETURNING id`, batch).Scan(&id); err != nil {
			return err
		}
		_, err := collaboration.RecordTaskOutput(ctx, tx.Tx, project, id, collaboration.TaskAnalysisOutput{Summary: "Controlled schema analysis records the delivery evidence and asks for independent review.", Basis: []string{}, Disagreements: []string{}, CurrentNodeCandidates: []collaboration.WorkflowNodeRef{}, RelatedEvidenceNodes: []collaboration.WorkflowNodeRef{}, SuggestedNextNodes: []collaboration.WorkflowNodeRef{}, Discussion: &collaboration.DiscussionProposal{Title: "Contract discussion", Reason: "Review available evidence"}}, time.Now())
		if err != nil {
			return err
		}
		return collaboration.FinishTaskAnalysis(ctx, tx.Tx, project, batch, "completed", "Controlled schema analysis", time.Now())
	}); err != nil {
		s.t.Fatal(err)
	}
	var suggestion uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM discussion_suggestions WHERE analysis_id=$1`, analysis["id"]).Scan(&suggestion); err != nil {
		s.t.Fatal(err)
	}
	suggestionPath := p + "/discussion-suggestions/" + suggestion.String()
	item := c("GET", suggestionPath, nil)
	c("POST", suggestionPath, map[string]any{"expectedVersion": item["version"], "mode": "dismiss"})
	activity := c("GET", s.taskPath+"/activity", nil)["items"].([]any)
	found := false
	for _, raw := range activity {
		row := raw.(map[string]any)
		if value, ok := row["analysis"].(map[string]any); ok && value["id"] == analysis["id"] {
			found = value["scopeKind"] == "task" && value["output"] != nil
		}
	}
	if !found {
		s.t.Fatal("activity lost scope or structured analysis output")
	}
	source := c("POST", p+"/submissions", map[string]any{"clientSubmissionId": uuid.NewString(), "purpose": "message", "text": "Project scoped contract analysis"})
	c("GET", p+"/submissions/"+source["id"].(string)+"/analysis", nil)
}

package integrationtest_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/project/catalog"
	"github.com/kakj-go/Judex/internal/workflow"
	"sync"
	"testing"
	"time"
)

func TestWorkflowPresetImport(t *testing.T) {
	svc, projects, identity, pool := newWorkflowService(t)
	ctx := context.Background()
	users := make([]uuid.UUID, 3)
	for i := range users {
		user, _, err := identity.Register(ctx, fmt.Sprintf("WF preset %d", i), fmt.Sprintf("wf-preset%d@project.test", i), "password-workflow-123", "10.9.0.1")
		if err != nil {
			t.Fatal(err)
		}
		users[i] = user.ID
	}
	p, err := projects.Create(ctx, users[0], project.CreateRequest{Title: "Workflow templates"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',$3)`, p.ID, users[1], time.Now()); err != nil {
		t.Fatal(err)
	}
	request := workflow.ImportPresetsRequest{CatalogVersion: catalog.Workflows().Version, ScenarioID: "software", WorkflowIDs: []string{"software-standard", "software-bug"}, Locale: "zh-CN"}
	count := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM workflow_definitions WHERE project_id=$1`, p.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, user := range users[1:] {
		if _, err := svc.ImportPresets(ctx, user, p.ID, request); err == nil {
			t.Fatal("non-manager import allowed")
		}
	}
	if count() != 0 {
		t.Fatal("unauthorized mutation")
	}
	result, err := svc.ImportPresets(ctx, users[0], p.ID, request)
	if err != nil || len(result.Items) != 2 || len(result.Skipped) != 0 {
		t.Fatal(result, err)
	}
	first := result.Items[0]
	if first.PublishedVersionID != nil || !first.HasDraft || first.PresetID == nil || *first.PresetID != "software-standard" {
		t.Fatal("import published or lost source", first)
	}
	versions, err := svc.ListVersions(ctx, users[0], p.ID, first.ID, true)
	if err != nil || len(versions) != 1 || versions[0].State != "draft" || len(versions[0].Body.Nodes) != 6 {
		t.Fatal(versions, err)
	}
	body := *versions[0].Body
	if len(body.AdvisoryEdges) != 6 || len(body.HardRules) != 0 || len(body.ApprovalPolicies) != 0 {
		t.Fatal("definition copy lost branches or imported authority")
	}
	for _, node := range body.Nodes {
		if len(node.AllowedPositionIds) > 0 || len(node.DelegationUserIDs) > 0 || node.Responsibility == "" {
			t.Fatal("node authority side effect")
		}
	}
	body.Name = "本项目定制流程"
	body.Nodes[0].Name = "本项目需求"
	body.Instructions = "本项目独立说明"
	if _, err = svc.UpdateDraft(ctx, users[0], p.ID, first.ID, first.Version, body); err != nil {
		t.Fatal(err)
	}
	request.Locale = "en"
	replay, err := svc.ImportPresets(ctx, users[0], p.ID, request)
	if err != nil || len(replay.Items) != 0 || len(replay.Skipped) != 2 || count() != 2 {
		t.Fatal("cross-language duplicate", replay, err)
	}
	versions, _ = svc.ListVersions(ctx, users[0], p.ID, first.ID, true)
	if versions[0].Body.Name != body.Name || versions[0].Body.Nodes[0].Name != body.Nodes[0].Name || versions[0].Body.Instructions != body.Instructions {
		t.Fatal("template replay overwrote custom draft")
	}
	if _, err = svc.Publish(ctx, users[0], p.ID, first.ID, 2, versions[0].DraftHash); err != nil {
		t.Fatal(err)
	}
	definitions, _ := svc.List(ctx, users[0], p.ID)
	for _, item := range definitions {
		if item.ID == first.ID && (item.PublishedVersionID == nil || item.PresetID == nil || *item.PresetID != "software-standard") {
			t.Fatal("publish lost source", item)
		}
	}
	for _, invalid := range []workflow.ImportPresetsRequest{
		{CatalogVersion: request.CatalogVersion, ScenarioID: "software", WorkflowIDs: []string{"software-light", "missing"}, Locale: "en"},
		{CatalogVersion: request.CatalogVersion, ScenarioID: "software", WorkflowIDs: []string{"content-publish"}, Locale: "en"},
		{CatalogVersion: request.CatalogVersion, ScenarioID: "software", WorkflowIDs: []string{"software-light", "software-light"}, Locale: "en"},
		{CatalogVersion: "old", ScenarioID: "software", WorkflowIDs: []string{"software-light"}, Locale: "en"},
	} {
		if _, err := svc.ImportPresets(ctx, users[0], p.ID, invalid); err == nil {
			t.Fatal("invalid selection accepted")
		}
		if count() != 2 {
			t.Fatal("partial invalid import")
		}
	}
	var wait sync.WaitGroup
	results := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := svc.ImportPresets(ctx, users[0], p.ID, workflow.ImportPresetsRequest{CatalogVersion: request.CatalogVersion, ScenarioID: "software", WorkflowIDs: []string{"software-light"}, Locale: "en"})
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count() != 3 {
		t.Fatal("concurrent duplicate")
	}
	custom := validBody()
	custom.Name = "VERSION RELEASE"
	if _, err = svc.Create(ctx, users[0], p.ID, custom); err != nil {
		t.Fatal(err)
	}
	collision, err := svc.ImportPresets(ctx, users[0], p.ID, workflow.ImportPresetsRequest{CatalogVersion: request.CatalogVersion, ScenarioID: "software", WorkflowIDs: []string{"software-release"}, Locale: "zh-CN"})
	if err != nil || len(collision.Items) != 0 || len(collision.Skipped) != 1 {
		t.Fatal("custom-name protection", collision, err)
	}
	before := count()
	if _, err = pool.Exec(ctx, `CREATE FUNCTION fail_workflow_preset_test() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN IF NEW.name='图文协作' THEN RAISE EXCEPTION 'injected failure'; END IF; RETURN NEW; END$$; CREATE TRIGGER fail_workflow_preset_test BEFORE INSERT ON workflow_versions FOR EACH ROW EXECUTE FUNCTION fail_workflow_preset_test();`); err != nil {
		t.Fatal(err)
	}
	rollback := workflow.ImportPresetsRequest{CatalogVersion: request.CatalogVersion, ScenarioID: "publishing", WorkflowIDs: []string{"content-publish", "content-design"}, Locale: "zh-CN"}
	if _, err = svc.ImportPresets(ctx, users[0], p.ID, rollback); err == nil || count() != before {
		t.Fatal("partial import after failure", err)
	}
	if _, err = pool.Exec(ctx, `DROP TRIGGER fail_workflow_preset_test ON workflow_versions; DROP FUNCTION fail_workflow_preset_test();`); err != nil {
		t.Fatal(err)
	}
	var positions, bindings, audits, versionsCount int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM position_templates WHERE project_id=$1),(SELECT count(*) FROM position_node_bindings WHERE project_id=$1),(SELECT count(*) FROM audit_events WHERE project_id=$1 AND operation='workflow.create'),(SELECT count(*) FROM workflow_versions WHERE project_id=$1)`, p.ID).Scan(&positions, &bindings, &audits, &versionsCount); err != nil {
		t.Fatal(err)
	}
	if positions != 0 || bindings != 0 || audits != before || versionsCount != before {
		t.Fatal("atomicity or authority", positions, bindings, audits, versionsCount)
	}
	if _, err = pool.Exec(ctx, `UPDATE projects SET status='archived' WHERE id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ImportPresets(ctx, users[0], p.ID, rollback); err == nil {
		t.Fatal("archived write allowed")
	}
}

package integrationtest_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/project"
)

func TestPositionPresetImport(t *testing.T) {
	svc, identity, pool := newProjectService(t)
	ctx := context.Background()
	users := make([]uuid.UUID, 3)
	for i := range users {
		user, _, err := identity.Register(ctx, fmt.Sprintf("Preset %d", i), fmt.Sprintf("preset%d@project.test", i), "password-preset-123", "10.8.0.1")
		if err != nil {
			t.Fatal(err)
		}
		users[i] = user.ID
	}
	p, err := svc.Create(ctx, users[0], project.CreateRequest{Title: "Preset import"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',$3)`, p.ID, users[1], time.Now()); err != nil {
		t.Fatal(err)
	}
	request := project.ImportPositionPresetsRequest{CatalogVersion: project.BuiltinPositionCatalog().Version, ScenarioID: "software", RoleIDs: []string{"frontend-developer", "qa-engineer"}, Locale: "zh-CN"}
	var initialIdentities int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_identities WHERE project_id=$1`, p.ID).Scan(&initialIdentities); err != nil {
		t.Fatal(err)
	}
	count := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM position_templates WHERE project_id=$1`, p.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, user := range users[1:] {
		if _, err := svc.ImportPositionPresets(ctx, user, p.ID, request); err == nil {
			t.Fatal("non-manager import allowed")
		}
	}
	if count() != 0 {
		t.Fatal("unauthorized mutation")
	}
	result, err := svc.ImportPositionPresets(ctx, users[0], p.ID, request)
	if err != nil || len(result.Items) != 2 || len(result.Skipped) != 0 {
		t.Fatal("selective import", result, err)
	}
	first := result.Items[0]
	if first.PresetID == nil || *first.PresetID != "frontend-developer" || first.Name != "前端开发" || len(first.NodeBindings) != 0 || first.NodeBindings == nil || first.Prompt == "" || first.PublicSummary == "" || first.ModelID != nil {
		t.Fatal("incorrect copied position", first)
	}
	updated, err := svc.UpdatePosition(ctx, users[0], p.ID, first.ID, first.CurrentVersion, project.PositionDraft{Name: "定制前端职责", Prompt: "本项目的独立要求", PublicSummary: "独立简介"})
	if err != nil || updated.PresetID == nil || *updated.PresetID != *first.PresetID {
		t.Fatal("source lost after edit", err)
	}
	request.Locale = "en"
	replay, err := svc.ImportPositionPresets(ctx, users[0], p.ID, request)
	if err != nil || len(replay.Items) != 0 || len(replay.Skipped) != 2 || count() != 2 {
		t.Fatal("duplicate import", replay, err)
	}
	list, err := svc.ListPositions(ctx, users[0], p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, position := range list {
		if position.ID == first.ID && (position.Name != updated.Name || position.Prompt != updated.Prompt || position.CurrentVersion != 2) {
			t.Fatal("import overwrote edited content")
		}
	}
	for _, invalid := range []project.ImportPositionPresetsRequest{
		{CatalogVersion: request.CatalogVersion, ScenarioID: "software", RoleIDs: []string{"backend-developer", "missing"}, Locale: "zh-CN"},
		{CatalogVersion: request.CatalogVersion, ScenarioID: "software", RoleIDs: []string{"screenwriter"}, Locale: "zh-CN"},
		{CatalogVersion: request.CatalogVersion, ScenarioID: "software", RoleIDs: []string{"backend-developer", "backend-developer"}, Locale: "zh-CN"},
		{CatalogVersion: "old", ScenarioID: "software", RoleIDs: []string{"backend-developer"}, Locale: "zh-CN"},
	} {
		if _, err := svc.ImportPositionPresets(ctx, users[0], p.ID, invalid); err == nil {
			t.Fatal("invalid import allowed", invalid)
		}
		if count() != 2 {
			t.Fatal("invalid selection partially imported")
		}
	}
	// Managers racing on the same selection must create each preset exactly once.
	concurrent := request
	concurrent.RoleIDs = []string{"backend-developer", "devops-engineer"}
	var wait sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := svc.ImportPositionPresets(ctx, users[0], p.ID, concurrent)
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count() != 4 {
		t.Fatal("concurrent import duplicated positions")
	}
	custom, err := svc.CreatePosition(ctx, users[0], p.ID, project.PositionDraft{Name: "PRODUCT MANAGER", Prompt: "Custom responsibilities"})
	if err != nil {
		t.Fatal(err)
	}
	collision := request
	collision.RoleIDs = []string{"product-manager"}
	collision.Locale = "zh-CN"
	matched, err := svc.ImportPositionPresets(ctx, users[0], p.ID, collision)
	if err != nil || len(matched.Items) != 0 || len(matched.Skipped) != 1 || matched.Skipped[0].PositionID != custom.ID {
		t.Fatal("custom-name duplicate", matched, err)
	}
	// Inject a failure after the first insertion in this owned test database.
	if _, err := pool.Exec(ctx, `CREATE FUNCTION fail_preset_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.prompt LIKE '你承担“UI/UX 设计师”%' THEN RAISE EXCEPTION 'preset test failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_preset_test BEFORE INSERT ON position_versions FOR EACH ROW EXECUTE FUNCTION fail_preset_test();`); err != nil {
		t.Fatal(err)
	}
	before := count()
	rollback := request
	rollback.RoleIDs = []string{"tech-lead", "ux-designer"}
	rollback.Locale = "zh-CN"
	if _, err := svc.ImportPositionPresets(ctx, users[0], p.ID, rollback); err == nil {
		t.Fatal("injected failure did not occur")
	}
	if count() != before {
		t.Fatal("database failure left a partial import")
	}
	if _, err := pool.Exec(ctx, `DROP TRIGGER fail_preset_test ON position_versions; DROP FUNCTION fail_preset_test();`); err != nil {
		t.Fatal(err)
	}
	var identities, bindings, audits int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM agent_identities WHERE project_id=$1), (SELECT count(*) FROM position_node_bindings WHERE project_id=$1), (SELECT count(*) FROM audit_events WHERE project_id=$1 AND operation='position.create')`, p.ID).Scan(&identities, &bindings, &audits); err != nil {
		t.Fatal(err)
	}
	if identities != initialIdentities || bindings != 0 || audits != before {
		t.Fatal("import changed authority or rollback lost audit atomicity", identities, bindings, audits, before)
	}
	if _, err := pool.Exec(ctx, `UPDATE projects SET status='archived' WHERE id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportPositionPresets(ctx, users[0], p.ID, rollback); err == nil {
		t.Fatal("archived project import allowed")
	}
}

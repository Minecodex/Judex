package integrationtest_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func TestPositionPreferencesIsolationAndReplacement(t *testing.T) {
	svc, ids, pool := newProjectService(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Same Name", "prefs-owner@test.local", "password-prefs-11", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := ids.Register(ctx, "Same Name", "prefs-other@test.local", "password-prefs-11", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	proj, err := svc.Create(ctx, owner.ID, project.CreateRequest{Title: "Position preferences"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',now())`, proj.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	create := func(name string, user uuid.UUID) (project.Position, project.Identity) {
		t.Helper()
		position, err := svc.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		identity, err := svc.CreateIdentity(ctx, owner.ID, proj.ID, position.ID, user)
		if err != nil {
			t.Fatal(err)
		}
		return position, identity
	}
	dev, devIdentity := create("Development", owner.ID)
	review, _ := create("Review", owner.ID)
	otherPosition, _ := create("Another person's position", other.ID)
	// Multiple identities of the same position still have one private setting.
	if _, err = svc.CreateIdentity(ctx, owner.ID, proj.ID, review.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	prefs, err := svc.GetMyPreferences(ctx, owner.ID, proj.ID)
	if err != nil || len(prefs) != 2 {
		t.Fatalf("my positions: %+v %v", prefs, err)
	}
	for _, pref := range prefs {
		if pref.Revision != 0 || pref.Prompt != "" {
			t.Fatalf("unsaved position: %+v", pref)
		}
	}
	for _, update := range []struct {
		id     uuid.UUID
		prompt string
	}{{dev.ID, "PRIVATE-DEVELOPMENT"}, {review.ID, "PRIVATE-REVIEW"}} {
		pref, err := svc.UpdateMyPreferences(ctx, owner.ID, proj.ID, update.id, 0, update.prompt)
		if err != nil || pref.PositionID != update.id || pref.Revision != 1 {
			t.Fatalf("save: %+v %v", pref, err)
		}
	}
	if _, err = svc.UpdateMyPreferences(ctx, owner.ID, proj.ID, dev.ID, 1, ""); err != nil {
		t.Fatal(err)
	}
	prefs, _ = svc.GetMyPreferences(ctx, owner.ID, proj.ID)
	for _, pref := range prefs {
		if pref.PositionID == dev.ID && (pref.Revision != 2 || pref.Prompt != "") {
			t.Fatalf("clear: %+v", pref)
		}
		if pref.PositionID == review.ID && (pref.Revision != 1 || pref.Prompt != "PRIVATE-REVIEW") {
			t.Fatalf("other role overwritten: %+v", pref)
		}
	}
	if _, err = svc.UpdateMyPreferences(ctx, owner.ID, proj.ID, otherPosition.ID, 0, "deny"); !apierrors.IsCode(err, apierrors.Forbidden) {
		t.Fatalf("unassigned write: %v", err)
	}
	if _, err = svc.UpdateMyPreferences(ctx, owner.ID, proj.ID, review.ID, 0, "stale"); !apierrors.IsCode(err, apierrors.VersionConflict) {
		t.Fatalf("stale write: %v", err)
	}
	if _, err = svc.UpdateMyPreferences(ctx, owner.ID, proj.ID, review.ID, 1, strings.Repeat("字", 20001)); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("oversized prompt: %v", err)
	}
	foreign, err := svc.Create(ctx, owner.ID, project.CreateRequest{Title: "Another project"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateMyPreferences(ctx, owner.ID, foreign.ID, review.ID, 0, "deny"); !apierrors.IsCode(err, apierrors.Forbidden) {
		t.Fatalf("cross-project position: %v", err)
	}
	if _, err = svc.GetMyPreferences(ctx, other.ID, foreign.ID); err == nil {
		t.Fatal("nonmember read succeeded")
	}
	if _, err = svc.ReplaceIdentityBinding(ctx, owner.ID, proj.ID, devIdentity.ID, other.ID, 1, "Replacement"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateMyPreferences(ctx, owner.ID, proj.ID, dev.ID, 2, "lost position"); !apierrors.IsCode(err, apierrors.Forbidden) {
		t.Fatalf("former holder write: %v", err)
	}
	prefs, _ = svc.GetMyPreferences(ctx, other.ID, proj.ID)
	for _, pref := range prefs {
		if pref.Prompt != "" || pref.Revision != 0 {
			t.Fatalf("successor inherited preference: %+v", pref)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM position_preference_revisions WHERE project_id=$1 AND user_id=$2`, proj.ID, owner.ID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("revision history: %d %v", count, err)
	}
	// A shared expected revision permits exactly one concurrent update.
	var group sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := svc.UpdateMyPreferences(ctx, owner.ID, proj.ID, review.ID, 1, "concurrent")
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else if apierrors.IsCode(err, apierrors.VersionConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent update: success=%d conflicts=%d", successes, conflicts)
	}
}

func TestPositionPreferencesMigrationCopiesCurrentAssignments(t *testing.T) {
	svc, ids, pool := newProjectService(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Migration", "prefs-migration@test.local", "password-prefs-11", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	proj, err := svc.Create(ctx, owner.ID, project.CreateRequest{Title: "Migration"})
	if err != nil {
		t.Fatal(err)
	}
	var positions []uuid.UUID
	for _, name := range []string{"Development", "Review"} {
		position, err := svc.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		positions = append(positions, position.ID)
		if _, err := svc.CreateIdentity(ctx, owner.ID, proj.ID, position.ID, owner.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.CreateIdentity(ctx, owner.ID, proj.ID, positions[0], owner.ID); err != nil {
		t.Fatal(err)
	}
	apply := func(file string) {
		t.Helper()
		sql, err := os.ReadFile(integration.ProjectPath(t, "internal", "infrastructure", "postgres", "migrations", file))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	// Only this test's disposable database is rewound to the legacy schema.
	apply("00022_position_preferences.down.sql")
	if _, err := pool.Exec(ctx, `INSERT INTO personal_project_preferences(project_id,user_id,revision,prompt,updated_at) VALUES($1,$2,2,'legacy-current',now());
`, proj.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO preference_revisions(project_id,user_id,revision,prompt,created_at) VALUES($1,$2,1,'legacy-old',now()),($1,$2,2,'legacy-current',now())`, proj.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	apply("00022_position_preferences.up.sql")
	prefs, err := svc.GetMyPreferences(ctx, owner.ID, proj.ID)
	if err != nil || len(prefs) != 2 {
		t.Fatalf("migration preferences: %+v %v", prefs, err)
	}
	for _, pref := range prefs {
		if pref.Revision != 2 || pref.Prompt != "legacy-current" {
			t.Fatalf("lost effective preference: %+v", pref)
		}
	}
	if _, err := svc.UpdateMyPreferences(ctx, owner.ID, proj.ID, positions[0], 2, "independent"); err != nil {
		t.Fatal(err)
	}
	prefs, _ = svc.GetMyPreferences(ctx, owner.ID, proj.ID)
	for _, pref := range prefs {
		if pref.PositionID == positions[1] && pref.Prompt != "legacy-current" {
			t.Fatalf("migrated copies still coupled: %+v", pref)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM position_preference_revisions WHERE project_id=$1 AND user_id=$2`, proj.ID, owner.ID).Scan(&count); err != nil || count != 5 {
		t.Fatalf("migration history: %d %v", count, err)
	}
	position, err := svc.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "New assignment"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateIdentity(ctx, owner.ID, proj.ID, position.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	prefs, _ = svc.GetMyPreferences(ctx, owner.ID, proj.ID)
	for _, pref := range prefs {
		if pref.PositionID == position.ID && (pref.Prompt != "" || pref.Revision != 0) {
			t.Fatalf("new assignment inherited legacy prompt: %+v", pref)
		}
	}
}

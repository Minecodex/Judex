package peer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func TestConfigurationEventsWithoutGroupActivity(t *testing.T) {
	f := setup(t)
	raw, err := f.s.SaveProject(f.ctx, uuid.Nil, collab.ProjectInput{Name: "Configuration only"})
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		ID      uuid.UUID
		Version int64
	}
	if err = json.Unmarshal(raw, &project); err != nil {
		t.Fatal(err)
	}
	if err = f.s.SaveMember(f.ctx, project.ID, collab.ProjectMemberInput{UserID: f.other, Role: "admin"}, false); err != nil {
		t.Fatal(err)
	}
	other := auth.WithPrincipal(f.ctx, &auth.Principal{Kind: auth.KindWeb, UserID: f.other})
	events := func(ctx context.Context, after int64) ([]collab.Event, int64) {
		t.Helper()
		items, cursor, e := f.s.PersonalEvents(ctx, after)
		if e != nil {
			t.Fatal(e)
		}
		return items, cursor
	}
	_, before := events(other, 0)
	_, err = f.s.SaveProject(f.ctx, project.ID, collab.ProjectInput{Name: "Updated without chat", Description: "Remote description", ExpectedVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	items, after := events(other, before)
	if len(items) != 1 || items[0].Kind != "project.changed" || !strings.Contains(string(items[0].Data), project.ID.String()) {
		t.Fatalf("missing project-only event: %+v", items)
	}
	if strings.Contains(string(items[0].Data), "Remote description") || strings.Contains(string(items[0].Data), "conversation") {
		t.Fatal("configuration event leaked content or group identifiers")
	}
	// Conflicting saves emit nothing and do not replace the remote configuration.
	_, err = f.s.SaveProject(f.ctx, project.ID, collab.ProjectInput{Name: "stale", ExpectedVersion: 1})
	if !apierrors.IsCode(err, apierrors.VersionConflict) {
		t.Fatalf("conflict: %v", err)
	}
	if items, _ = events(other, after); len(items) != 0 {
		t.Fatal("conflict emitted an update")
	}
	position, err := f.s.SaveTemplate(f.ctx, project.ID, uuid.Nil, "positions", collab.TemplateInput{Name: "Position", Prompt: "Public role"})
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := f.s.SaveTemplate(f.ctx, project.ID, uuid.Nil, "workflows", collab.TemplateInput{Name: "Workflow", Content: "Public reference"})
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range []struct {
		id   uuid.UUID
		kind string
	}{{position, "positions"}, {workflow, "workflows"}} {
		if _, err = f.s.SaveTemplate(f.ctx, project.ID, spec.id, spec.kind, collab.TemplateInput{Name: "Revised", Prompt: "Updated", Content: "Updated", ExpectedVersion: 1}); err != nil {
			t.Fatal(err)
		}
	}
	items, after = events(other, after)
	if len(items) != 4 {
		t.Fatalf("template create/revision events: %d", len(items))
	}
	for _, item := range items {
		if item.Kind != "project.changed" {
			t.Fatalf("unexpected event: %+v", item)
		}
	}
	// Private prompts refresh only their author's sessions, with identifiers only.
	_, ownerBefore := events(f.ctx, 0)
	if err = f.s.SavePrompt(f.ctx, project.ID, position, "private instructions do not broadcast", 0); err != nil {
		t.Fatal(err)
	}
	if items, _ = events(other, after); len(items) != 0 {
		t.Fatal("other member received private-prompt event")
	}
	items, _ = events(f.ctx, ownerBefore)
	if len(items) != 1 || items[0].Kind != "personal-prompt.changed" || strings.Contains(string(items[0].Data), "private instructions") {
		t.Fatalf("private event: %+v", items)
	}
	// A narrowed CLI cannot receive even configuration identifiers out of scope.
	cli := auth.WithPrincipal(f.ctx, &auth.Principal{Kind: auth.KindCLI, UserID: f.other, ProjectScope: []uuid.UUID{f.project}})
	scoped, cursor := events(cli, before)
	if len(scoped) != 0 || cursor < after {
		t.Fatalf("scope filtering/cursor failed: %+v %d", scoped, cursor)
	}
	// Events and configuration share their caller's transaction.
	sentinel := errors.New("abort outer command")
	err = f.s.Pool.Transact(f.ctx, func(ctx context.Context, _ postgres.Tx) error {
		_, e := f.s.SaveTemplate(ctx, project.ID, uuid.Nil, "positions", collab.TemplateInput{Name: "Rolled back"})
		if e != nil {
			return e
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if items, _ = events(other, after); len(items) != 0 {
		t.Fatal("rolled-back configuration emitted an event")
	}
	if err = f.s.SaveMember(f.ctx, project.ID, collab.ProjectMemberInput{UserID: f.other, Role: "member", ExpectedVersion: 1}, false); err != nil {
		t.Fatal(err)
	}
	items, after = events(other, after)
	if len(items) != 2 || items[0].Kind != "project.changed" {
		t.Fatalf("membership change: %+v", items)
	}
	if err = f.s.SaveMember(f.ctx, project.ID, collab.ProjectMemberInput{UserID: f.other, ExpectedVersion: 2}, true); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.SaveProject(f.ctx, project.ID, collab.ProjectInput{Name: "After revocation", ExpectedVersion: 2}); err != nil {
		t.Fatal(err)
	}
	items, _ = events(other, after)
	if len(items) != 1 || items[0].Kind != "access.changed" {
		t.Fatalf("revoked user received config: %+v", items)
	}
}

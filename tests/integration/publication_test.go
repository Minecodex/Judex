package integrationtest_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/material"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"io"
	"strings"
	"testing"
)

func TestAgentPublicationIsVersionedIdempotentAndSourceGuarded(t *testing.T) {
	materials, projects, ids, pool := newMaterialEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Publisher", "publisher@test.local", "publication-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Derived artifact"})
	if err != nil {
		t.Fatal(err)
	}
	upload := func(text string, source *uuid.UUID) material.MaterialVersion {
		t.Helper()
		u, err := materials.CreateUpload(ctx, user.ID, p.ID, "source.txt", "file", "text/plain", int64(len(text)), digest(text), "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = materials.UploadPart(ctx, user.ID, p.ID, u.ID, 1, digest(text), strings.NewReader(text)); err != nil {
			t.Fatal(err)
		}
		v, err := materials.Complete(ctx, user.ID, p.ID, u.ID, source)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	source := upload("明确保留反对意见", nil)
	var topic uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT id FROM topics WHERE project_id=$1`, p.ID).Scan(&topic); err != nil {
		t.Fatal(err)
	}
	sub, err := discussion.NewService(pool, nil).CreateSubmission(ctx, user.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "web", Text: "分析", TopicID: &topic})
	if err != nil {
		t.Fatal(err)
	}
	runtime := batch.Service{Pool: pool}
	_, run, err := runtime.Start(ctx, user.ID, p.ID, topic, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	publication := material.Publication{Name: "分析页面", Kind: "html_bundle", Mime: "text/html", Entrypoint: "index.html", SourceVersionIDs: []uuid.UUID{source.ID}, Key: "one-artifact"}
	html := []byte("<!doctype html><h1>待人工核对</h1><p>保留反对意见</p>")
	publish := func(in material.Publication, body []byte) (material.MaterialVersion, error) {
		var out material.MaterialVersion
		err := pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
			if err := tx.LockActiveProject(ctx, p.ID.String()); err != nil {
				return err
			}
			var err error
			out, err = materials.PublishInTx(ctx, tx, p.ID, run, in, body)
			return err
		})
		return out, err
	}
	v, err := publish(publication, html)
	if err != nil {
		t.Fatal(err)
	}
	again, err := publish(publication, html)
	if err != nil || again.ID != v.ID {
		t.Fatalf("publication duplicated: %v", err)
	}
	if _, err = publish(publication, []byte("changed")); !apierrors.IsCode(err, apierrors.IdempotencyConflict) {
		t.Fatalf("key reused with changed bytes: %v", err)
	}
	reader, _, err := materials.VersionObject(ctx, user.ID, p.ID, v.MaterialID, v.ID, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(reader)
	reader.Close()
	if string(content) != string(html) {
		t.Fatal("published HTML differs")
	}
	upload("新的来源版本", &source.ID)
	publication.Key = "stale-artifact"
	if _, err = publish(publication, html); !apierrors.IsCode(err, apierrors.SourceVersionConf) {
		t.Fatalf("stale source accepted: %v", err)
	}
	result, err := runtime.Get(ctx, user.ID, p.ID, run)
	if err != nil || len(result.ArtifactRefs) != 1 {
		t.Fatalf("run lacks published reference: %+v %v", result, err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM artifact_publications WHERE run_id=$1`, run).Scan(&count); err != nil || count != 1 {
		t.Fatalf("wrong publication count: %d %v", count, err)
	}
}

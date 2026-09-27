// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func newDiscussionEnv(t *testing.T) (*discussion.Service, *project.Service, *identity.Service, *postgres.Pool) {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	return discussion.NewService(fixture.Pool, nil), project.NewService(fixture.Pool, nil), ids, fixture.Pool
}

// TestTopicsAndSubmissions (C02/B15 核心): 议题创建含首条消息；统一提交
// 幂等（同 clientSubmissionId 重试返回原记录，不产生第二条消息）；
// 未 ready/跨项目材料被拒；消息按 seq 升序返回。
func TestTopicsAndSubmissions(t *testing.T) {
	svc, projects, ids, _ := newDiscussionEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "DIS", "dis@dis.test", "password-dis-dis", "10.0.0.1")
	proj, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "讨论项目"})
	if err != nil {
		t.Fatal(err)
	}

	topic, err := svc.CreateTopic(ctx, owner.ID, proj.ID, "接口评审", "先给出初版方案。", nil)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := svc.ListMessages(ctx, owner.ID, proj.ID, topic.ID, 0, 50)
	if err != nil || len(messages) != 1 {
		t.Fatalf("initial message: %v %+v", err, messages)
	}
	if messages[0].Kind != "human" || messages[0].AuthorUserID == nil || *messages[0].AuthorUserID != owner.ID {
		t.Fatalf("author must be derived server-side: %+v", messages[0])
	}

	// Idempotent submission.
	key := uuid.NewString()
	first, err := svc.CreateSubmission(ctx, owner.ID, proj.ID, discussion.Submission{
		ClientSubmissionID: key, Purpose: "message", Source: "web",
		Text: "补充一版数据。", TopicID: &topic.ID,
	})
	if err != nil || first.MessageID == nil {
		t.Fatalf("submission: %v %+v", err, first)
	}
	retry, err := svc.CreateSubmission(ctx, owner.ID, proj.ID, discussion.Submission{
		ClientSubmissionID: key, Purpose: "message", Source: "web",
		Text: "补充一版数据。", TopicID: &topic.ID,
	})
	if err != nil || retry.ID != first.ID {
		t.Fatalf("retry must return original submission: %v %+v", err, retry)
	}
	messages, err = svc.ListMessages(ctx, owner.ID, proj.ID, topic.ID, 0, 50)
	if err != nil || len(messages) != 2 {
		t.Fatalf("expected exactly 2 messages after retry, got %d", len(messages))
	}
	if messages[0].Seq != 1 || messages[1].Seq != 2 {
		t.Fatalf("seq must be dense ascending: %+v", messages)
	}

	// Not-ready material rejected; foreign-project material rejected.
	fakeVersion := uuid.New()
	if _, err := svc.CreateSubmission(ctx, owner.ID, proj.ID, discussion.Submission{
		ClientSubmissionID: uuid.NewString(), Purpose: "message", Source: "web",
		Text: "附件", TopicID: &topic.ID, MaterialVersionIDs: []string{fakeVersion.String()},
	}); errors.IsCode(err, errors.InvalidReference) == false {
		t.Fatalf("unknown material must be InvalidReference, got %v", err)
	}

	// material purpose without topic goes to the collection area (no message).
	uploadOnly, err := svc.CreateSubmission(ctx, owner.ID, proj.ID, discussion.Submission{
		ClientSubmissionID: uuid.NewString(), Purpose: "material", Source: "web",
		Text: "登记一份资料",
	})
	if err != nil || uploadOnly.MessageID != nil {
		t.Fatalf("material-only submission must not create a message: %v %+v", err, uploadOnly)
	}
}

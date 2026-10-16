package integrationtest_test

import (
	"context"
	"github.com/google/uuid"
	hservice "github.com/kakj-go/Judex/internal/handoff"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *contractScenario) approve(changes []any, delegate bool) map[string]any {
	c, p := s.call, s.prefix
	proposal := c("POST", p+"/proposals", map[string]any{"kind": "work_arrangement", "changes": changes})
	path := p + "/proposals/" + proposal["id"].(string)
	c("PUT", path+"/draft", map[string]any{"expectedVersion": proposal["version"], "reason": "Contract explicit revision", "changes": changes})
	review := c("GET", path+"/review", nil)
	review = c("POST", path+"/submit", map[string]any{"expectedVersion": review["version"], "draftHash": review["reviewHash"]})
	slots, bindings := []any{}, []any{}
	for _, raw := range review["slots"].([]any) {
		slot := raw.(map[string]any)
		slots = append(slots, slot["id"])
		if slot["authorityType"] == "identity" {
			bindings = append(bindings, map[string]any{"identityId": slot["authorityId"], "bindingVersion": slot["bindingVersion"]})
		}
	}
	if delegate {
		c("POST", path+"/delegate-decisions", map[string]any{"reviewId": review["reviewId"], "reviewHash": review["reviewHash"], "slotIds": slots, "reason": "Owner explicitly covers selected fixture seats"})
		return c("GET", path+"/review", nil)
	}
	return c("POST", path+"/decisions", map[string]any{"reviewId": review["reviewId"], "reviewHash": review["reviewHash"], "expectedVersion": review["version"], "decision": "approve", "slotIds": slots, "actingBindingVersions": bindings})
}

func (s *contractScenario) workLifecycle() {
	c, p := s.call, s.prefix
	result := s.approve([]any{
		map[string]any{"operation": "create_plan", "targetType": "plan", "clientRef": "plan", "fields": map[string]any{"title": "Formal contract plan", "ownerIdentityId": s.identity["id"]}},
		map[string]any{"operation": "create_task", "targetType": "task", "clientRef": "task", "fields": map[string]any{"title": "Formal contract task", "planId": "plan", "participantIdentityIds": []any{s.identity["id"]}, "reviewerIdentityId": s.identity["id"]}},
	}, false)
	ids := result["createdIds"].(map[string]any)
	planPath := p + "/plans/" + ids["plan"].(string)
	taskPath := p + "/tasks/" + ids["task"].(string)
	c("GET", planPath+"/acceptances?limit=1", nil)
	c("GET", taskPath+"/acceptances?limit=1", nil)
	task := c("POST", taskPath+"/start", map[string]any{"expectedVersion": 1, "identityId": s.identity["id"]})
	report := c("POST", taskPath+"/reports", map[string]any{"kind": "delivery", "text": "Contract handoff proof", "identityId": s.identity["id"], "expectedTaskVersion": task["version"]})
	handoff := c("POST", p+"/handoffs", map[string]any{"title": "Contract handoff", "kind": "stage", "targetTaskId": ids["task"], "receiverIdentityId": s.identity["id"], "sources": []any{map[string]any{"sourceTaskId": ids["task"], "senderIdentityId": s.identity["id"]}}})
	handoffs := c("GET", p+"/handoffs", nil)["items"].([]any)
	var source map[string]any
	for _, raw := range handoffs {
		h := raw.(map[string]any)
		if h["id"] == handoff["id"] {
			source = h["sources"].([]any)[0].(map[string]any)
		}
	}
	sourcePath := p + "/handoffs/" + handoff["id"].(string) + "/sources/" + source["id"].(string)
	first := c("POST", sourcePath+"/send", map[string]any{"sourceVersion": 1, "reviewHash": report["id"], "summary": "First reviewed delivery"})
	c("POST", sourcePath+"/decisions", map[string]any{"sourceVersion": first["currentVersion"], "reviewHash": first["currentVersionId"], "decision": "reject", "reason": "Request revised source"})
	currentTask := c("GET", taskPath, nil)
	currentTask = c("POST", taskPath+"/start", map[string]any{"expectedVersion": currentTask["version"], "identityId": s.identity["id"]})
	report = c("POST", taskPath+"/reports", map[string]any{"kind": "delivery", "text": "Revised source proof", "identityId": s.identity["id"], "expectedTaskVersion": currentTask["version"]})
	revision := c("POST", sourcePath+"/revisions", map[string]any{"summary": "Explicit source revision", "reviewHash": report["id"], "sourceVersion": 2})
	for _, raw := range c("GET", p+"/handoffs", nil)["items"].([]any) {
		row := raw.(map[string]any)
		if row["id"] == handoff["id"] {
			handoff = row
		}
	}
	c("POST", p+"/handoffs/"+handoff["id"].(string)+"/reminders", map[string]any{"expectedVersion": handoff["version"]})
	if _, err := hservice.NewService(s.pool, nil).Remind(context.Background(), uuid.MustParse(s.user["id"].(string)), uuid.MustParse(p[len("/projects/"):]), uuid.MustParse(handoff["id"].(string)), int64(handoff["version"].(float64))); !apierrors.IsCode(err, apierrors.RateLimited) {
		s.t.Fatalf("repeated reminder bypassed cooldown: %v", err)
	}
	c("POST", sourcePath+"/decisions", map[string]any{"sourceVersion": revision["currentVersion"], "reviewHash": revision["currentVersionId"], "decision": "accept"})
	review := c("GET", taskPath+"/acceptance-review", nil)
	accepted := c("POST", taskPath+"/acceptances", map[string]any{"reviewId": review["reviewId"], "reviewHash": review["reviewHash"], "expectedVersion": review["targetVersion"], "decision": "accept"})
	// CLI-style send intents freeze the CURRENT revision; the domain creates
	// its successor only when the named browser confirms. Keep final task
	// acceptance intact through a source rejection and corrected re-send.
	intentHandoff := c("POST", p+"/handoffs", map[string]any{"title": "Intent revision proof", "kind": "stage", "targetTaskId": ids["task"], "receiverIdentityId": s.identity["id"], "sources": []any{map[string]any{"sourceTaskId": ids["task"], "senderIdentityId": s.identity["id"]}}})
	for _, raw := range c("GET", p+"/handoffs", nil)["items"].([]any) {
		row := raw.(map[string]any)
		if row["id"] == intentHandoff["id"] {
			source = row["sources"].([]any)[0].(map[string]any)
		}
	}
	intentSourcePath := p + "/handoffs/" + intentHandoff["id"].(string) + "/sources/" + source["id"].(string)
	for currentRevision := 0; currentRevision < 2; currentRevision++ {
		intent := c("POST", p+"/confirmation-intents", map[string]any{"operation": "handoff.send", "objectId": source["id"], "reviewHash": report["id"], "payload": map[string]any{"sourceVersion": currentRevision, "summary": "Reviewed source revision"}})
		intentPath := p + "/confirmation-intents/" + intent["id"].(string)
		frozen := c("GET", intentPath, nil)
		c("POST", intentPath+"/confirm", map[string]any{"decision": "approve", "intentHash": frozen["intentHash"]})
		for _, raw := range c("GET", p+"/handoffs", nil)["items"].([]any) {
			row := raw.(map[string]any)
			if row["id"] == intentHandoff["id"] {
				source = row["sources"].([]any)[0].(map[string]any)
			}
		}
		if source["currentVersion"] != float64(currentRevision+1) || source["state"] != "pending" {
			s.t.Fatalf("intent did not create the reviewed successor: %+v", source)
		}
		decision, reason := "reject", "Correct the source summary"
		if currentRevision == 1 {
			decision, reason = "accept", ""
		}
		c("POST", intentSourcePath+"/decisions", map[string]any{"sourceVersion": source["currentVersion"], "reviewHash": source["currentVersionId"], "decision": decision, "reason": reason})
		unchanged := c("GET", taskPath, nil)
		if unchanged["status"] != "accepted" || unchanged["version"] != accepted["version"] || unchanged["latestAcceptanceId"] != accepted["latestAcceptanceId"] {
			s.t.Fatal("source action changed final task acceptance")
		}
	}
	c("GET", planPath+"/discussion-summary", nil)
	review = c("GET", planPath+"/acceptance-review", nil)
	plan := c("POST", planPath+"/acceptances", map[string]any{"reviewId": review["reviewId"], "reviewHash": review["reviewHash"], "expectedVersion": review["targetVersion"], "decision": "accept"})
	c("POST", planPath+"/reopens", map[string]any{"expectedVersion": plan["version"], "acceptanceId": plan["latestAcceptanceId"], "reason": "Contract plan reopen"})
	c("POST", taskPath+"/reopens", map[string]any{"expectedVersion": accepted["version"], "acceptanceId": accepted["latestAcceptanceId"], "reason": "Contract task reopen"})
	bug := c("POST", p+"/tasks", map[string]any{"title": "Contract bug", "kind": "bug", "bugDetails": map[string]any{"environment": "isolated", "steps": "Read contract", "expected": "Schema matches", "actual": "Mismatch", "severity": "medium"}})
	c("POST", p+"/tasks/"+bug["id"].(string)+"/fix-propagations", map[string]any{"targetReleaseRefs": []any{"contract-release.1"}})
	s.approve([]any{map[string]any{"operation": "create_task", "targetType": "task", "clientRef": "delegated", "fields": map[string]any{"title": "Explicit delegated fixture task", "reviewerIdentityId": s.identity["id"], "participantIdentityIds": []any{s.identity["id"]}}}}, true)
	// A rejected review is revised through the public API, retaining its ID.
	changes := []any{map[string]any{"operation": "create_task", "targetType": "task", "clientRef": "revision", "fields": map[string]any{"title": "Revision fixture", "reviewerIdentityId": s.identity["id"], "participantIdentityIds": []any{s.identity["id"]}}}}
	proposal := c("POST", p+"/proposals", map[string]any{"kind": "work_arrangement", "changes": changes})
	path := p + "/proposals/" + proposal["id"].(string)
	review = c("GET", path+"/review", nil)
	review = c("POST", path+"/submit", map[string]any{"expectedVersion": 1, "draftHash": review["reviewHash"]})
	slots := []any{}
	bindings := []any{}
	for _, raw := range review["slots"].([]any) {
		slot := raw.(map[string]any)
		slots = append(slots, slot["id"])
		if slot["authorityType"] == "identity" {
			bindings = append(bindings, map[string]any{"identityId": slot["authorityId"], "bindingVersion": slot["bindingVersion"]})
		}
	}
	c("POST", path+"/decisions", map[string]any{"reviewId": review["reviewId"], "reviewHash": review["reviewHash"], "expectedVersion": review["version"], "decision": "reject", "reason": "More proof needed", "slotIds": slots, "actingBindingVersions": bindings})
	c("POST", path+"/revisions", map[string]any{"previousReviewId": review["reviewId"], "reason": "Add proof", "changes": changes})
}

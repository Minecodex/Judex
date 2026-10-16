package work

import apierrors "github.com/kakj-go/Judex/internal/platform/errors"

// ValidateCreateChangeFields shares the formal work field contract with draft
// creation/revision/submission. A field cannot survive review and be discarded
// or rejected only when the last approval attempts to apply the change group.
func ValidateCreateChangeFields(c Change) error {
	if c.Operation != "create_plan" && c.Operation != "create_task" {
		return nil
	}
	allowed := map[string]bool{"title": true, "acceptanceCriteria": true, "workflowId": true}
	if c.Operation == "create_plan" {
		for _, key := range []string{"goal", "ownerIdentityId", "sourceTopicId", "forkAfterSeq"} {
			allowed[key] = true
		}
	} else {
		for _, key := range []string{"expectedOutput", "reviewerIdentityId", "participantIdentityIds", "planId", "parentTaskId", "nodeId", "kind", "requirements", "bugDetails"} {
			allowed[key] = true
		}
	}
	for key := range c.Fields {
		if !allowed[key] {
			return apierrors.Fields("fields."+key, "unsupported")
		}
	}
	return nil
}

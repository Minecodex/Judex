package decision

import apierrors "github.com/kakj-go/Judex/internal/platform/errors"

// ValidateDraftChanges is shared by human and Agent draft creation.
func ValidateDraftChanges(changes []Change) error {
	if len(changes) == 0 {
		return apierrors.Fields("changes", "required")
	}
	for i, change := range changes {
		if !allowedOperations[change.Operation] {
			return apierrors.Newf(apierrors.Validation, "changes[%d].operation not supported", i)
		}
		if change.TargetType != "plan" && change.TargetType != "task" {
			return apierrors.Fields("targetType", "enum")
		}
	}
	return nil
}

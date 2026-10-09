package work

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"math"
)

func ForkOriginFromFields(ctx context.Context, q collaboration.Reader, project uuid.UUID, fields map[string]any) (collaboration.Origin, error) {
	origin := collaboration.Origin{}
	raw, valid := fields["sourceTopicId"].(string)
	if _, exists := fields["sourceTopicId"]; exists && !valid {
		return origin, apierrors.Fields("sourceTopicId", "uuid")
	}
	if raw == "" {
		if _, ok := fields["forkAfterSeq"]; ok {
			return origin, apierrors.Fields("sourceTopicId", "required with fork point")
		}
		return origin, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return origin, apierrors.Fields("sourceTopicId", "uuid")
	}
	origin.TopicID = id
	if v, ok := fields["forkAfterSeq"]; ok {
		var n int64
		switch value := v.(type) {
		case float64:
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value >= float64(math.MaxInt64) || value != math.Trunc(value) {
				return origin, apierrors.Fields("forkAfterSeq", "integer")
			}
			n = int64(value)
		case int64:
			n = value
		case int:
			n = int64(value)
		default:
			return origin, apierrors.Fields("forkAfterSeq", "integer")
		}
		origin.AfterSeq = &n
	}
	cut, err := collaboration.ValidateFork(ctx, q, project, id, origin.AfterSeq)
	if err != nil {
		return origin, err
	}
	origin.AfterSeq = &cut
	return origin, nil
}

// SnapshotPlanOrigins is shared by human and Agent draft entry points. Approval
// later uses these frozen values even when the source discussion has moved on.
func SnapshotPlanOrigins(ctx context.Context, q collaboration.Reader, project uuid.UUID, topic *uuid.UUID, point *int64, changes []Change) ([]Change, error) {
	out := append([]Change(nil), changes...)
	for i := range out {
		if out[i].Operation != "create_plan" {
			continue
		}
		fields := map[string]any{}
		for key, value := range out[i].Fields {
			fields[key] = value
		}
		if _, ok := fields["sourceTopicId"]; !ok && topic != nil {
			fields["sourceTopicId"] = topic.String()
			if point != nil {
				fields["forkAfterSeq"] = *point
			}
		}
		origin, err := ForkOriginFromFields(ctx, q, project, fields)
		if err != nil {
			return nil, err
		}
		if origin.TopicID != uuid.Nil {
			fields["sourceTopicId"] = origin.TopicID.String()
			fields["forkAfterSeq"] = *origin.AfterSeq
		}
		out[i].Fields = fields
	}
	return out, nil
}

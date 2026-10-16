package batch

import (
	"context"
	"errors"

	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// Persist a bounded diagnostic category, never the provider's response body or
// credentials. Usage remains independently known/unknown and conservatively
// charged; a known failure cause does not make unknown usage equal to zero.
func modelFailureCode(err error) *string {
	if err == nil {
		return nil
	}
	code := "UNKNOWN_PROVIDER_ERROR"
	switch {
	case errors.Is(err, context.Canceled):
		code = "CANCELLED"
	case errors.Is(err, context.DeadlineExceeded):
		code = "TIMEOUT"
	default:
		switch model.Classify(err) {
		case model.ErrorTransient:
			code = "UPSTREAM_TRANSIENT"
		case model.ErrorQuota:
			code = "QUOTA_EXHAUSTED"
		case model.ErrorAuth:
			code = "AUTH_FAILED"
		case model.ErrorInvalidRequest:
			code = "REQUEST_INVALID"
		case model.ErrorContextExceeded:
			code = "CONTEXT_EXCEEDED"
		case model.ErrorCancelled:
			code = "CANCELLED"
		case model.ErrorRateLimited:
			code = "RATE_LIMITED"
		case model.ErrorOutputTruncated:
			code = "OUTPUT_TRUNCATED"
		}
	}
	return &code
}

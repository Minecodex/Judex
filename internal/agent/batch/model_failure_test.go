package batch

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

func TestModelFailureCategoriesDoNotPersistProviderText(t *testing.T) {
	if modelFailureCode(nil) != nil {
		t.Fatal("successful call has a failure category")
	}
	for _, sample := range []struct {
		err  error
		code string
	}{
		{&model.ProviderError{Class: model.ErrorRateLimited, Err: errors.New("sensitive upstream body")}, "RATE_LIMITED"},
		{&model.ProviderError{Class: model.ErrorQuota, Err: errors.New("sensitive upstream body")}, "QUOTA_EXHAUSTED"},
		{&model.ProviderError{Class: model.ErrorAuth, Err: errors.New("sensitive upstream body")}, "AUTH_FAILED"},
		{&model.ProviderError{Class: model.ErrorInvalidRequest, Err: errors.New("sensitive upstream body")}, "REQUEST_INVALID"},
		{&model.ProviderError{Class: model.ErrorContextExceeded, Err: errors.New("sensitive upstream body")}, "CONTEXT_EXCEEDED"},
		{&model.ProviderError{Class: model.ErrorOutputTruncated, Err: errors.New("sensitive upstream body")}, "OUTPUT_TRUNCATED"},
		{&model.ProviderError{Class: model.ErrorTransient, Err: errors.New("sensitive upstream body")}, "UPSTREAM_TRANSIENT"},
		{fmt.Errorf("wrapped: %w", context.Canceled), "CANCELLED"},
		{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), "TIMEOUT"},
		{errors.New("sensitive unclassified upstream body"), "UNKNOWN_PROVIDER_ERROR"},
	} {
		if got := modelFailureCode(sample.err); got == nil || *got != sample.code {
			t.Fatalf("wrong bounded category: expected %s got %v", sample.code, got)
		}
	}
}

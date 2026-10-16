package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

func TestQuotaRequiresProviderEvidence(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   model.ErrorClass
	}{
		{"rate", 429, `{"error":{"type":"rate_limit_error","message":"too many requests"}}`, model.ErrorRateLimited},
		{"ambiguous", 429, `{"error":{"message":"quota"}}`, model.ErrorRateLimited},
		{"coding-window", 429, `{"error":{"code":"1308","message":"已达到 5 小时的使用上限。您的限额将在 2026-10-13 01:01:31 重置。"}}`, model.ErrorQuota},
		{"unqualified-1308", 429, `{"error":{"code":"1308","message":"too many requests"}}`, model.ErrorRateLimited},
		{"bigmodel", 429, `{"error":{"code":"1113","message":"insufficient resources"}}`, model.ErrorQuota},
		{"quota", 429, `{"error":{"type":"insufficient_quota"}}`, model.ErrorQuota},
		{"invalid-key", 401, `{"error":{"code":"1113"}}`, model.ErrorAuth},
		{"kimi-five-hour-window", 403, `{"error":{"type":"access_terminated_error","message":"You've reached your 5-hour usage limit. Your quota will reset when the current 5-hour window ends."}}`, model.ErrorQuota},
		{"kimi-terminated-access", 403, `{"error":{"type":"access_terminated_error","message":"Your access has been terminated."}}`, model.ErrorAuth},
		{"kimi-limit-without-reset", 403, `{"error":{"type":"access_terminated_error","message":"You've reached your 5-hour usage limit."}}`, model.ErrorAuth},
		{"kimi-reset-without-limit", 403, `{"error":{"type":"access_terminated_error","message":"Your quota will reset when the window ends."}}`, model.ErrorAuth},
		{"untyped-forbidden-quota", 403, `{"error":{"message":"You've reached your 5-hour usage limit. Your quota will reset."}}`, model.ErrorAuth},
		{"unauthenticated-kimi-window", 401, `{"error":{"type":"access_terminated_error","message":"You've reached your 5-hour usage limit. Your quota will reset."}}`, model.ErrorAuth},
		{"forbidden-other-quota", 403, `{"error":{"type":"insufficient_quota"}}`, model.ErrorAuth},
		{"malformed-forbidden", 403, `{"error":`, model.ErrorAuth},
		{"network-gateway", 503, `{"error":{"message":"temporary"}}`, model.ErrorTransient},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			_, err := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: s.URL, APIKey: "test-key"}).Stream(context.Background(), model.Request{Model: "m"})
			if err == nil || model.Classify(err) != tc.want {
				t.Fatalf("class=%v, err=%v", model.Classify(err), err)
			}
		})
	}
}

func TestOpenAIStreamQuotaAndRateErrors(t *testing.T) {
	for _, tc := range []struct {
		body  string
		class model.ErrorClass
	}{
		{`{"error":{"code":"1113"}}`, model.ErrorQuota},
		{`{"error":{"type":"access_terminated_error","message":"You've reached your 5-hour usage limit. Your quota will reset when the current 5-hour window ends."}}`, model.ErrorQuota},
		{`{"error":{"type":"access_terminated_error","message":"Your access has been terminated."}}`, model.ErrorUnknown},
		{`{"error":{"type":"rate_limit_error"}}`, model.ErrorRateLimited},
		{`{"error":{"message":"quota"}}`, model.ErrorUnknown},
	} {
		server := fakeGateway(t, []string{tc.body}, 200)
		stream, err := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: server.URL, APIKey: "test-key"}).Stream(context.Background(), model.Request{Model: "m"})
		if err != nil {
			t.Fatal(err)
		}
		var failed error
		for event := range stream {
			if event.Type == "error" {
				failed = event.Err
			}
		}
		server.Close()
		if failed == nil || model.Classify(failed) != tc.class {
			t.Fatalf("stream error=%v want=%v", failed, tc.class)
		}
	}
}

func TestPositionCannotSeeOrExecuteCoordinatorPersistence(t *testing.T) {
	r := tools.New()
	tools.RegisterDefaults(r)
	tools.RegisterCallAgent(r, nil)
	for _, principal := range []string{"position", "user", "agent"} {
		for _, schema := range r.SchemasFor(principal) {
			if schema["name"] == "call_agent" || schema["name"] == "record_task_analysis" {
				t.Fatalf("%s sees coordinator capability", principal)
			}
		}
		if _, err := r.Execute(context.Background(), "record_task_analysis", principal, map[string]any{"summary": "test"}, tools.Env{}); err == nil {
			t.Fatalf("%s could execute persistence", principal)
		}
	}
	seen := false
	for _, schema := range r.SchemasFor("coordinator") {
		if schema["name"] == "record_task_analysis" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("coordinator persistence missing")
	}
}

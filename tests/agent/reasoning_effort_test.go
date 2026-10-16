package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

func TestReasoningEffortUsesConfiguredWireAndPreservesCap(t *testing.T) {
	for _, effort := range []string{"", "low", "high", "max"} {
		t.Run(effort, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["max_tokens"] != float64(8192) {
					t.Error("effort altered output cap")
				}
				if effort == "" {
					if _, present := body["reasoning_effort"]; present {
						t.Error("unset effort was injected")
					}
				} else if body["reasoning_effort"] != effort {
					t.Error("configured effort missing")
				}
				if body["thinking"].(map[string]any)["type"] != "enabled" {
					t.Error("thinking option lost")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
			}))
			defer server.Close()
			provider := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: server.URL, APIKey: "controlled", RequestOptions: model.RequestOptions{ThinkingType: "enabled", ReasoningEffort: effort}})
			stream, err := provider.Stream(context.Background(), model.Request{Model: "controlled", MaxOutputTokens: 8192})
			if err != nil {
				t.Fatal(err)
			}
			for event := range stream {
				if event.Type == "error" {
					t.Fatal(event.Err)
				}
			}
		})
	}
}

func TestReasoningEffortRejectsInvalidAndNonOpenAIOptions(t *testing.T) {
	for _, options := range []model.RequestOptions{{ReasoningEffort: "invalid"}, {ThinkingType: "invalid", ReasoningEffort: "high"}} {
		if options.Validate(model.ProtocolOpenAI) == nil {
			t.Fatal("invalid option admitted")
		}
	}
	if (model.RequestOptions{ReasoningEffort: "high"}).Validate(model.ProtocolAnthropic) == nil {
		t.Fatal("unsupported protocol admitted")
	}
	if (model.RequestOptions{ReasoningEffort: "high"}).Validate(model.ProtocolOpenAI) != nil {
		t.Fatal("effort-only option rejected")
	}
}

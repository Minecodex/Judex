package agent_test

import (
	"context"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"testing"
)

func TestOpenAICacheWriteAliasesPresenceAndCumulativeFrames(t *testing.T) {
	for _, tc := range []struct {
		name, details string
		want          *int64
	}{
		{"missing", `"cached_tokens":800`, nil},
		{"explicit-zero", `"cached_tokens":800,"cache_write_tokens":0`, model.TokenCount(0)},
		{"write", `"cached_tokens":800,"cache_write_tokens":100`, model.TokenCount(100)},
		{"creation", `"cached_tokens":800,"cache_creation_tokens":100`, model.TokenCount(100)},
		{"matching-aliases", `"cached_tokens":800,"cache_write_tokens":100,"cache_creation_tokens":100`, model.TokenCount(100)},
		{"conflicting-aliases", `"cached_tokens":800,"cache_write_tokens":0,"cache_creation_tokens":100`, nil},
		{"negative", `"cached_tokens":800,"cache_write_tokens":-1`, nil},
		{"exceeds-input", `"cached_tokens":800,"cache_write_tokens":1001`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gateway := fakeGateway(t, []string{`{"model":"m","choices":[],"usage":{"prompt_tokens":1000,"completion_tokens":1,"prompt_tokens_details":{` + tc.details + `}}}`, `{"choices":[],"usage":{"completion_tokens":5}}`}, 200)
			defer gateway.Close()
			stream, err := model.NewOpenAIProvider(model.OpenAIConfig{BaseURL: gateway.URL, APIKey: "test-key"}).Stream(context.Background(), model.Request{Model: "m"})
			if err != nil {
				t.Fatal(err)
			}
			_, _, end, failed := collect(stream)
			if failed != nil || end == nil || !end.Usage.Known() || end.Usage.Total() != 1005 {
				t.Fatalf("usage=%+v failure=%+v", end, failed)
			}
			got := end.Usage.CacheWriteTokens
			if (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
				t.Fatalf("write=%v want=%v", got, tc.want)
			}
		})
	}
}

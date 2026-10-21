package peer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

type callFailure struct {
	reason string
	err    error
}

var errTimeBudget = errors.New("technical time budget exhausted")

func (f *callFailure) Error() string { return f.err.Error() }
func (f *callFailure) Unwrap() error { return f.err }
func failureReason(err error, otherwise string) string {
	var failure *callFailure
	if errors.As(err, &failure) {
		return failure.reason
	}
	return otherwise
}

type modelResult struct {
	text  string
	calls []model.ToolCall
}

func modelTargets(provider model.Provider, name string, input, output int64) []model.Target {
	if candidates, ok := provider.(model.QuotaCandidates); ok {
		return candidates.QuotaTargets()
	}
	return []model.Target{{Provider: provider, Name: name, MaxInputTokens: input, MaxOutputTokens: output}}
}

func minimumCapacity(targets []model.Target) int64 {
	if len(targets) == 0 {
		return 0
	}
	capacity := targets[0].MaxInputTokens
	for _, target := range targets[1:] {
		capacity = min(capacity, target.MaxInputTokens)
	}
	return capacity
}

// Every real request, including compression and fallback, reserves a separate
// durable attempt. A quota rejection is not evidence of zero billed tokens.
func (e *Executor) callTargets(ctx context.Context, r *run, targets []model.Target, req model.Request) (modelResult, error) {
	outputLimit := req.MaxOutputTokens
	for index, target := range targets {
		if ctx.Err() != nil {
			return modelResult{}, ctx.Err()
		}
		if r.Calls >= 30 || r.Tokens >= 300000 || time.Since(r.BudgetStarted) > 30*time.Minute {
			return modelResult{}, &callFailure{"technical_budget", fmt.Errorf("technical model/token/time budget exhausted")}
		}
		req.Model = target.Name
		req.MaxOutputTokens = target.MaxOutputTokens
		if outputLimit > 0 {
			req.MaxOutputTokens = min(target.MaxOutputTokens, outputLimit)
		}
		raw, _ := json.Marshal(struct {
			Messages []model.Message
			Tools    []model.ToolSchema
		}{req.Messages, req.Tools})
		estimate := collab.Tokens(string(raw))
		if target.Provider == nil || target.MaxInputTokens <= 0 || req.MaxOutputTokens <= 0 || estimate+req.MaxOutputTokens > target.MaxInputTokens {
			return modelResult{}, &callFailure{"context_budget", fmt.Errorf("prompt and execution context exceed candidate model capacity")}
		}
		id := uuid.New()
		hash := sha256.Sum256(raw)
		reserved := estimate + req.MaxOutputTokens
		if err := e.reserveCall(ctx, r, id, r.SnapshotVersion, target.Name, hex.EncodeToString(hash[:]), reserved); err != nil {
			return modelResult{}, &callFailure{"technical_budget", err}
		}
		r.Calls++
		r.Tokens += reserved
		stream, err := target.Provider.Stream(ctx, req)
		var text strings.Builder
		calls := []model.ToolCall{}
		usage := model.UsageSnapshot{}
		finished, partial := false, false
		if err == nil {
		readStream:
			for {
				var event model.Event
				var open bool
				// Even a stalled stream without a finish event must release the
				// worker at the persisted deadline. Unknown usage stays reserved.
				select {
				case <-ctx.Done():
					return modelResult{}, ctx.Err()
				case event, open = <-stream:
					if !open {
						break readStream
					}
				}
				switch event.Type {
				case "textDelta":
					text.WriteString(event.TextDelta)
					partial = partial || event.TextDelta != ""
				case "toolCallDelta":
					partial = true
				case "toolCallReady":
					calls = append(calls, model.ToolCall{ID: event.ToolCallID, Name: event.ToolName, Arguments: event.ArgsJSON})
					partial = true
				case "usage":
					usage = event.Usage.Validated()
				case "finish":
					finished = true
				case "error":
					err = event.Err
					if err == nil {
						err = fmt.Errorf("model emitted an unspecified error")
					}
				}
			}
		}
		if ctx.Err() != nil {
			return modelResult{}, ctx.Err()
		}
		if err == nil && !finished {
			err = fmt.Errorf("model stream ended without completion")
		}
		if err == nil {
			output, _ := json.Marshal(calls)
			if err = e.chargeCall(ctx, r, id, usage, estimate+collab.Tokens(text.String()+string(output))); err != nil {
				return modelResult{}, err
			}
			return modelResult{text: text.String(), calls: calls}, nil
		}
		quota := model.Classify(err) == model.ErrorQuota
		if journalErr := e.failedCall(ctx, r, id, usage, quota); journalErr != nil {
			return modelResult{}, journalErr
		}
		// Never discard partially produced output/tool protocol to try another
		// model. Ordinary rate limits, auth errors and unknown errors also pause.
		if !quota || partial || index == len(targets)-1 {
			return modelResult{}, err
		}
	}
	return modelResult{}, fmt.Errorf("no available model candidates")
}

func (e *Executor) failedCall(ctx context.Context, r *run, id uuid.UUID, usage model.UsageSnapshot, quota bool) error {
	return e.Service.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error {
		if usage.Known() {
			if err := e.chargeCall(ctx, r, id, usage, 0); err != nil {
				return err
			}
		}
		state := "failed"
		if quota {
			state = "quota_failed"
		}
		raw, _ := json.Marshal(usage)
		_, err := e.Service.Pool.Exec(ctx, `UPDATE role_model_calls SET state=$2,usage=$3 WHERE id=$1`, id, state, raw)
		return err
	})
}

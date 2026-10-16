// SPDX-License-Identifier: Apache-2.0
package model

import "math"

// UsageSnapshot is a cumulative call snapshot, never a delta. A nil counter
// means unreported/invalid, while an explicit zero remains observable.
// InputTokens is the entire input including cached tokens, not the cache miss
// portion; each wire adapter is responsible for its protocol's normalization.
type UsageSnapshot struct {
	ModelName         string `json:"modelName"`
	InputTokens       *int64 `json:"inputTokens"`
	OutputTokens      *int64 `json:"outputTokens"`
	CacheReadTokens   *int64 `json:"cacheReadTokens"`
	CacheWriteTokens  *int64 `json:"cacheWriteTokens"`
	ProviderRequestID string `json:"-"`
}

func TokenCount(value int64) *int64 { return &value }

func (u UsageSnapshot) Known() bool {
	return u.InputTokens != nil && u.OutputTokens != nil &&
		*u.InputTokens >= 0 && *u.OutputTokens >= 0 &&
		*u.InputTokens <= math.MaxInt64-*u.OutputTokens
}

// Total is meaningful only when Known is true. Unknown costs are reserved by
// the runner, not represented by artificial zero usage.
func (u UsageSnapshot) Total() int64 {
	if !u.Known() {
		return 0
	}
	return *u.InputTokens + *u.OutputTokens
}

func validCounter(value *int64) *int64 {
	if value == nil || *value < 0 {
		return nil
	}
	return TokenCount(*value)
}

// Validated returns an independent snapshot and rejects impossible cache
// counts without discarding otherwise valid input/output billing counters.
func (u UsageSnapshot) Validated() UsageSnapshot {
	u.InputTokens, u.OutputTokens = validCounter(u.InputTokens), validCounter(u.OutputTokens)
	u.CacheReadTokens, u.CacheWriteTokens = validCounter(u.CacheReadTokens), validCounter(u.CacheWriteTokens)
	if u.InputTokens != nil {
		if u.CacheReadTokens != nil && *u.CacheReadTokens > *u.InputTokens {
			u.CacheReadTokens = nil
		}
		if u.CacheWriteTokens != nil && *u.CacheWriteTokens > *u.InputTokens {
			u.CacheWriteTokens = nil
		}
		if u.CacheReadTokens != nil && u.CacheWriteTokens != nil &&
			*u.CacheReadTokens > *u.InputTokens-*u.CacheWriteTokens {
			u.CacheReadTokens, u.CacheWriteTokens = nil, nil
		}
	}
	if len(u.ModelName) > 256 {
		u.ModelName = ""
	}
	if len(u.ProviderRequestID) > 256 {
		u.ProviderRequestID = ""
	}
	return u
}

func updateCounter(target **int64, value *int64) {
	if value != nil {
		*target = value
	}
}

// OpenAI-compatible gateways use both spellings. Presence, including zero, is
// meaningful; contradictory counters cannot support a cache/billing claim.
func cacheWriteCounter(written, created *int64) *int64 {
	if written != nil && created != nil && *written != *created {
		return nil
	}
	if written != nil {
		return validCounter(written)
	}
	return validCounter(created)
}

// anthropicUsage keeps the raw split input counters across streaming frames.
// Gateways omitting both cache counters retain their legacy reported input;
// a partly reported split cannot supply a trustworthy full input total.
type anthropicUsage struct {
	Input      *int64 `json:"input_tokens"`
	Output     *int64 `json:"output_tokens"`
	CacheRead  *int64 `json:"cache_read_input_tokens"`
	CacheWrite *int64 `json:"cache_creation_input_tokens"`
}

func (u *anthropicUsage) update(next *anthropicUsage) {
	updateCounter(&u.Input, next.Input)
	updateCounter(&u.Output, next.Output)
	updateCounter(&u.CacheRead, next.CacheRead)
	updateCounter(&u.CacheWrite, next.CacheWrite)
}

func (u anthropicUsage) snapshot(name, id string) UsageSnapshot {
	input := validCounter(u.Input)
	read, write := validCounter(u.CacheRead), validCounter(u.CacheWrite)
	if u.CacheRead != nil || u.CacheWrite != nil {
		input = nil
		if u.Input != nil && *u.Input >= 0 && read != nil && write != nil &&
			*read <= math.MaxInt64-*write && *u.Input <= math.MaxInt64-*read-*write {
			input = TokenCount(*u.Input + *read + *write)
		}
	}
	return (UsageSnapshot{ModelName: name, ProviderRequestID: id, InputTokens: input,
		OutputTokens: u.Output, CacheReadTokens: read, CacheWriteTokens: write}).Validated()
}

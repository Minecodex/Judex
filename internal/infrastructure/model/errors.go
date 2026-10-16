package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

// A transport 429 is a rate limit, not proof that a subscription is exhausted.
// Only explicit provider quota evidence may enable a model fallback.
func classifyGatewayError(status int, raw []byte) ErrorClass {
	var body struct {
		Error struct {
			Code    any    `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	parsed := json.Unmarshal(raw, &body) == nil
	// Kimi Code reports an exhausted five-hour window as HTTP 403. The
	// error type alone can also mean terminated access, so require both
	// explicit limit and reset evidence before overriding the auth status.
	message := strings.ToLower(body.Error.Message)
	if parsed && (status == 403 || status == 0) && strings.EqualFold(body.Error.Type, "access_terminated_error") &&
		strings.Contains(message, "reached your 5-hour usage limit") && strings.Contains(message, "quota will reset") {
		return ErrorQuota
	}
	if status == 401 || status == 403 {
		return ErrorAuth
	}
	if parsed {
		code := strings.ToLower(fmt.Sprint(body.Error.Code))
		typ := strings.ToLower(body.Error.Type)
		// Coding Plan window exhaustion was observed as 1308 with a reset
		// message. An unqualified 429/1308 is still an ordinary rate limit.
		if code == "1308" && strings.Contains(body.Error.Message, "使用上限") && strings.Contains(body.Error.Message, "重置") {
			return ErrorQuota
		}
		for _, marker := range []string{"1113", "insufficient_quota", "quota_exhausted", "billing_hard_limit_reached"} {
			if code == marker || typ == marker {
				return ErrorQuota
			}
		}
		if code == "context_length_exceeded" || typ == "context_length_exceeded" {
			return ErrorContextExceeded
		}
		if typ == "rate_limit_error" || typ == "rate_limit_exceeded" {
			return ErrorRateLimited
		}
	}
	if status == 0 {
		return ErrorUnknown
	}
	return classifyStatus(status)
}

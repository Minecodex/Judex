// SPDX-License-Identifier: Apache-2.0

// Package errors defines the single API error model from docs/plans/v1/06 §1:
// a stable machine code, an HTTP status, optional structured details, and a
// retryable flag. Handlers and domain services return *Error; transport maps
// it to the envelope {error:{code,message,details,retryable},requestId}.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

type Code string

const (
	Validation          Code = "VALIDATION_ERROR"
	UnsupportedSchema   Code = "UNSUPPORTED_SCHEMA"
	InvalidReference    Code = "INVALID_REFERENCE"
	EmailInUse          Code = "EMAIL_IN_USE"
	Unauthenticated     Code = "UNAUTHENTICATED"
	SessionExpired      Code = "SESSION_EXPIRED"
	GrantRevoked        Code = "GRANT_REVOKED"
	Forbidden           Code = "FORBIDDEN"
	HumanConfirmNeeded  Code = "HUMAN_CONFIRMATION_REQUIRED"
	NotFound            Code = "NOT_FOUND"
	VersionConflict     Code = "VERSION_CONFLICT"
	ReviewStale         Code = "REVIEW_STALE"
	DependencyChanged   Code = "DEPENDENCY_CHANGED"
	InvalidTransition   Code = "INVALID_TRANSITION"
	IdempotencyConflict Code = "IDEMPOTENCY_CONFLICT"
	SourceVersionConf   Code = "SOURCE_VERSION_CONFLICT"
	CursorExpired       Code = "EVENT_CURSOR_EXPIRED"
	RequirementUnmet    Code = "REQUIREMENT_UNMET"
	PayloadTooLarge     Code = "PAYLOAD_TOO_LARGE"
	UnsupportedMedia    Code = "UNSUPPORTED_MEDIA"
	RateLimited         Code = "RATE_LIMITED"
	BudgetExhausted     Code = "BUDGET_EXHAUSTED"
	DependencyDown      Code = "DEPENDENCY_UNAVAILABLE"
	ModelUnavailable    Code = "MODEL_UNAVAILABLE"
	SandboxUnavailable  Code = "SANDBOX_UNAVAILABLE"
	NotImplemented      Code = "NOT_IMPLEMENTED"
	MethodNotAllowed    Code = "METHOD_NOT_ALLOWED"
	Internal            Code = "INTERNAL_ERROR"
)

// httpStatus is the fixed mapping; clients must not parse messages for flow.
var httpStatus = map[Code]int{
	Validation:          http.StatusBadRequest,
	UnsupportedSchema:   http.StatusBadRequest,
	InvalidReference:    http.StatusBadRequest,
	EmailInUse:          http.StatusConflict,
	Unauthenticated:     http.StatusUnauthorized,
	SessionExpired:      http.StatusUnauthorized,
	GrantRevoked:        http.StatusUnauthorized,
	Forbidden:           http.StatusForbidden,
	HumanConfirmNeeded:  http.StatusForbidden,
	NotFound:            http.StatusNotFound,
	VersionConflict:     http.StatusConflict,
	ReviewStale:         http.StatusConflict,
	DependencyChanged:   http.StatusConflict,
	InvalidTransition:   http.StatusConflict,
	IdempotencyConflict: http.StatusConflict,
	SourceVersionConf:   http.StatusConflict,
	CursorExpired:       http.StatusConflict,
	RequirementUnmet:    http.StatusPreconditionFailed,
	PayloadTooLarge:     http.StatusRequestEntityTooLarge,
	UnsupportedMedia:    http.StatusUnsupportedMediaType,
	RateLimited:         http.StatusTooManyRequests,
	BudgetExhausted:     http.StatusTooManyRequests,
	DependencyDown:      http.StatusServiceUnavailable,
	ModelUnavailable:    http.StatusServiceUnavailable,
	SandboxUnavailable:  http.StatusServiceUnavailable,
	NotImplemented:      http.StatusNotImplemented,
	MethodNotAllowed:    http.StatusMethodNotAllowed,
	Internal:            http.StatusInternalServerError,
}

// FieldIssue points at one invalid request field (error.details.fields).
type FieldIssue struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

// Error is the canonical API error. Details must be JSON-serializable and
// must never contain secrets, prompts or full tokens.
type Error struct {
	Code         Code
	Message      string
	Details      any
	Retryable    bool
	CommitResult bool // domain conflict persisted after rolling back its proposed changes
	cause        error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// HTTPStatus resolves the transport status for the code.
func (e *Error) HTTPStatus() int {
	if status, ok := httpStatus[e.Code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// New builds an *Error with a human-readable fallback message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf is New with formatting.
func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// WithDetails attaches structured details (e.g. []FieldIssue, blockers).
func (e *Error) WithDetails(details any) *Error {
	e.Details = details
	return e
}

// WithRetryable marks transient failures safe to retry.
func (e *Error) WithRetryable(retryable bool) *Error {
	e.Retryable = retryable
	return e
}

func (e *Error) WithCommittedResult() *Error { e.CommitResult = true; return e }

// Wrap keeps the cause for logs without exposing it to clients.
func (e *Error) Wrap(err error) *Error {
	e.cause = err
	return e
}

// Fields builds validation details from path/code pairs.
func Fields(path, code string) *Error {
	return New(Validation, "request fields are invalid").WithDetails(map[string]any{
		"fields": []FieldIssue{{Path: path, Code: code}},
	})
}

// From extracts an *Error, converting unknown values into INTERNAL_ERROR.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return New(Internal, "internal error").Wrap(err)
}

// IsCode reports whether err carries the given code.
func IsCode(err error, code Code) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.Code == code
}

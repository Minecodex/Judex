// SPDX-License-Identifier: Apache-2.0

// Package auth defines the server-side Principal model for V2: who is acting, through which channel, with which narrowed
// scope. Principals are ALWAYS resolved by the server from the session
// cookie or the CLI bearer grant — a client can never assert its own actor.
package auth

import (
	"github.com/google/uuid"
)

// Kind identifies the authentication channel.
type Kind string

const (
	// KindWeb is a browser session (opaque cookie, CSRF-protected writes).
	KindWeb Kind = "web"
	// KindCLI is a device grant access token (restricted scopes, never
	// allowed to change group configuration or discussion controls).
	KindCLI Kind = "cli"
	// KindOperator is a server-side operator command (recovery/disable),
	// audited separately and never carrying business approval rights.
	KindOperator Kind = "operator"
)

// V2 CLI grant scopes. Scopes narrow what a
// grant may request; membership/binding checks are still per call.
const (
	ScopeMessagesWrite  = "messages:write"
	ScopeProjectsRead   = "projects:read"
	ScopeContextRead    = "context:read"
	ScopeMaterialsRead  = "materials:read"
	ScopeMaterialsWrite = "materials:write"
	ScopeAgentRequest   = "agent:request"
	ScopeEventsRead     = "events:read"
)

// HasScope reports whether the principal's channel allows the scope. Web
// sessions are not scope-narrowed (they rely on membership + CSRF); CLI
// grants must hold the scope explicitly.
func (p *Principal) HasScope(scope string) bool {
	if p == nil {
		return false
	}
	switch p.Kind {
	case KindWeb, KindOperator:
		return true
	case KindCLI:
		for _, s := range p.Scopes {
			if s == scope {
				return true
			}
		}
	}
	return false
}

// InProjectScope reports whether the principal's grant covers the project
// Empty CLI scope permits discovery of owned projects, never access to a project.
func (p *Principal) InProjectScope(projectID uuid.UUID) bool {
	if p == nil {
		return false
	}
	if p.Kind != KindCLI {
		return true
	}
	if len(p.ProjectScope) == 0 {
		return false
	}
	for _, id := range p.ProjectScope {
		if id == projectID {
			return true
		}
	}
	return false
}

// Principal is the resolved actor attached to every request context. UserID
// is the real human account; GrantID/SessionID identify the channel.
type Principal struct {
	AccessTokenID uuid.UUID
	Kind          Kind
	UserID        uuid.UUID
	AuthVersion   int
	SessionID     uuid.UUID // web only
	CSRFToken     string    // web only; echo for same-session writes
	GrantID       uuid.UUID // cli only
	Scopes        []string  // cli only
	ProjectScope  []uuid.UUID
}

// CanConfirmHumanDecision restricts account/device confirmation to Web sessions.
func (p *Principal) CanConfirmHumanDecision() bool {
	return p != nil && p.Kind == KindWeb
}

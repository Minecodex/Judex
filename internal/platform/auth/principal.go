// SPDX-License-Identifier: Apache-2.0

// Package auth defines the server-side Principal model from docs/plans/v1/06
// §1 and P0-02: who is acting, through which channel, with which narrowed
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
	// allowed to confirm human decisions by itself).
	KindCLI Kind = "cli"
	// KindOperator is a server-side operator command (recovery/disable),
	// audited separately and never carrying business approval rights.
	KindOperator Kind = "operator"
)

// Well-known CLI grant scopes (docs/plans/v1/07 §3). Scopes narrow what a
// grant may request; membership/binding checks are still per call.
const (
	ScopeProjectsRead     = "projects:read"
	ScopeContextRead      = "context:read"
	ScopeMaterialsRead    = "materials:read"
	ScopeMaterialsWrite   = "materials:write"
	ScopeSubmissionsWrite = "submissions:write"
	ScopeReportsWrite     = "reports:write"
	ScopeProposalsDraft   = "proposals:draft"
	ScopeAgentRequest     = "agent:request"
	ScopeEventsRead       = "events:read"
	ScopeIntentsCreate    = "intents:create"
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
// (empty CLI project scope means "all my projects" per 07 §3).
func (p *Principal) InProjectScope(projectID uuid.UUID) bool {
	if p == nil {
		return false
	}
	if p.Kind != KindCLI {
		return true
	}
	if len(p.ProjectScope) == 0 {
		return true
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
	Kind         Kind
	UserID       uuid.UUID
	AuthVersion  int
	SessionID    uuid.UUID // web only
	CSRFToken    string    // web only; echo for same-session writes
	GrantID      uuid.UUID // cli only
	Scopes       []string  // cli only
	ProjectScope []uuid.UUID
}

// CanConfirmHumanDecision reports whether this channel may execute a human
// confirmation (browser session only; CLI grants create intents instead —
// docs/plans/v1/07 §5).
func (p *Principal) CanConfirmHumanDecision() bool {
	return p != nil && p.Kind == KindWeb
}

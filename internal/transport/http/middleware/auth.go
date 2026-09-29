// SPDX-License-Identifier: Apache-2.0

// Package middleware hosts gin middlewares: principal resolution (session
// cookie today, CLI bearer in P4) and CSRF/Origin enforcement for web writes
// (docs/plans/v1/02 §2).
package middleware

import (
	"context"

	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

const (
	// ProdSessionCookie is the production cookie name (02 §2).
	ProdSessionCookie = "__Host-judex_session"
	// DevSessionCookie is the explicit development cookie for plain HTTP.
	DevSessionCookie = "judex_dev_session"

	principalKey = "judex.principal"
	csrfKey      = "judex.csrf"
)

// AuthConfig selects cookie name by environment and lists allowed origins.
type AuthConfig struct {
	Development    bool
	AllowedOrigins []string // exact match, e.g. https://judex.internal
}

func (c AuthConfig) CookieName() string {
	if c.Development {
		return DevSessionCookie
	}
	return ProdSessionCookie
}

// Resolver abstracts identity lookups for the middleware.
type Resolver interface {
	ResolveSession(ctx context.Context, secret string) (identity.ResolvedSession, error)
}

// Principal returns the resolved principal for the request (nil if anonymous).
func Principal(c *gin.Context) *auth.Principal {
	p, _ := c.Value(principalKey).(*auth.Principal)
	return p
}

// SessionMiddleware resolves the session cookie (when present) and stores the
// principal; it never rejects by itself so anonymous endpoints keep working.
func SessionMiddleware(cfg AuthConfig, resolve func(ctx context.Context, secret string) (identity.ResolvedSession, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret, err := c.Cookie(cfg.CookieName())
		if err != nil || secret == "" {
			c.Next()
			return
		}
		session, err := resolve(c.Request.Context(), secret)
		if err != nil {
			// Keep the exact error so RequireAuth can surface 401 semantics.
			c.Set("judex.auth_error", err)
			c.Next()
			return
		}
		p := &auth.Principal{
			Kind:        auth.KindWeb,
			UserID:      session.User.ID,
			AuthVersion: int(session.User.AuthVersion),
			SessionID:   session.SessionID,
			CSRFToken:   session.CSRFToken,
		}
		c.Set(principalKey, p)
		c.Set(csrfKey, session.CSRFToken)
		c.Next()
	}
}

// RequireAuth rejects anonymous requests with 401.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if Principal(c) == nil {
			err, _ := c.Value("judex.auth_error").(error)
			if err == nil {
				err = apierrors.New(apierrors.Unauthenticated, "authentication required")
			}
			respondAuthError(c, err)
			return
		}
		c.Next()
	}
}

func respondAuthError(c *gin.Context, err error) {
	apiErr := apierrors.From(err)
	details := apiErr.Details
	if details == nil {
		details = gin.H{}
	}
	if apiErr.Code == apierrors.Internal {
		apiErr = apierrors.New(apierrors.Unauthenticated, "invalid session")
	}
	c.AbortWithStatusJSON(apiErr.HTTPStatus(), gin.H{
		"error": gin.H{
			"code":      string(apiErr.Code),
			"message":   apiErr.Message,
			"details":   details,
			"retryable": apiErr.Retryable,
		},
		"requestId": c.GetString("request_id"),
	})
}

// RequireCSRF enforces the web write contract: X-CSRF-Token must match the
// session token, and an Origin header (when present) must be allowed
// (02 §2). Called after RequireAuth for mutating routes.
func RequireCSRF(cfg AuthConfig) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range cfg.AllowedOrigins {
		allowed[strings.TrimSuffix(o, "/")] = true
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		p := Principal(c)
		if p == nil || p.Kind != auth.KindWeb {
			c.Next() // CLI bearer (P4) validates through its own scopes.
			return
		}
		token := c.GetHeader("X-CSRF-Token")
		want, _ := c.Value(csrfKey).(string)
		if token == "" || want == "" || token != want {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":      "FORBIDDEN",
					"message":   "missing or invalid CSRF token",
					"retryable": false,
				},
				"requestId": c.GetString("request_id"),
			})
			return
		}
		if origin := c.GetHeader("Origin"); origin != "" && origin != "null" {
			if !allowed[origin] {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": gin.H{
						"code":      "FORBIDDEN",
						"message":   "origin not allowed",
						"retryable": false,
					},
					"requestId": c.GetString("request_id"),
				})
				return
			}
		}
		c.Next()
	}
}

// CheckOriginOnly enforces the Origin rule for anonymous writes (register,
// login, recover) where no CSRF token exists yet (02 §2).
func CheckOriginOnly(cfg AuthConfig) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range cfg.AllowedOrigins {
		allowed[strings.TrimSuffix(o, "/")] = true
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if origin := c.GetHeader("Origin"); origin != "" && origin != "null" && !allowed[origin] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":      "FORBIDDEN",
					"message":   "origin not allowed",
					"retryable": false,
				},
				"requestId": c.GetString("request_id"),
			})
			return
		}
		c.Next()
	}
}

// BearerMiddleware resolves CLI refresh-secrets (Authorization: Bearer ...)
// into KindCLI principals; scope enforcement stays per operation.
func BearerMiddleware(resolve func(ctx context.Context, secret string) (*auth.Principal, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if p := Principal(c); p != nil {
			c.Next()
			return
		}
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.Next()
			return
		}
		secret := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if secret == "" {
			c.Next()
			return
		}
		principal, err := resolve(c.Request.Context(), secret)
		if err != nil {
			c.Set("judex.auth_error", err)
			c.Next()
			return
		}
		c.Set(principalKey, principal)
		c.Next()
	}
}

// RequireScope enforces a CLI scope for the operation (06 §1); web sessions
// rely on membership checks instead.
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := Principal(c)
		if p == nil {
			c.Next()
			return
		}
		if !p.HasScope(scope) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":      "FORBIDDEN",
					"message":   "grant 缺少 scope: " + scope,
					"retryable": false,
				},
				"requestId": c.GetString("request_id"),
			})
			return
		}
		c.Next()
	}
}

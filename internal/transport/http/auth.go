// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/identity"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/transport/http/middleware"
)

// IdentityHandlers wires register/login/session/logout/logout-all to the
// contract operationIds (docs/plans/v1/06 §2).
type IdentityHandlers struct {
	Service      *identity.Service
	Auth         middleware.AuthConfig
	secureCookie bool
}

func NewIdentityHandlers(service *identity.Service, auth middleware.AuthConfig, development bool) *IdentityHandlers {
	return &IdentityHandlers{Service: service, Auth: auth, secureCookie: !development}
}

func (h *IdentityHandlers) Register(spec *SpecRouter) {
	spec.Register("register", h.register)
	spec.Register("login", h.login)
	spec.Register("getSession", h.getSession)
	spec.Register("logout", h.logout)
	spec.Register("logoutAll", h.logoutAll)
	spec.Register("recoverPassword", h.recoverPassword)
	spec.Register("listSessions", withAuth(h.listSessions))
	spec.Register("revokeSession", withAuth(h.revokeSession))
}

func clientIP(c *gin.Context) string {
	// Trusted proxies are explicitly configured (plans/v1/02 §2); gin's
	// ClientIP honors only those, so this is safe to use for rate buckets.
	return c.ClientIP()
}

func (h *IdentityHandlers) setSessionCookie(c *gin.Context, secret string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	cookie := &http.Cookie{
		Name:     h.Auth.CookieName(),
		Value:    secret,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
	// __Host- prefix requires no Domain (and Secure) — never set Domain.
	http.SetCookie(c.Writer, cookie)
}

func (h *IdentityHandlers) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: h.Auth.CookieName(), Value: "", Path: "/",
		HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

type registerRequest struct {
	DisplayName string `json:"displayName" binding:"required"`
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

func (h *IdentityHandlers) register(c *gin.Context) {
	var req registerRequest
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	user, token, err := h.Service.Register(c.Request.Context(), req.DisplayName, req.Email, req.Password, clientIP(c))
	if err != nil {
		respond{}.error(c, err)
		return
	}
	h.setSessionCookie(c, token.Secret, token.ExpiresAt)
	respond{}.created(c, gin.H{"user": user, "sessionExpiresAt": token.ExpiresAt})
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *IdentityHandlers) login(c *gin.Context) {
	var req loginRequest
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	user, token, err := h.Service.Login(c.Request.Context(), req.Email, req.Password, clientIP(c))
	if err != nil {
		if apierrors.IsCode(err, apierrors.RateLimited) {
			if d, ok := err.(*apierrors.Error); ok {
				if details, ok := d.Details.(map[string]any); ok {
					if secs, ok := details["retryAfterSeconds"].(int); ok && secs > 0 {
						c.Header("Retry-After", strconv.Itoa(secs))
					}
				}
			}
		}
		respond{}.error(c, err)
		return
	}
	h.setSessionCookie(c, token.Secret, token.ExpiresAt)
	respond{}.ok(c, gin.H{"user": user, "sessionExpiresAt": token.ExpiresAt})
}

func (h *IdentityHandlers) getSession(c *gin.Context) {
	p := principalFrom(c)
	if p == nil {
		if err, ok := c.Value("judex.auth_error").(error); ok {
			respond{}.error(c, err)
			return
		}
		respond{}.error(c, apierrors.New(apierrors.Unauthenticated, "authentication required"))
		return
	}
	// The session middleware already verified expiry/auth_version; re-query
	// only the mutable projection fields.
	user, expiresAt, err := h.Service.SessionProjection(c.Request.Context(), p.SessionID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"user": user, "expiresAt": expiresAt, "csrfToken": p.CSRFToken})
}

func (h *IdentityHandlers) logout(c *gin.Context) {
	p := principalFrom(c)
	// Even an expired/absent session clears the local cookie (06 §2).
	if p != nil {
		if err := h.Service.Logout(c.Request.Context(), p.SessionID); err != nil {
			respond{}.error(c, err)
			return
		}
	}
	h.clearSessionCookie(c)
	respond{}.ok(c, gin.H{"revoked": true})
}

func (h *IdentityHandlers) logoutAll(c *gin.Context) {
	p := principalFrom(c)
	if p == nil {
		respond{}.error(c, apierrors.New(apierrors.Unauthenticated, "authentication required"))
		return
	}
	var req struct {
		ExpectedAuthVersion int64 `json:"expectedAuthVersion" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	sessions, grants, err := h.Service.LogoutAll(c.Request.Context(), p.UserID, p.SessionID, req.ExpectedAuthVersion)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"revokedSessions": sessions, "revokedGrants": grants})
}

func (h *IdentityHandlers) recoverPassword(c *gin.Context) {
	var req struct {
		RecoveryCode string `json:"recoveryCode" binding:"required"`
		NewPassword  string `json:"newPassword" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if err := h.Service.RecoverPassword(c.Request.Context(), req.RecoveryCode, req.NewPassword); err != nil {
		respond{}.error(c, err)
		return
	}
	h.clearSessionCookie(c)
	respond{}.ok(c, gin.H{"completed": true})
}

func (h *IdentityHandlers) listSessions(c *gin.Context) {
	p := principalFrom(c)
	sessions, err := h.Service.ListSessions(c.Request.Context(), p.UserID, p.SessionID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(sessions, nil))
}

func (h *IdentityHandlers) revokeSession(c *gin.Context) {
	p := principalFrom(c)
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("sessionId", "invalid"))
		return
	}
	if err := h.Service.RevokeSession(c.Request.Context(), p.UserID, sessionID); err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"revoked": true})
}

package httptransport

import "github.com/gin-gonic/gin"

func (h *IdentityHandlers) updateMe(c *gin.Context) {
	var req struct {
		ExpectedVersion int64   `json:"expectedVersion"`
		DisplayName     *string `json:"displayName"`
		Locale          *string `json:"locale"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	out, err := h.Service.UpdateProfile(c.Request.Context(), principalFrom(c).UserID, req.ExpectedVersion, req.DisplayName, req.Locale)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, out)
}
func (h *IdentityHandlers) changePassword(c *gin.Context) {
	var req struct {
		Current string `json:"currentPassword"`
		New     string `json:"newPassword"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	user, token, err := h.Service.ChangePassword(c.Request.Context(), principalFrom(c).UserID, req.Current, req.New)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	h.setSessionCookie(c, token.Secret, token.ExpiresAt)
	respond{}.ok(c, gin.H{"user": user, "sessionExpiresAt": token.ExpiresAt})
}

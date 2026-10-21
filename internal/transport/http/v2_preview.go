package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collab"
	"io"
	"net/http"
	"strings"
)

func mountV2Preview(router *gin.Engine, service *collab.Service) {
	if service == nil {
		return
	}
	router.GET("/file-preview/:token/:versionId/*entry", func(c *gin.Context) {
		version, e := uuid.Parse(c.Param("versionId"))
		if e != nil {
			respond{}.error(c, errAPINotFound)
			return
		}
		entry := strings.TrimPrefix(c.Param("entry"), "/")
		out, e := service.PreviewEntry(c.Request.Context(), c.Param("token"), version, entry)
		if e != nil {
			respond{}.error(c, e)
			return
		}
		defer out.Body.Close()
		c.Header("Cache-Control", "private, no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "sandbox allow-scripts; default-src 'self' data: blob:; script-src 'self' 'unsafe-inline' blob:; style-src 'self' 'unsafe-inline'; connect-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none'")
		c.Header("Content-Type", out.Mime)
		c.Header("Content-Disposition", "inline")
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, out.Body)
	})
}

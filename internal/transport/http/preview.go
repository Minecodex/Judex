package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/material"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"io"
	"net/url"
	"strings"
)

func mountPreview(router *gin.Engine, service *material.Service, origin string) {
	router.GET("/preview/:token/:versionId/*entry", func(c *gin.Context) {
		configured, err := url.Parse(origin)
		if err != nil || configured.Host == "" || !strings.EqualFold(c.Request.Host, configured.Host) || service == nil {
			respond{}.error(c, apierrors.New(apierrors.NotFound, "preview origin unavailable"))
			return
		}
		version, err := uuid.Parse(c.Param("versionId"))
		if err != nil {
			respond{}.error(c, apierrors.Fields("versionId", "uuid"))
			return
		}
		next, err := service.ExchangePreview(c.Request.Context(), c.Param("token"), version)
		if err != nil {
			respond{}.error(c, err)
			return
		}
		if next != "" {
			c.Header("Referrer-Policy", "no-referrer")
			c.Header("Cache-Control", "no-store")
			c.Redirect(303, "/preview/"+url.PathEscape(next)+"/"+version.String()+c.Param("entry"))
			return
		}
		entry := strings.TrimPrefix(c.Param("entry"), "/")
		if entry == "" {
			entry, err = service.PreviewEntrypoint(c.Request.Context(), c.Param("token"), version)
			if err != nil {
				respond{}.error(c, err)
				return
			}
		}
		body, mime, err := service.PreviewOpen(c.Request.Context(), c.Param("token"), version, entry)
		if err != nil {
			respond{}.error(c, err)
			return
		}
		defer body.Close()
		c.Header("Content-Security-Policy", "sandbox allow-scripts; default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-src 'none'")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Type", mime)
		c.Status(200)
		_, _ = io.Copy(c.Writer, body)
	})
}

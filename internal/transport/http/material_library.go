package httptransport

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"io"
	"strconv"
	"strings"
)

func (h *MaterialHandlers) libraryIds(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	project, e := projectParam(c)
	if e != nil {
		respond{}.error(c, e)
		return project, uuid.Nil, false
	}
	version, e := uuid.Parse(c.Param("versionId"))
	if e != nil {
		respond{}.error(c, apierrors.Fields("versionId", "uuid"))
		return project, version, false
	}
	return project, version, true
}
func (h *MaterialHandlers) usages(c *gin.Context) {
	p, v, ok := h.libraryIds(c)
	if !ok {
		return
	}
	rows, e := h.Materials.Usages(c.Request.Context(), principalFrom(c).UserID, p, v)
	if e != nil {
		respond{}.error(c, e)
		return
	}
	respond{}.ok(c, respond{}.list(c, rows, nil))
}
func (h *MaterialHandlers) filePreview(prepare bool) Handler {
	return func(c *gin.Context) {
		p, v, ok := h.libraryIds(c)
		if !ok {
			return
		}
		out, e := h.Materials.FilePreview(c.Request.Context(), principalFrom(c).UserID, p, v)
		if prepare {
			out, e = h.Materials.PrepareFilePreview(c.Request.Context(), principalFrom(c).UserID, p, v, c.Query("retry") == "true")
		}
		if e != nil {
			respond{}.error(c, e)
			return
		}
		respond{}.ok(c, out)
	}
}
func (h *MaterialHandlers) previewContent(c *gin.Context) {
	p, v, ok := h.libraryIds(c)
	if !ok {
		return
	}
	body, info, e := h.Materials.PreviewContent(c.Request.Context(), principalFrom(c).UserID, p, v)
	if e != nil {
		respond{}.error(c, e)
		return
	}
	defer body.Close()
	serveMaterialStream(c, body, info.Size, info.Mime, "inline")
}
func (h *MaterialHandlers) deleteLibrary(c *gin.Context) {
	p := principalFrom(c)
	if p.Kind == "cli" {
		respond{}.error(c, apierrors.New(apierrors.HumanConfirmNeeded, "delete through browser"))
		return
	}
	project, e := projectParam(c)
	if e != nil {
		respond{}.error(c, e)
		return
	}
	id, e := uuid.Parse(c.Param("materialId"))
	if e != nil {
		respond{}.error(c, apierrors.Fields("materialId", "uuid"))
		return
	}
	var req struct {
		ExpectedCurrentVersionID uuid.UUID `json:"expectedCurrentVersionId" binding:"required"`
	}
	if e = bindJSON(c, &req); e != nil {
		respond{}.error(c, e)
		return
	}
	if e = h.Materials.DeleteFromLibrary(c.Request.Context(), p.UserID, project, id, req.ExpectedCurrentVersionID); e != nil {
		respond{}.error(c, e)
		return
	}
	respond{}.ok(c, gin.H{"deleted": true})
}

// The immutable stream is bounded to the authorized file; Range never touches
// another object or bypasses membership. Readers also close on cancellation.
func serveMaterialStream(c *gin.Context, body io.Reader, size int64, mime, disposition string) {
	c.Header("Content-Type", mime)
	c.Header("Content-Disposition", disposition)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "private, no-cache")
	start, end := int64(0), size-1
	status := 200
	if header := c.GetHeader("Range"); header != "" {
		valid := strings.HasPrefix(header, "bytes=") && !strings.Contains(header, ",")
		parts := strings.SplitN(strings.TrimPrefix(header, "bytes="), "-", 2)
		if len(parts) != 2 {
			valid = false
		} else {
			if parts[0] == "" {
				n, e := strconv.ParseInt(parts[1], 10, 64)
				if e != nil || n < 1 {
					valid = false
				} else {
					start = max(0, size-n)
				}
			} else {
				n, e := strconv.ParseInt(parts[0], 10, 64)
				if e != nil || n < 0 {
					valid = false
				} else {
					start = n
				}
				if parts[1] != "" {
					n, e = strconv.ParseInt(parts[1], 10, 64)
					if e != nil || n < 0 {
						valid = false
					} else {
						end = min(n, size-1)
					}
				}
			}
		}
		if !valid || start >= size || start > end {
			c.Header("Content-Range", fmt.Sprintf("bytes */%d", size))
			c.AbortWithStatus(416)
			return
		}
		status = 206
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	}
	if start > 0 {
		if _, e := io.CopyN(io.Discard, body, start); e != nil {
			respond{}.error(c, apierrors.New(apierrors.DependencyDown, "file stream unavailable"))
			return
		}
	}
	c.Header("Content-Length", strconv.FormatInt(max(0, end-start+1), 10))
	c.Status(status)
	_, _ = io.CopyN(c.Writer, body, max(0, end-start+1))
}

package httptransport

import (
	"fmt"
	"github.com/gin-gonic/gin"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"io"
	"strconv"
	"strings"
)

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

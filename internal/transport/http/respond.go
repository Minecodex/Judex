// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// respond builds the canonical success envelope from docs/plans/v1/06 §1:
// {data, meta:{requestId, serverTime}}; commands add meta.affected.
type respond struct{}

func (respond) data(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{
		"data": data,
		"meta": gin.H{
			"requestId":  c.GetString(requestIDKey),
			"serverTime": time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
}

func (respond) created(c *gin.Context, data any) { respond{}.data(c, http.StatusCreated, data) }
func (respond) ok(c *gin.Context, data any)      { respond{}.data(c, http.StatusOK, data) }

// affected attaches the command result list [{type,id,version}] to the response.
func (r respond) affected(c *gin.Context, status int, data any, affected []gin.H) {
	c.JSON(status, gin.H{
		"data": data,
		"meta": gin.H{
			"requestId":  c.GetString(requestIDKey),
			"serverTime": time.Now().UTC().Format(time.RFC3339Nano),
			"affected":   affected,
		},
	})
}

// error writes the error envelope {error:{code,message,details,retryable},requestId}.
func (respond) error(c *gin.Context, err error) {
	apiErr := apierrors.From(err)
	if apiErr.Code == apierrors.Internal {
		c.AbortWithStatusJSON(apiErr.HTTPStatus(), gin.H{
			"error": gin.H{
				"code":      string(apierrors.Internal),
				"message":   "internal error",
				"retryable": false,
			},
			"requestId": c.GetString(requestIDKey),
		})
		return
	}
	c.AbortWithStatusJSON(apiErr.HTTPStatus(), gin.H{
		"error": gin.H{
			"code":      string(apiErr.Code),
			"message":   apiErr.Message,
			"details":   apiErr.Details,
			"retryable": apiErr.Retryable,
		},
		"requestId": c.GetString(requestIDKey),
	})
}

// list is the paginated data shape {items,nextCursor}.
func (respond) list(items any, nextCursor *string) gin.H {
	cursor := ""
	if nextCursor != nil {
		cursor = *nextCursor
	}
	return gin.H{"items": items, "nextCursor": cursor}
}

// accepted writes a 202 envelope for async-accepted commands.
func (respond) accepted(c *gin.Context, data any, _ []gin.H) {
	respond{}.data(c, http.StatusAccepted, data)
}

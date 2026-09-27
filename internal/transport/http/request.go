// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// maxJSONBody bounds request bodies for ordinary JSON endpoints (uploads use
// their own streaming routes).
const maxJSONBody = 1 << 20

// bindJSON decodes a strict JSON body: unknown fields are rejected so typos
// cannot silently drop required input (docs/plans/v1/06 §8).
func bindJSON(c *gin.Context, target any) error {
	if ct := c.GetHeader("Content-Type"); ct != "" {
		mime := ct
		if i := indexByte(mime, ';'); i >= 0 {
			mime = mime[:i]
		}
		if trimSpace(mime) != "application/json" {
			return apierrors.New(apierrors.UnsupportedMedia, "Content-Type must be application/json")
		}
	}
	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBody)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apierrors.New(apierrors.PayloadTooLarge, "request body too large")
		}
		return apierrors.New(apierrors.Validation, "invalid request body").Wrap(err)
	}
	if decoder.More() {
		return apierrors.New(apierrors.Validation, "unexpected trailing content")
	}
	return nil
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// principalFrom returns the middleware-resolved principal or nil.
func principalFrom(c *gin.Context) *auth.Principal {
	if p, ok := c.Value("judex.principal").(*auth.Principal); ok {
		return p
	}
	return nil
}

// ensure io import stays referenced if body handling changes.
var _ io.Reader = (io.Reader)(nil)

// withAuth wraps a handler that requires an authenticated principal; it is
// the transport-level guard for every security-marked operation (06 §1).
func withAuth(h Handler) Handler {
	return func(c *gin.Context) {
		if principalFrom(c) == nil {
			if err, ok := c.Value("judex.auth_error").(error); ok {
				respond{}.error(c, err)
				return
			}
			respond{}.error(c, apierrors.New(apierrors.Unauthenticated, "authentication required"))
			return
		}
		h(c)
	}
}

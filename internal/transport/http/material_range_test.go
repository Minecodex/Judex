package httptransport

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaterialRanges(t *testing.T) {
	for _, tc := range []struct {
		rangeHeader        string
		status             int
		body, contentRange string
	}{{"", 200, "abcdef", ""}, {"bytes=1-3", 206, "bcd", "bytes 1-3/6"}, {"bytes=-2", 206, "ef", "bytes 4-5/6"}, {"bytes=3-", 206, "def", "bytes 3-5/6"}, {"bytes=6-", 416, "", "bytes */6"}, {"bytes=3-1", 416, "", "bytes */6"}, {"bytes=1-2,4-5", 416, "", "bytes */6"}} {
		t.Run(tc.rangeHeader, func(t *testing.T) {
			writer := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest("GET", "/", nil)
			c.Request.Header.Set("Range", tc.rangeHeader)
			serveMaterialStream(c, strings.NewReader("abcdef"), 6, "text/plain", "inline")
			if writer.Code != tc.status || writer.Body.String() != tc.body || writer.Header().Get("Content-Range") != tc.contentRange {
				t.Fatalf("range response: %d %q %v", writer.Code, writer.Body.String(), writer.Header())
			}
		})
	}
}

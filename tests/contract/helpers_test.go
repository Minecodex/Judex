// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func performJSON(t *testing.T, router *gin.Engine, method, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec.Code, rec.Body.String()
}

// TestEmbeddedSpecInSync guards the generated-artifact discipline: the spec
// copy embedded into the binary must be byte-identical to api/openapi.yaml,
// otherwise `make generate` was not run after a contract edit.
func TestEmbeddedSpecInSync(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := os.ReadFile(filepath.Join("..", "..", "internal", "gen", "api", "spec", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != string(embedded) {
		t.Fatal("internal/gen/api/spec/openapi.yaml differs from api/openapi.yaml - run `make generate`")
	}
}

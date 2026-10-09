// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	transport "github.com/kakj-go/Judex/internal/transport/http"
)

// TestSpecRouterParity asserts that every operation declared in the embedded
// OpenAPI contract is mounted on the real router with the same method and
// path shape — and that the router exposes no /api/v1 routes beyond the
// contract. Unimplemented operations must answer 501, never silently 404.
func TestSpecRouterParity(t *testing.T) {
	router, err := transport.NewRouter(transport.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	live := map[string]bool{}
	for _, r := range router.Routes() {
		if !strings.HasPrefix(r.Path, "/api/v1/") {
			continue
		}
		live[r.Method+" "+r.Path] = true
	}
	specRoutes, err := transport.SpecRouteTableForTest()
	if err != nil {
		t.Fatal(err)
	}
	if len(specRoutes) < 100 {
		t.Fatalf("contract too small: %d routes", len(specRoutes))
	}
	for _, row := range specRoutes {
		if !live[row] {
			t.Errorf("spec operation not mounted: %s", row)
		}
	}
	specSet := map[string]bool{}
	for _, row := range specRoutes {
		specSet[row] = true
	}
	var extra []string
	for row := range live {
		if !specSet[row] {
			extra = append(extra, row)
		}
	}
	sort.Strings(extra)
	for _, row := range extra {
		t.Errorf("router route missing from contract: %s", row)
	}
}

// TestNotImplementedIs501 keeps the "explicit capability off" rule: calling
// any not-yet-implemented business operation returns 501 NOT_IMPLEMENTED in
// the canonical error envelope, not an empty 200 or a 404.
func TestNotImplementedIs501(t *testing.T) {
	var draining atomic.Bool
	router, err := transport.NewRouter(transport.Options{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Draining: &draining,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, body := performJSON(t, router, "POST", "/api/v1/auth/register")
	if code != 501 || !strings.Contains(body, "NOT_IMPLEMENTED") {
		t.Fatalf("register before P1 must be 501, got %d %s", code, body)
	}
}

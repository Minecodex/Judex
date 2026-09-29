// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"log/slog"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	apigen "github.com/kakj-go/Judex/internal/gen/api"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

const apiPrefix = "/api/v1"

// Handler serves one operation identified by its stable operationId.
// Handlers attach via Register; anything not registered answers 501
// NOT_IMPLEMENTED until its stage lands (docs/plans/v1/06 §8).
type Handler func(*gin.Context)

// SpecRouter registers every contract operation from the embedded OpenAPI
// document so spec<->router parity holds by construction; the contract test
// still verifies it against the live route table.
type SpecRouter struct {
	doc          *openapi3.T
	handlers     map[string]Handler
	logger       *slog.Logger
	CommandPool  *postgres.Pool
	requiredKeys map[string]bool
}

func NewSpecRouter(logger *slog.Logger) (*SpecRouter, error) {
	doc, err := apigen.Load()
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "openapi contract failed to load").Wrap(err)
	}
	return &SpecRouter{doc: doc, handlers: map[string]Handler{}, requiredKeys: map[string]bool{}, logger: logger}, nil
}

// Register attaches a real handler for one operationId. It fails loudly on
// unknown ids so contract drift cannot silently drop a route.
func (s *SpecRouter) Register(operationID string, h Handler) {
	s.handlers[operationID] = h
}

// notImplemented is the default until a stage implements the operation.
func (s *SpecRouter) notImplemented(c *gin.Context) {
	respond{}.error(c, apierrors.New(apierrors.NotImplemented,
		"This business module is not implemented yet."))
}

// Mount registers all spec operations under /api/v1 on the gin engine.
func (s *SpecRouter) Mount(router *gin.RouterGroup) {
	paths := make([]string, 0, len(s.doc.Paths.Map()))
	for p := range s.doc.Paths.Map() {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, specPath := range paths {
		item := s.doc.Paths.Map()[specPath]
		ginPath := apiPrefix + specPathToStrings(specPath)
		for method, op := range item.Operations() {
			for _, param := range append(append(openapi3.Parameters{}, item.Parameters...), op.Parameters...) {
				if param.Value != nil && param.Value.Name == "Idempotency-Key" && param.Value.Required {
					s.requiredKeys[op.OperationID] = true
				}
			}
			handler, ok := s.handlers[op.OperationID]
			if !ok {
				handler = s.notImplemented
			}
			withSchema := func(c *gin.Context) {
				if s.CommandPool != nil && op.RequestBody != nil && op.RequestBody.Value != nil {
					if content := op.RequestBody.Value.Content.Get("application/json"); content != nil && content.Schema != nil {
						c.Set("judex.requestSchema", content.Schema.Value)
					}
				}
				handler(c)
			}
			router.Handle(strings.ToUpper(method), ginPath, gin.HandlerFunc(s.guardOperation(op.OperationID, withSchema)))
		}
	}
}

// RouteTable returns "METHOD path" for every mounted operation, used by the
// contract parity test.
func (s *SpecRouter) RouteTable() []string {
	rows := make([]string, 0)
	paths := make([]string, 0, len(s.doc.Paths.Map()))
	for p := range s.doc.Paths.Map() {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, specPath := range paths {
		item := s.doc.Paths.Map()[specPath]
		for method := range item.Operations() {
			rows = append(rows, strings.ToUpper(method)+" "+apiPrefix+specPathToStrings(specPath))
		}
	}
	return rows
}

// SpecRouteTableForTest exposes the contract route table for parity tests.
func SpecRouteTableForTest() ([]string, error) {
	s, err := NewSpecRouter(nil)
	if err != nil {
		return nil, err
	}
	return s.RouteTable(), nil
}

// specPathToStrings converts /projects/{id}/runs to /projects/:id/runs.
func specPathToStrings(p string) string {
	out := strings.ReplaceAll(p, "{", ":")
	return strings.ReplaceAll(out, "}", "")
}

// ValidateRegistration makes an omitted implementation a startup error in the
// database-backed app. Scaffold and focused handler fixtures remain possible.
func (s *SpecRouter) ValidateRegistration() error {
	expected := map[string]bool{}
	for _, path := range s.doc.Paths.Map() {
		for _, op := range path.Operations() {
			expected[op.OperationID] = true
			if s.handlers[op.OperationID] == nil {
				return apierrors.Newf(apierrors.Internal, "missing handler: %s", op.OperationID)
			}
		}
	}
	for op := range s.handlers {
		if !expected[op] {
			return apierrors.Newf(apierrors.Internal, "handler outside contract: %s", op)
		}
	}
	return nil
}

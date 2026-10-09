package contract

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestSpecLoadsAndValidates guards the contract source: it must parse, resolve
// every external component reference, and declare an operationId everywhere.
func TestSpecLoadsAndValidates(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("validate: %v", err)
	}
	ops := 0
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == "" {
				t.Errorf("%s %s: missing operationId", method, path)
				continue
			}
			if len(op.Tags) == 0 {
				t.Errorf("%s %s: missing tags", method, path)
			}
			ops++
		}
	}
	if ops < 100 {
		t.Fatalf("expected >=100 operations, got %d", ops)
	}
	t.Logf("paths=%d operations=%d", len(doc.Paths.Map()), ops)
}

package document

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewValidationReadsActualFiles(t *testing.T) {
	for _, name := range []string{"acceptance.pdf", "chinese-cid.pdf"} {
		raw, err := os.ReadFile(filepath.Join("../../../tests/fixtures/materials", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidatePDF(context.Background(), raw); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	raw, err := os.ReadFile("../../../tests/fixtures/materials/acceptance.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateImage(raw); err != nil {
		t.Fatal(err)
	}
	if err := ValidateImage(raw[:33]); err == nil {
		t.Fatal("truncated PNG accepted")
	}
	if err := ValidatePDF(context.Background(), []byte("%PDF-1.7\n1 0 obj\n<< /Type /Catalog >>\nendobj\n")); err == nil {
		t.Fatal("truncated PDF accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ValidatePDF(ctx, []byte("%PDF-1.7\n")); err == nil {
		t.Fatal("cancelled PDF validation succeeded")
	}
}

package collab

import (
	"context"
	"github.com/google/uuid"
	"testing"
)

func TestTextFormatRecognition(t *testing.T) {
	s := &Service{}
	for _, name := range []string{"budget.csv", "rows.tsv", "utf16.txt", "pipeline.yaml", "diagram.mmd", "view.tsx", "notes.markdown"} {
		data, text, e := s.previewData(context.Background(), uuid.New(), name, "application/octet-stream", []byte("a\x00b"))
		if e != nil || data["kind"] != "text" || text != "" {
			t.Fatalf("%s: %v %q %v", name, data, text, e)
		}
	}
}

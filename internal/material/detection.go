package material

import (
	"bytes"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// The sender's MIME is descriptive only. Strong signatures override it; Office
// ZIP containers are identified by their extension and validated in conversion.
func detectedMime(name string, probe []byte) string {
	detected := http.DetectContentType(probe)
	if bytes.Contains(probe, []byte("%PDF-")) {
		return "application/pdf"
	}
	ext := strings.ToLower(filepath.Ext(name))
	if bytes.HasPrefix(probe, []byte{'P', 'K', 3, 4}) || bytes.HasPrefix(probe, []byte{'P', 'K', 5, 6}) {
		if ext == ".docx" || ext == ".xlsx" || ext == ".pptx" {
			if m := mime.TypeByExtension(ext); m != "" {
				return m
			}
		}
		return "application/zip"
	}
	if strings.HasPrefix(detected, "text/plain") {
		if ext == ".json" {
			return "application/json"
		}
		if ext == ".md" {
			return "text/markdown; charset=utf-8"
		}
	}
	return detected
}

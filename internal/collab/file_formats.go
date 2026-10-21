package collab

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/document"
)

func inExtensions(ext, list string) bool { return strings.Contains(" "+list+" ", " "+ext+" ") }
func textExtension(ext string) bool {
	return inExtensions(ext, ".md .markdown .txt .log .csv .tsv .json .jsonl .yaml .yml .xml .toml .ini .go .py .js .ts .tsx .jsx .java .rs .sql .sh .bash .css .scss .html .htm .vue .svelte .c .h .cpp .hpp .rb .php .swift .kt .kts .mmd .mermaid .conf .env .dockerfile .r .lua .bat .ps1")
}
func (s *Service) convertPreview(ctx context.Context, name, format string, raw []byte) ([]byte, string, error) {
	if s.ConverterURL == "" {
		return nil, "", fmt.Errorf("preview converter unavailable")
	}
	req, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(s.ConverterURL, "/")+"/convert?name="+url.QueryEscape(name)+"&format="+format, bytes.NewReader(raw))
	if e != nil {
		return nil, "", e
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, e := (&http.Client{Timeout: 95 * time.Second}).Do(req)
	if e != nil {
		return nil, "", e
	}
	defer resp.Body.Close()
	converted, e := io.ReadAll(io.LimitReader(resp.Body, MaxFileSize+1))
	if e != nil {
		return nil, "", e
	}
	if resp.StatusCode != 200 || int64(len(converted)) > MaxFileSize {
		return nil, "", fmt.Errorf("preview conversion failed")
	}
	return converted, resp.Header.Get("Content-Type"), nil
}
func (s *Service) convertedPreview(ctx context.Context, id uuid.UUID, name, format, kind string, raw []byte) (map[string]any, error) {
	converted, media, e := s.convertPreview(ctx, name, format, raw)
	if e != nil {
		return nil, e
	}
	if format == "pdf" {
		if e = document.ValidatePDF(ctx, converted); e != nil {
			return nil, e
		}
	}
	if format == "png" {
		if e = document.ValidateImage(converted); e != nil {
			return nil, e
		}
	}
	sum := sha256.Sum256(converted)
	hash := hex.EncodeToString(sum[:])
	key := "v2/versions/" + id.String() + "/preview/" + hash + "." + format
	if e = s.Objects.Put(ctx, key, bytes.NewReader(converted), int64(len(converted)), media); e != nil {
		return nil, e
	}
	return map[string]any{"kind": kind, "objectKey": key, "size": len(converted), "sha256": hash, "mime": media}, nil
}
func (s *Service) previewData(ctx context.Context, id uuid.UUID, name, media string, raw []byte) (map[string]any, string, error) {
	ext := strings.ToLower(path.Ext(name))
	data := map[string]any{}
	text := ""
	var e error
	switch {
	case inExtensions(ext, ".docx .xlsx .pptx .doc .xls .ppt .odt .ods .odp .rtf"):
		data, e = s.convertedPreview(ctx, id, name, "pdf", "pdf", raw)
		if e == nil && inExtensions(ext, ".xlsx .xls .ods") {
			data["reader"] = "spreadsheet"
		}
	case ext == ".svg":
		e = document.ValidateSVG(raw)
		data = map[string]any{"kind": "image", "mime": "image/svg+xml"}
	case inExtensions(ext, ".bmp .avif"):
		data, e = s.convertedPreview(ctx, id, name, "png", "image", raw)
	case inExtensions(ext, ".mp3 .wav .ogg .m4a .flac .aac .opus"):
		data, e = s.convertedPreview(ctx, id, name, "mp3", "audio", raw)
	case inExtensions(ext, ".mp4 .webm .mov .mkv .m4v"):
		data, e = s.convertedPreview(ctx, id, name, "mp4", "video", raw)
	case inExtensions(ext, ".tar .gz .tgz .7z"):
		var converted []byte
		converted, _, e = s.convertPreview(ctx, name, "zip", raw)
		if e == nil {
			data, e = s.expandBundle(ctx, id, converted)
			if e == nil {
				data["kind"] = "archive"
			}
		}
	case media == "application/pdf":
		e = document.ValidatePDF(ctx, raw)
		data["kind"] = "pdf"
	case strings.HasPrefix(media, "image/"):
		e = document.ValidateImage(raw)
		data["kind"] = "image"
	case textExtension(ext) || strings.HasPrefix(media, "text/") || media == "application/json":
		data["kind"] = "text"
		if utf8.Valid(raw) && !bytes.ContainsRune(raw, 0) {
			text = string(raw)
		}
		if ext == ".html" || ext == ".htm" {
			data["kind"] = "html"
			data["entrypoint"] = "index.html"
			data["raw"] = true
		}
	case media == "application/zip":
		data, e = s.expandBundle(ctx, id, raw)
	default:
		data["kind"] = "unsupported"
	}
	return data, text, e
}

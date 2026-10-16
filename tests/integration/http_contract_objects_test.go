package integrationtest_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/kakj-go/Judex/internal/infrastructure/objectstore"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	"github.com/kakj-go/Judex/internal/material"
)

// A controlled S3 protocol seam for HTTP schema tests. Real SeaweedFS byte,
// version and recovery evidence is collected by the separate production E2E.
func contractObjects(t *testing.T) string {
	var mu sync.Mutex
	files := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "PUT":
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read failed", 500)
				return
			}
			files[r.URL.Path] = raw
			w.Header().Set("ETag", `"contract-object"`)
			w.WriteHeader(200)
		case "GET", "HEAD":
			raw, ok := files[r.URL.Path]
			if !ok {
				http.Error(w, "missing", 404)
				return
			}
			w.Header().Set("Content-Length", fmt.Sprint(len(raw)))
			w.Header().Set("Content-Type", "application/octet-stream")
			if r.Method == "GET" {
				_, _ = w.Write(raw)
			}
		case "DELETE":
			delete(files, r.URL.Path)
			w.WriteHeader(204)
		default:
			http.Error(w, "unsupported", 405)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL
}

type contractStreamRecorder struct {
	*httptest.ResponseRecorder
	cancel             context.CancelFunc
	flushes, stopAfter int
}

func (r *contractStreamRecorder) Flush() {
	r.ResponseRecorder.Flush()
	r.flushes++
	if r.flushes >= r.stopAfter {
		r.cancel()
	}
}

func (s *contractScenario) files(pool *postgres.Pool, endpoint string) {
	c, p := s.call, s.prefix
	text := []byte("Contract immutable original\n")
	sum := sha256.Sum256(text)
	sha := hex.EncodeToString(sum[:])
	upload := c("POST", p+"/uploads", map[string]any{"name": "contract.txt", "kind": "file", "mime": "text/plain", "size": len(text), "sha256": sha})
	path := p + "/uploads/" + upload["id"].(string)
	c("PUT", path+"/parts/1", text)
	version := c("POST", path+"/complete", map[string]any{"parts": []any{map[string]any{"partNumber": 1, "sha256": sha}}})
	materialPath := p + "/materials/" + version["materialId"].(string)
	versionPath := p + "/material-versions/" + version["id"].(string)
	c("GET", materialPath+"/versions", nil)
	c("GET", versionPath, nil)
	for _, url := range []string{materialPath + "/versions/" + version["id"].(string) + "/content", versionPath + "/content"} {
		result := c("GET", url, nil)
		if result["content"] != string(text) {
			s.t.Fatal("download bytes changed")
		}
	}
	c("POST", materialPath+"/versions/"+version["id"].(string)+"/preview-session", map[string]any{})
	c("GET", versionPath+"/usages", nil)
	c("GET", versionPath+"/preview", nil)
	c("POST", versionPath+"/preview", map[string]any{})
	store, err := objectstore.New(context.Background(), objectstore.Options{Endpoint: endpoint, Region: "us-east-1", AccessKeyID: "contract", SecretAccessKey: "contract", Bucket: "contract", PathStyle: true})
	if err != nil {
		s.t.Fatal(err)
	}
	service := material.NewService(pool, store, material.Limits{}, nil)
	var previewJob job.Job
	err = pool.QueryRow(context.Background(), `UPDATE background_jobs SET state='running',fencing_token=fencing_token+1 WHERE id=(SELECT job_id FROM material_previews WHERE version_id=$1) RETURNING id,payload,fencing_token`, version["id"]).Scan(&previewJob.ID, &previewJob.Payload, &previewJob.FencingToken)
	if err != nil {
		s.t.Fatal(err)
	}
	if err = service.PreviewHandler().Execute(context.Background(), previewJob); err != nil {
		s.t.Fatal(err)
	}
	c("GET", versionPath+"/preview/content", nil)
	// Exercise the declared UUID sourceVersion contract, keeping old bytes fixed.
	next := c("POST", p+"/uploads", map[string]any{"name": "contract.txt", "kind": "file", "mime": "text/plain", "size": len(text), "sha256": sha})
	c("PUT", p+"/uploads/"+next["id"].(string)+"/parts/1", text)
	c("POST", p+"/uploads/"+next["id"].(string)+"/complete", map[string]any{"parts": []any{map[string]any{"partNumber": 1, "sha256": sha}}, "sourceVersion": version["id"]})
	unused := c("POST", p+"/uploads", map[string]any{"name": "unused.txt", "kind": "file", "mime": "text/plain", "size": len(text), "sha256": sha})
	c("DELETE", p+"/uploads/"+unused["id"].(string), nil)
	current := c("GET", materialPath+"/versions", nil)["items"].([]any)[0].(map[string]any)
	c("DELETE", materialPath, map[string]any{"expectedCurrentVersionId": current["id"]})
}

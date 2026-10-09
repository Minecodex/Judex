// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"

	"github.com/kakj-go/Judex/pkg/client"
)

// UploadFile streams a local file into an upload session + parts and
// completes the immutable version (07 §4). 8MiB parts.
const partSize = 8 << 20

func UploadFile(ctx context.Context, c *client.Client, projectID, path string) (map[string]any, error) {
	return uploadFile(ctx, c, projectID, path, "file", "", "")
}

func uploadFile(ctx context.Context, c *client.Client, projectID, path, kind, entrypoint, name string, purposes ...string) (map[string]any, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Whole-file digest first (single pass).
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var session struct {
		ID        string `json:"id"`
		PartCount int    `json:"partCount"`
		PartSize  int    `json:"partSize"`
	}
	if name == "" {
		name = info.Name()
	}
	body := map[string]any{
		"name":   name,
		"size":   info.Size(),
		"sha256": checksum,
		"mime":   uploadMime(name),
		"kind":   kind,
	}
	if len(purposes) > 0 && purposes[0] != "" {
		body["purpose"] = purposes[0]
	}
	if kind == "html_bundle" {
		body["entrypoint"] = entrypoint
	}
	receipt := ""
	if c.OutboxDirectory != "" {
		key := sha256.Sum256([]byte(projectID + "\n" + name + "\n" + checksum + "\n" + kind + "\n" + entrypoint))
		directory := filepath.Join(c.OutboxDirectory, "uploads")
		if err = os.MkdirAll(directory, 0700); err != nil {
			return nil, err
		}
		receipt = filepath.Join(directory, hex.EncodeToString(key[:])+".json")
		if raw, err := os.ReadFile(receipt); err == nil {
			if err = json.Unmarshal(raw, &session); err != nil {
				return nil, err
			}
		}
	}
	cursor := ""
	for session.ID == "" && kind == "file" {
		var page struct {
			Items []struct {
				ID, Name, Kind, Checksum string
				ExpectedSize             int64 `json:"expectedSize"`
				PartSize                 int   `json:"partSize"`
			} `json:"items"`
			NextCursor string `json:"nextCursor"`
		}
		if err = c.Do(ctx, "GET", "/projects/"+projectID+"/uploads?limit=100&cursor="+url.QueryEscape(cursor), nil, &page, ""); err != nil {
			return nil, err
		}
		for _, candidate := range page.Items {
			if candidate.Name == name && candidate.Kind == kind && candidate.Checksum == checksum && candidate.ExpectedSize == info.Size() {
				session.ID = candidate.ID
				session.PartSize = candidate.PartSize
				break
			}
		}
		if session.ID != "" || page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if session.ID == "" {
		if err := c.Do(ctx, "POST", "/projects/"+projectID+"/uploads", body, &session, requestKey()); err != nil {
			return nil, err
		}
	}

	if session.PartSize < 1 {
		session.PartSize = partSize
	}
	if receipt != "" {
		raw, _ := json.Marshal(session)
		if err = os.WriteFile(receipt, raw, 0600); err != nil {
			return nil, err
		}
	}
	completePath := fmt.Sprintf("/projects/%s/uploads/%s/complete", projectID, session.ID)
	var already map[string]any
	if err = c.Do(ctx, "POST", completePath, map[string]any{}, &already, requestKey()); err == nil {
		if already["sha256"] != checksum {
			return nil, fmt.Errorf("saved upload has a different checksum")
		}
		if receipt != "" {
			_ = os.Remove(receipt)
		}
		return already, nil
	} else {
		var cliError *client.CLIError
		if !errors.As(err, &cliError) || cliError.Status != 412 {
			return nil, err
		}
	}
	buf := make([]byte, session.PartSize)
	for part := 1; ; part++ {
		n, readErr := io.ReadFull(file, buf)
		if n == 0 {
			break
		}
		chunk := buf[:n]
		partSum := sha256.Sum256(chunk)
		if err := c.UploadPart(ctx, projectID, session.ID, part, chunk, hex.EncodeToString(partSum[:])); err != nil {
			return nil, fmt.Errorf("分片 %d 上传失败: %w", part, err)
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	var version map[string]any
	if err := c.Do(ctx, "POST", fmt.Sprintf("/projects/%s/uploads/%s/complete", projectID, session.ID),
		map[string]any{}, &version, client.NewKey()); err != nil {
		return nil, err
	}
	if receipt != "" {
		_ = os.Remove(receipt)
	}
	return version, nil
}

func UploadFileWithPurpose(ctx context.Context, c *client.Client, project, path, purpose string) (map[string]any, error) {
	return uploadFile(ctx, c, project, path, "file", "", "", purpose)
}
func uploadMime(name string) string {
	value := mime.TypeByExtension(filepath.Ext(name))
	if value == "" {
		return "application/octet-stream"
	}
	return value
}

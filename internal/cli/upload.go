// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/kakj-go/Judex/pkg/client"
)

// UploadFile streams a local file into an upload session + parts and
// completes the immutable version (07 §4). 8MiB parts.
const partSize = 8 << 20

func UploadFile(ctx context.Context, c *client.Client, projectID, path string) (map[string]any, error) {
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
	}
	body := map[string]any{
		"name":   info.Name(),
		"size":   info.Size(),
		"sha256": checksum,
		"mime":   "application/octet-stream",
		"kind":   "file",
	}
	if err := c.Do(ctx, "POST", "/projects/"+projectID+"/uploads", body, &session, ""); err != nil {
		return nil, err
	}

	buf := make([]byte, partSize)
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
	return version, nil
}

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/pkg/client"
)

// A composite command persists its intent before uploading anything. File
// receipts and the final wire body live until the final message is confirmed.
type sendOperation struct {
	Key       string          `json:"key"`
	Intent    string          `json:"intent"`
	Body      json.RawMessage `json:"body,omitempty"`
	Confirmed bool            `json:"confirmed"`
	Path      string          `json:"path"`
	CreatedAt time.Time       `json:"createdAt"`
}

func writeReceipt(name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(name), ".receipt-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), name)
	}
	return err
}

func sendWithFiles(ctx context.Context, c *client.Client, group, path string, body map[string]any, files []string, explicitKey string) (any, error) {
	if c.OutboxDirectory == "" {
		return nil, fmt.Errorf("authenticate before sending files so recovery can be saved")
	}
	descriptors := make([]map[string]string, 0, len(files))
	for _, name := range files {
		absolute, err := filepath.Abs(name)
		if err != nil {
			return nil, err
		}
		file, err := os.Open(absolute)
		if err != nil {
			return nil, err
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 100<<20 {
			file.Close()
			return nil, fmt.Errorf("regular file up to 100 MiB required: %s", name)
		}
		hash := sha256.New()
		_, err = io.Copy(hash, file)
		file.Close()
		if err != nil {
			return nil, err
		}
		descriptors = append(descriptors, map[string]string{"path": absolute, "sha256": hex.EncodeToString(hash.Sum(nil))})
	}
	raw, err := json.Marshal([]any{c.Server, group, path, body, descriptors})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	intent := hex.EncodeToString(digest[:])
	fingerprint := intent
	if explicitKey != "" {
		id, err := uuid.Parse(explicitKey)
		if err != nil {
			return nil, fmt.Errorf("--request-id must be a UUID: %w", err)
		}
		fingerprint = id.String()
	}
	directory := filepath.Join(c.OutboxDirectory, "operations-v2")
	if err = os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(directory, fingerprint+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	unlock, err := tryCredentialLock(lock)
	if err != nil {
		return nil, fmt.Errorf("this operation is already running; retry after it finishes")
	}
	defer unlock()
	directory = filepath.Join(directory, fingerprint)
	receipt := filepath.Join(directory, "operation.json")
	op := sendOperation{Key: explicitKey, Intent: intent, Path: path, CreatedAt: time.Now().UTC()}
	if raw, err = os.ReadFile(receipt); err == nil {
		if err = json.Unmarshal(raw, &op); err != nil {
			return nil, err
		}
		if op.Intent != intent {
			return nil, fmt.Errorf("--request-id already belongs to different content; use a new request ID for a new intent")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	} else {
		if op.Key == "" {
			op.Key = uuid.NewString()
		}
		if err = writeReceipt(receipt, op); err != nil {
			return nil, err
		}
	}
	seed, err := uuid.Parse(op.Key)
	if err != nil {
		return nil, err
	}
	// This journal owns all stage keys; the generic body-based outbox must not
	// coalesce a different operation with identical file metadata.
	stageClient := client.New(c.Server)
	stageClient.HTTP, stageClient.TokenSource, stageClient.Token = c.HTTP, c.TokenSource, c.Token
	if len(op.Body) == 0 {
		versions := []string{}
		for i, descriptor := range descriptors {
			key := uuid.NewSHA1(seed, []byte(fmt.Sprintf("file:%d", i))).String()
			result, err := uploadFile(ctx, stageClient, group, descriptor["path"], "", 0, false, uploadRecovery{
				key: key, receipt: filepath.Join(directory, fmt.Sprintf("file-%d.json", i)), retain: true, checksum: descriptor["sha256"],
			})
			if err != nil {
				return nil, err
			}
			versions = append(versions, result.ID)
		}
		body["fileVersionIds"] = versions
		op.Body, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
		if err = writeReceipt(receipt, op); err != nil {
			return nil, err
		}
	}
	var out any
	// Always ask the server, even for a locally confirmed explicit key. Current
	// grant, membership and effect permissions are checked again on replay.
	if err = stageClient.Do(ctx, "POST", path, op.Body, &out, op.Key); err != nil {
		return nil, err
	}
	if explicitKey == "" {
		err = os.RemoveAll(directory)
	} else {
		op.Confirmed = true
		err = writeReceipt(receipt, op)
	}
	return out, err
}

func pendingOperations(c *client.Client) ([]map[string]any, error) {
	out := []map[string]any{}
	if c.OutboxDirectory == "" {
		return out, nil
	}
	directory := filepath.Join(c.OutboxDirectory, "operations-v2")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(directory, entry.Name(), "operation.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var op sendOperation
		if err = json.Unmarshal(raw, &op); err != nil {
			return nil, err
		}
		if !op.Confirmed {
			out = append(out, map[string]any{"id": entry.Name(), "kind": "file-message", "method": "POST", "path": op.Path, "requestId": op.Key, "createdAt": op.CreatedAt, "resume": "repeat the original command"})
		}
	}
	return out, nil
}

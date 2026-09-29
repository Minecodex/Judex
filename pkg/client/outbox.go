package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type pendingCommand struct {
	Method    string          `json:"method"`
	Path      string          `json:"path"`
	Server    string          `json:"server"`
	CreatedAt time.Time       `json:"createdAt"`
	Key       string          `json:"key"`
	Body      json.RawMessage `json:"body"`
}

// prepareCommand persists a retry identity before a write can reach the
// server. Network loss retains the exact wire body and key across processes.
func (c *Client) prepareCommand(method, path string, raw []byte, key string) (pendingCommand, string, error) {
	command := pendingCommand{Key: key, Body: raw, Method: method, Path: path, Server: c.Server, CreatedAt: time.Now().UTC()}
	fingerprint := sha256.Sum256(append([]byte(c.Server+"\n"+method+"\n"+path+"\n"), raw...))
	filename := ""
	if c.OutboxDirectory != "" {
		if err := os.MkdirAll(c.OutboxDirectory, 0700); err != nil {
			return command, "", err
		}
		filename = filepath.Join(c.OutboxDirectory, hex.EncodeToString(fingerprint[:])+".json")
		if data, err := os.ReadFile(filename); err == nil {
			if err = json.Unmarshal(data, &command); err != nil {
				return command, "", fmt.Errorf("invalid pending command: %w", err)
			}
			return command, filename, nil
		} else if !os.IsNotExist(err) {
			return command, "", err
		}
	}
	if method == "POST" && strings.HasSuffix(path, "/submissions") {
		var body map[string]any
		if json.Unmarshal(raw, &body) == nil && body["clientSubmissionId"] == nil {
			body["clientSubmissionId"] = key
			command.Body, _ = json.Marshal(body)
		}
	}
	if filename == "" {
		return command, "", nil
	}
	data, err := json.Marshal(command)
	if err != nil {
		return command, "", err
	}
	file, err := os.CreateTemp(c.OutboxDirectory, ".command-*")
	if err != nil {
		return command, "", err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return command, "", err
	}
	// Publish a complete file without replacing another process's retry key.
	err = os.Link(temporary, filename)
	if os.IsExist(err) {
		data, err = os.ReadFile(filename)
		if err != nil {
			return command, "", err
		}
		err = json.Unmarshal(data, &command)
	}

	return command, filename, err
}
func retireCommand(filename string) {
	if filename != "" {
		_ = os.Remove(filename)
	}
}

func (c *Client) PendingCommands() ([]map[string]any, error) {
	out := []map[string]any{}
	if c.OutboxDirectory == "" {
		return out, nil
	}
	files, err := os.ReadDir(c.OutboxDirectory)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(c.OutboxDirectory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	for _, file := range files {
		if file.IsDir() || file.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(file.Name(), ".json")
		decoded, err := hex.DecodeString(id)
		if err != nil || len(decoded) != 32 {
			continue
		}
		raw, err := root.ReadFile(file.Name())
		if err != nil {
			return nil, err
		}
		var command pendingCommand
		if err = json.Unmarshal(raw, &command); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "method": command.Method, "path": command.Path, "requestId": command.Key, "createdAt": command.CreatedAt})
	}
	return out, nil
}
func (c *Client) RetryPending(ctx context.Context, id string) (any, error) {
	decoded, err := hex.DecodeString(id)
	if err != nil || len(decoded) != 32 || c.OutboxDirectory == "" {
		return nil, fmt.Errorf("invalid pending command id")
	}
	root, err := os.OpenRoot(c.OutboxDirectory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	name := id + ".json"
	raw, err := root.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var command pendingCommand
	if err = json.Unmarshal(raw, &command); err != nil {
		return nil, err
	}
	if command.Server != c.Server || !strings.HasPrefix(command.Path, "/") || strings.HasPrefix(command.Path, "/auth/") {
		return nil, fmt.Errorf("pending command lacks matching server/path metadata; retry the original command")
	}
	if _, err = uuid.Parse(command.Key); err != nil {
		return nil, err
	}
	switch command.Method {
	case "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, fmt.Errorf("pending method not supported")
	}
	var result any
	var body any = command.Body
	if len(command.Body) == 0 || bytes.Equal(command.Body, []byte("null")) {
		body = nil
	}
	err = c.Do(ctx, command.Method, command.Path, body, &result, command.Key)
	if err == nil {
		_ = root.Remove(name)
	}
	return result, err
}

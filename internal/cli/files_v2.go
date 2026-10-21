package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/pkg/client"
	"github.com/spf13/cobra"
)

type UploadedFile struct {
	ID, FileID string
	Revision   int
	Version    int64
}

func UploadFile(ctx context.Context, c *client.Client, group, path, update string, expected int64, announce ...bool) (UploadedFile, error) {
	return uploadFile(ctx, c, group, path, update, expected, len(announce) == 0 || announce[0], uploadRecovery{})
}

type uploadRecovery struct {
	key, receipt, checksum string
	retain                 bool
}

func uploadFile(ctx context.Context, c *client.Client, group, path, update string, expected int64, publish bool, recovery uploadRecovery) (UploadedFile, error) {
	result := UploadedFile{}
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() > 100<<20 {
		return result, fmt.Errorf("regular file up to 100 MiB required")
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return result, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if recovery.checksum != "" && recovery.checksum != checksum {
		return result, fmt.Errorf("file changed during the operation; retry with stable file contents")
	}
	if _, err = file.Seek(0, 0); err != nil {
		return result, err
	}
	body := map[string]any{"name": info.Name(), "mime": mime.TypeByExtension(filepath.Ext(path)), "size": info.Size(), "sha256": checksum}
	if update != "" {
		body["fileId"] = update
		body["expectedVersion"] = expected
	}
	var session struct {
		ID                  string `json:"id"`
		PartSize, PartCount int64
		NextPart            int64
		Result              *UploadedFile
	}
	receipt := recovery.receipt
	key := func(stage string) string {
		if recovery.key == "" {
			return requestKey()
		}
		return uuid.NewSHA1(uuid.MustParse(recovery.key), []byte(stage)).String()
	}
	if c.OutboxDirectory != "" {
		key := sha256.Sum256([]byte(group + "\n" + path + "\n" + checksum + "\n" + update + fmt.Sprint(expected)))
		directory := filepath.Join(c.OutboxDirectory, "uploads-v2")
		if err = os.MkdirAll(directory, 0700); err != nil {
			return result, err
		}
		if receipt == "" {
			receipt = filepath.Join(directory, hex.EncodeToString(key[:])+".json")
		}
	}
	if receipt != "" {
		if raw, e := os.ReadFile(receipt); e == nil {
			if err = json.Unmarshal(raw, &session); err != nil {
				return result, err
			}
		} else if !os.IsNotExist(e) {
			return result, e
		}
	}
	if session.Result != nil {
		return *session.Result, nil
	}
	if session.ID == "" {
		if err = c.Do(ctx, "POST", "/conversations/"+group+"/uploads", body, &session, key("create")); err != nil {
			return result, err
		}
		if receipt != "" {
			if err = writeReceipt(receipt, session); err != nil {
				return result, err
			}
		}
	}
	if session.ID == "" || session.PartSize <= 0 || session.PartCount < 0 {
		return result, fmt.Errorf("invalid upload session; retry to confirm the original request")
	}
	session.NextPart = max(1, session.NextPart)
	if _, err = file.Seek((session.NextPart-1)*session.PartSize, 0); err != nil {
		return result, err
	}
	for part := session.NextPart; part <= session.PartCount; part++ {
		data, err := io.ReadAll(io.LimitReader(file, session.PartSize))
		if err != nil {
			return result, err
		}
		uploadID, err := uuid.Parse(session.ID)
		if err != nil {
			return result, err
		}
		key := uuid.NewSHA1(uploadID, []byte(fmt.Sprint(part)+checksum)).String()
		if err = c.PutBytes(ctx, fmt.Sprintf("/conversations/%s/uploads/%s/parts/%d", group, session.ID, part), bytes.NewReader(data), int64(len(data)), key); err != nil {
			return result, err
		}
		session.NextPart = part + 1
		if receipt != "" {
			if err = writeReceipt(receipt, session); err != nil {
				return result, err
			}
		}
	}
	err = c.Do(ctx, "POST", "/conversations/"+group+"/uploads/"+session.ID+"/complete", map[string]any{"publishMessage": publish}, &result, key("complete"))
	if err == nil && result.ID == "" {
		return result, fmt.Errorf("file version could not be confirmed; retry the same operation")
	}
	if err == nil && receipt != "" {
		if recovery.retain {
			session.Result = &result
			err = writeReceipt(receipt, session)
		} else {
			err = os.Remove(receipt)
		}
	}
	return result, err
}
func uploadCommand() *cobra.Command {
	var update string
	var expected int64
	cmd := &cobra.Command{Use: "upload PATH", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, e := currentGroup()
		if e != nil {
			return emit(nil, e)
		}
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		out, e := UploadFile(cmd.Context(), c, id, args[0], update, expected)
		return emit(out, e)
	}}
	cmd.Flags().StringVar(&update, "update", "", "Owned file ID to explicitly update")
	cmd.Flags().Int64Var(&expected, "expected-version", 0, "Expected file configuration version")
	return cmd
}
func downloadCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{Use: "download VERSION_ID OUTPUT", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		group, e := currentGroup()
		if e != nil {
			return emit(nil, e)
		}
		if _, e = os.Stat(args[1]); e == nil && !force {
			return emit(nil, fmt.Errorf("output exists; choose another name or --force"))
		}
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		file, e := os.CreateTemp(filepath.Dir(args[1]), ".judex-download-*")
		if e != nil {
			return emit(nil, e)
		}
		name := file.Name()
		defer os.Remove(name)
		defer file.Close()
		e = c.Download(ctxOrBackground(cmd), "/conversations/"+group+"/file-versions/"+args[0]+"/content", file)
		if e != nil {
			return emit(nil, e)
		}
		if e = file.Close(); e == nil {
			e = os.Rename(name, args[1])
		}
		return emit(map[string]any{"path": args[1]}, e)
	}}
	cmd.Flags().BoolVar(&force, "force", false, "Replace an existing destination")
	return cmd
}
func ctxOrBackground(cmd *cobra.Command) context.Context { return cmd.Context() }

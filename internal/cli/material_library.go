package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kakj-go/Judex/pkg/client"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
)

type materialContextReceipt struct {
	Version map[string]any `json:"version"`
	Body    map[string]any `json:"body"`
	Result  map[string]any `json:"result,omitempty"`
}

func materialResult(v map[string]any, status string) map[string]any {
	out := map[string]any{}
	for key, value := range v {
		out[key] = value
	}
	out["versionId"] = v["id"]
	out["uploadStatus"] = "ready"
	out["registrationStatus"] = status
	return out
}

func saveMaterialReceipt(path string, r materialContextReceipt) error {
	if path == "" {
		return nil
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return e
	}
	temp, e := os.CreateTemp(filepath.Dir(path), ".material-context-*")
	if e != nil {
		return e
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, e = temp.Write(raw); e != nil {
		temp.Close()
		return e
	}
	if e = temp.Close(); e != nil {
		return e
	}
	return os.Rename(name, path)
}
func contextReceiptPath(c *client.Client, project, path, purpose, task, plan string) (string, error) {
	if c.OutboxDirectory == "" {
		return "", nil
	}
	file, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer file.Close()
	hash := sha256.New()
	if _, e = io.Copy(hash, file); e != nil {
		return "", e
	}
	name, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	key := sha256.Sum256([]byte(project + "\n" + name + "\n" + hex.EncodeToString(hash.Sum(nil)) + "\n" + purpose + "\n" + task + "\n" + plan))
	directory := filepath.Join(c.OutboxDirectory, "material-context")
	if e = os.MkdirAll(directory, 0700); e != nil {
		return "", e
	}
	return filepath.Join(directory, hex.EncodeToString(key[:])+".json"), nil
}
func materialUploadWithContext() *cobra.Command {
	var purpose, task, plan string
	cmd := &cobra.Command{Use: "upload PATH", Short: "上传固定版本，可登记用途和计划/任务关联", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if task != "" && plan != "" {
			return emit(nil, fmt.Errorf("--task and --plan are mutually exclusive"))
		}
		project, e := currentProject()
		if e != nil {
			return emit(nil, e)
		}
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		if purpose == "" && task == "" && plan == "" {
			v, e := UploadFile(cmd.Context(), c, project, args[0])
			if e != nil {
				return emit(nil, e)
			}
			return emit(materialResult(v, "not_requested"), nil)
		}
		path, e := contextReceiptPath(c, project, args[0], purpose, task, plan)
		if e != nil {
			return emit(nil, e)
		}
		var receipt materialContextReceipt
		if path != "" {
			raw, err := os.ReadFile(path)
			if err == nil {
				if e = json.Unmarshal(raw, &receipt); e != nil {
					return emit(nil, e)
				}
			} else if !os.IsNotExist(err) {
				return emit(nil, err)
			}
		}
		if receipt.Result != nil {
			return emit(receipt.Result, nil)
		}
		if receipt.Version == nil {
			receipt.Version, e = UploadFileWithPurpose(cmd.Context(), c, project, args[0], purpose)
			if e != nil {
				return emit(nil, e)
			}
		}
		version := receipt.Version
		result := materialResult(version, "pending")
		if receipt.Body == nil {
			description := purpose
			if description == "" {
				description = filepath.Base(args[0])
			}
			receipt.Body = map[string]any{"clientSubmissionId": client.NewKey(), "purpose": "material", "text": description, "materialVersionIds": []any{version["id"]}}
			if task != "" {
				receipt.Body["taskId"] = task
			}
			if plan != "" {
				receipt.Body["planId"] = plan
			}
		}
		if e = saveMaterialReceipt(path, receipt); e != nil {
			return emit(result, e)
		}
		var registration map[string]any
		if e = c.Do(cmd.Context(), "POST", "/projects/"+project+"/submissions", receipt.Body, &registration, fmt.Sprint(receipt.Body["clientSubmissionId"])); e != nil {
			result["registrationStatus"] = "failed"
			return emit(result, fmt.Errorf("upload succeeded (%v); context registration failed, retry the same command: %w", version["id"], e))
		}
		result["registrationStatus"] = "committed"
		result["submissionId"] = registration["id"]
		receipt.Result = result
		if e = saveMaterialReceipt(path, receipt); e != nil {
			return emit(result, e)
		}
		return emit(result, nil)
	}}
	cmd.Flags().StringVar(&purpose, "purpose", "", "资料用途说明")
	cmd.Flags().StringVar(&task, "task", "", "关联任务，所属计划由服务端确定")
	cmd.Flags().StringVar(&plan, "plan", "", "关联计划（与 --task 互斥）")
	return cmd
}

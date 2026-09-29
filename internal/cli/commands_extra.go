package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/pkg/client"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"time"
)

func requestKey() string {
	if requestIDFlag != "" {
		return requestIDFlag
	}
	return client.NewKey()
}
func emitIntent(cmd *cobra.Command, c *client.Client, prefix string, intent map[string]any) error {
	if link, ok := intent["confirmUrl"].(string); ok && strings.HasPrefix(link, "/") {
		intent["confirmUrl"] = c.Server + link
	}
	if noWaitFlag {
		return emit(intent, nil)
	}
	fmt.Fprintln(os.Stderr, "请本人在浏览器核对并确认：", intent["confirmUrl"])
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()
	id, _ := intent["id"].(string)
	if id == "" {
		return emit(nil, fmt.Errorf("missing intent id"))
	}
	for {
		var result map[string]any
		if err := c.Do(ctx, "GET", prefix+"/"+id, nil, &result, ""); err != nil {
			if ctx.Err() != nil {
				break
			}
			return emit(intent, err)
		}
		switch result["state"] {
		case "committed", "rejected":
			return emit(result, nil)
		case "stale", "expired":
			return emit(result, &client.CLIError{Status: 409, Code: "REVIEW_STALE", Message: "intent no longer current; request a new human review"})
		}
		select {
		case <-ctx.Done():
			return emit(intent, &client.CLIError{Status: 202, Code: "PENDING_CONFIRMATION", Message: "browser confirmation still pending"})
		case <-time.After(time.Second):
		}
	}
	return emit(intent, &client.CLIError{Status: 202, Code: "PENDING_CONFIRMATION", Message: "browser confirmation still pending"})
}
func projectShowCommand() *cobra.Command {
	return &cobra.Command{Use: "show ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var result map[string]any
		err = c.Do(cmd.Context(), "GET", "/projects/"+args[0], nil, &result, "")
		return emit(result, err)
	}}
}
func readObject(file string) (map[string]any, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var body map[string]any
	err = json.Unmarshal(raw, &body)
	if body == nil && err == nil {
		err = fmt.Errorf("JSON object required")
	}
	return body, err
}
func projectCreateCommand() *cobra.Command {
	var file string
	cmd := &cobra.Command{Use: "create", Short: "准备建项目，由浏览器本人确认", RunE: func(cmd *cobra.Command, args []string) error {
		body, err := readObject(file)
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var intent map[string]any
		err = c.Do(cmd.Context(), "POST", "/confirmation-intents", map[string]any{"operation": "project.create", "objectId": uuid.NewSHA1(uuid.NameSpaceOID, []byte(c.Server+payloadHash(body))).String(), "reviewHash": payloadHash(body), "payload": body}, &intent, requestKey())
		if err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, c, "/confirmation-intents", intent)
	}}
	cmd.Flags().StringVar(&file, "file", "", "项目 JSON")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}
func ownIdentities(cmd *cobra.Command) ([]map[string]any, error) {
	project, err := currentProject()
	if err != nil {
		return nil, err
	}
	c, err := NewClient()
	if err != nil {
		return nil, err
	}
	var session struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err = c.Do(cmd.Context(), "GET", "/auth/session", nil, &session, ""); err != nil {
		return nil, err
	}
	out := []map[string]any{}
	cursor := ""
	for {
		var page struct {
			Items      []map[string]any `json:"items"`
			NextCursor string           `json:"nextCursor"`
		}
		if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/identities?limit=100&cursor="+cursor, nil, &page, ""); err != nil {
			return nil, err
		}
		for _, identity := range page.Items {
			binding, _ := identity["currentBinding"].(map[string]any)
			if binding["userId"] == session.User.ID {
				out = append(out, identity)
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return out, nil
}
func identityListCommand() *cobra.Command {
	return &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		items, err := ownIdentities(cmd)
		return emit(items, err)
	}}
}
func identityUseCommand() *cobra.Command {
	return &cobra.Command{Use: "use ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		items, err := ownIdentities(cmd)
		if err != nil {
			return emit(nil, err)
		}
		for _, identity := range items {
			if identity["id"] == args[0] {
				profile := LoadProfile()
				profile.IdentityID = args[0]
				err = SaveProfile(profile)
				return emit(profile, err)
			}
		}
		return emit(nil, fmt.Errorf("identity is not currently held by this account"))
	}}
}
func fileCommand(name, description, suffix string, prepare func(map[string]any)) *cobra.Command {
	var file string
	cmd := &cobra.Command{Use: name, Short: description, RunE: func(cmd *cobra.Command, args []string) error {
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		body, err := readObject(file)
		if err != nil {
			return emit(nil, err)
		}
		if prepare != nil {
			prepare(body)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var result map[string]any
		err = c.Do(cmd.Context(), "POST", "/projects/"+project+suffix, body, &result, requestKey())
		return emit(result, err)
	}}
	cmd.Flags().StringVar(&file, "file", "", "明确选定的 JSON 文件")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}
func planAcceptCommand() *cobra.Command {
	return &cobra.Command{Use: "accept ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var review map[string]any
		if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/plans/"+args[0]+"/acceptance-review", nil, &review, ""); err != nil {
			return emit(nil, err)
		}
		var intent map[string]any
		prefix := "/projects/" + project + "/confirmation-intents"
		err = c.Do(cmd.Context(), "POST", prefix, map[string]any{"operation": "plan.acceptance", "objectId": args[0], "reviewHash": review["reviewHash"], "payload": map[string]any{"reviewId": review["reviewId"], "expectedVersion": review["targetVersion"], "decision": "accept"}}, &intent, requestKey())
		if err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, c, prefix, intent)
	}}
}
func taskReopenCommand() *cobra.Command {
	var reason string
	cmd := &cobra.Command{Use: "reopen ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var task map[string]any
		if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/tasks/"+args[0], nil, &task, ""); err != nil {
			return emit(nil, err)
		}
		var intent map[string]any
		prefix := "/projects/" + project + "/confirmation-intents"
		err = c.Do(cmd.Context(), "POST", prefix, map[string]any{"operation": "task.reopen", "objectId": args[0], "reviewHash": task["latestAcceptanceId"], "payload": map[string]any{"acceptanceId": task["latestAcceptanceId"], "expectedVersion": task["version"], "reason": reason}}, &intent, requestKey())
		if err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, c, prefix, intent)
	}}
	cmd.Flags().StringVar(&reason, "reason", "", "重开理由")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}
func runCommand(action string) *cobra.Command {
	var reason string
	cmd := &cobra.Command{Use: action + " ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		path := "/projects/" + project + "/runs/" + args[0]
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()
		for {
			var result map[string]any
			if err = c.Do(ctx, "GET", path, nil, &result, ""); err != nil {
				return emit(nil, err)
			}
			if action == "cancel" {
				var response any
				err = c.Do(ctx, "POST", path+"/cancel", map[string]any{"expectedVersion": result["version"], "reason": reason}, &response, requestKey())
				return emit(response, err)
			}
			state, _ := result["state"].(string)
			if action == "show" || state == "succeeded" || state == "failed" || state == "cancelled" || state == "waiting_human" {
				return emit(result, nil)
			}
			select {
			case <-ctx.Done():
				return emit(result, &client.CLIError{Status: 202, Code: "PENDING_CONFIRMATION", Message: "run still active"})
			case <-time.After(time.Second):
			}
		}
	}}
	if action == "cancel" {
		cmd.Flags().StringVar(&reason, "reason", "", "取消理由")
		_ = cmd.MarkFlagRequired("reason")
	}
	return cmd
}

func payloadHash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func pendingCommand() *cobra.Command {
	root := &cobra.Command{Use: "pending", Short: "当前凭据范围的中断请求"}
	root.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		items, err := c.PendingCommands()
		return emit(items, err)
	}})
	root.AddCommand(&cobra.Command{Use: "retry ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		result, err := c.RetryPending(cmd.Context(), args[0])
		return emit(result, err)
	}})
	return root
}

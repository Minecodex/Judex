// SPDX-License-Identifier: Apache-2.0

// Package cli hosts the judex command tree (docs/plans/v1/07 §2-§4).
// Commands are small and share pkg/client; secrets never enter logs.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/kakj-go/Judex/pkg/client"
)

// Output is the --json machine envelope (07 §6).
type Output struct {
	SchemaVersion string `json:"schemaVersion"`
	OK            bool   `json:"ok"`
	Data          any    `json:"data,omitempty"`
	Error         string `json:"error,omitempty"`
	RequestID     string `json:"requestId,omitempty"`
}

var (
	serverFlag  string
	profileFlag string
	jsonFlag    bool
)

// Profile is the non-secret context (.judex/project.json is safe to commit).
type Profile struct {
	Server     string `json:"server"`
	ProjectID  string `json:"projectId,omitempty"`
	IdentityID string `json:"identityId,omitempty"`
}

func credentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".judex-credentials.json"
	}
	return filepath.Join(home, ".judex", "credentials.json")
}

func profilePath() string {
	if dir, err := os.Getwd(); err == nil {
		candidate := filepath.Join(dir, ".judex", "project.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".judex", "project.json")
}

// LoadToken resolves the CLI secret: env override first, then the 0600
// credentials file (explicit fallback, warned once).
func LoadToken() (string, error) {
	if token := os.Getenv("JUDEX_TOKEN"); token != "" {
		return token, nil
	}
	path := credentialsPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("未登录：运行 judex auth login，或设置一次性 JUDEX_TOKEN")
	}
	var doc map[string]string
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", errors.New("凭据文件损坏：" + path)
	}
	return doc[profileKey()], nil
}

func SaveToken(server, token string) error {
	path := credentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(map[string]string{profileKey(): token, "server": server}, "", "  ")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		// Windows ACLs: keep the file user-scoped via attrib visibility only;
		// real ACL hardening documented in ops guide.
		_ = os.Chmod(path, 0o600)
	}
	fmt.Fprintln(os.Stderr, "提示：凭据保存在", path, "（0600）。建议使用环境变量 JUDEX_TOKEN 或系统凭据管理器。")
	return nil
}

func ClearToken() {
	_ = os.Remove(credentialsPath())
}

func profileKey() string {
	if profileFlag != "" {
		return profileFlag
	}
	return "default"
}

func LoadProfile() Profile {
	raw, err := os.ReadFile(profilePath())
	if err != nil {
		return Profile{}
	}
	var p Profile
	_ = json.Unmarshal(raw, &p)
	return p
}

func SaveProfile(p Profile) error {
	path := profilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(path, raw, 0o644)
}

// NewClient builds a client from flags/profile/credentials.
func NewClient() (*client.Client, error) {
	profile := LoadProfile()
	server := serverFlag
	if server == "" {
		server = profile.Server
	}
	if server == "" {
		server = "http://127.0.0.1:8080"
	}
	c := client.New(server)
	token, err := LoadToken()
	if err == nil {
		c.Token = token
		c.OnRotation = func(newToken string) { _ = SaveToken(server, newToken) }
	}
	return c, nil
}

func emit(data any, err error) error {
	if jsonFlag {
		out := Output{SchemaVersion: "1", OK: err == nil, Data: data}
		if err != nil {
			out.Error = err.Error()
		}
		raw, _ := json.Marshal(out)
		fmt.Println(string(raw))
	} else if err == nil {
		if data != nil {
			raw, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(raw))
		}
	} else {
		fmt.Fprintln(os.Stderr, "错误:", err)
	}
	if err != nil {
		var cliErr *client.CLIError
		if errors.As(err, &cliErr) {
			exitCode = cliErr.ExitCode()
		} else {
			exitCode = 1
		}
		return err
	}
	return nil
}

// exitCode is read by main to map errors to codes (07 §6).
var exitCode int

func TakeExitCode() int { return exitCode }

// Root builds the full command tree.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:          "judex",
		Short:        "Judex CLI：本地工作与平台协作的客户端",
		SilenceUsage: true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&serverFlag, "server", "", "服务器地址（覆盖 profile）")
	root.PersistentFlags().StringVar(&profileFlag, "profile", "default", "凭据 profile")
	root.PersistentFlags().BoolVar(&jsonFlag, "json", false, "机器可读输出")
	root.AddCommand(versionCommand(), statusCommand())

	authCmd := &cobra.Command{Use: "auth", Short: "登录与凭据"}
	authCmd.AddCommand(authLoginCommand(), authWhoamiCommand(), authLogoutCommand())
	root.AddCommand(authCmd)

	projectCmd := &cobra.Command{Use: "project", Short: "项目"}
	projectCmd.AddCommand(projectListCommand(), projectUseCommand())
	root.AddCommand(projectCmd)

	inboxCmd := &cobra.Command{Use: "inbox", Short: "待办"}
	inboxCmd.AddCommand(inboxCommand())
	root.AddCommand(inboxCmd)

	materialCmd := &cobra.Command{Use: "material", Short: "资料"}
	materialCmd.AddCommand(materialListCommand(), materialUploadCommand())
	root.AddCommand(materialCmd)

	root.AddCommand(submitCommand(), reportCommand())

	proposalCmd := &cobra.Command{Use: "proposal", Short: "提案"}
	proposalCmd.AddCommand(proposalDraftCommand(), proposalSubmitCommand())
	root.AddCommand(proposalCmd)

	decisionCmd := &cobra.Command{Use: "decision", Short: "审批"}
	decisionCmd.AddCommand(decisionReviewCommand())
	root.AddCommand(decisionCmd)

	taskCmd := &cobra.Command{Use: "task", Short: "任务"}
	taskCmd.AddCommand(taskAcceptCommand())
	root.AddCommand(taskCmd)

	handoffCmd := &cobra.Command{Use: "handoff", Short: "交接"}
	handoffCmd.AddCommand(handoffSendCommand(), handoffReceiveCommand())
	root.AddCommand(handoffCmd)

	eventsCmd := &cobra.Command{Use: "events", Short: "事件"}
	eventsCmd.AddCommand(eventsWatchCommand())
	root.AddCommand(eventsCmd)
	return root
}

func versionCommand() *cobra.Command {
	return &cobra.Command{Use: "version", Short: "协议与客户端版本", RunE: func(cmd *cobra.Command, args []string) error {
		return emit(map[string]string{"cli": "1", "protocol": "1"}, nil)
	}}
}

func statusCommand() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "服务可达性", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var system map[string]any
		if err := c.Do(cmd.Context(), "GET", "/system", nil, &system, ""); err != nil {
			return emit(nil, err)
		}
		return emit(system, nil)
	}}
}

func authLoginCommand() *cobra.Command {
	var scopes []string
	cmd := &cobra.Command{Use: "login", Short: "设备授权登录（浏览器确认一次）", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		if len(scopes) == 0 {
			scopes = []string{"projects:read", "context:read", "materials:read", "materials:write",
				"submissions:write", "reports:write", "proposals:draft", "events:read", "intents:create"}
		}
		token, err := c.DeviceLogin(cmd.Context(), "judex-cli@" + runtime.GOOS, scopes)
		if err != nil {
			return emit(nil, err)
		}
		if err := SaveToken(c.Server, token); err != nil {
			return emit(nil, err)
		}
		profile := LoadProfile()
		profile.Server = c.Server
		_ = SaveProfile(profile)
		return emit(map[string]string{"status": "logged_in", "server": c.Server}, nil)
	}}
	cmd.Flags().StringSliceVar(&scopes, "scopes", nil, "请求的 scopes")
	return cmd
}

func authWhoamiCommand() *cobra.Command {
	return &cobra.Command{Use: "whoami", Short: "当前账号与 scopes", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var session map[string]any
		if err := c.Do(cmd.Context(), "GET", "/auth/session", nil, &session, ""); err != nil {
			return emit(nil, err)
		}
		return emit(session, nil)
	}}
}

func authLogoutCommand() *cobra.Command {
	return &cobra.Command{Use: "logout", Short: "撤销本设备授权", RunE: func(cmd *cobra.Command, args []string) error {
		ClearToken()
		return emit(map[string]string{"status": "logged_out"}, nil)
	}}
}

func projectListCommand() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "我的项目", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var page struct {
			Items []map[string]any `json:"items"`
		}
		if err := c.Do(cmd.Context(), "GET", "/projects", nil, &page, ""); err != nil {
			return emit(nil, err)
		}
		return emit(page.Items, nil)
	}}
}

func projectUseCommand() *cobra.Command {
	return &cobra.Command{Use: "use ID", Short: "选择当前项目（非机密）", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		profile := LoadProfile()
		profile.ProjectID = args[0]
		if err := SaveProfile(profile); err != nil {
			return emit(nil, err)
		}
		return emit(profile, nil)
	}}
}

func currentProject() (string, error) {
	profile := LoadProfile()
	if profile.ProjectID == "" {
		return "", errors.New("未选择项目：运行 judex project use ID")
	}
	return profile.ProjectID, nil
}

func inboxCommand() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "我的待办", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var page struct {
			Items []map[string]any `json:"items"`
		}
		if err := c.Do(cmd.Context(), "GET", "/me/actions", nil, &page, ""); err != nil {
			return emit(nil, err)
		}
		return emit(page.Items, nil)
	}}
}

func materialListCommand() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "项目共享资料", RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var page struct {
			Items []map[string]any `json:"items"`
		}
		if err := c.Do(cmd.Context(), "GET", "/projects/"+projectID+"/materials", nil, &page, ""); err != nil {
			return emit(nil, err)
		}
		return emit(page.Items, nil)
	}}
}

func materialUploadCommand() *cobra.Command {
	return &cobra.Command{Use: "upload PATH", Short: "分片上传并登记版本", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		version, err := UploadFile(cmd.Context(), c, projectID, args[0])
		if err != nil {
			return emit(nil, err)
		}
		return emit(version, nil)
	}}
}

func submitCommand() *cobra.Command {
	var file string
	cmd := &cobra.Command{Use: "submit", Short: "自由文本+材料提交（--file submission.json）", RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return emit(nil, err)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			return emit(nil, err)
		}
		body["clientSubmissionId"] = client.NewKey()
		var result map[string]any
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/submissions", body, &result, client.NewKey()); err != nil {
			return emit(nil, err)
		}
		return emit(result, nil)
	}}
	cmd.Flags().StringVar(&file, "file", "", "submission JSON 文件")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func reportCommand() *cobra.Command {
	var file string
	var kind string
	var taskID string
	cmd := &cobra.Command{Use: "report", Short: "progress/delivery 上报", RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		raw, _ := os.ReadFile(file)
		var fields map[string]any
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &fields)
		}
		body := map[string]any{"kind": kind, "text": fmt.Sprintf("%v", fields["text"])}
		if v, ok := fields["expectedTaskVersion"].(float64); ok {
			body["expectedTaskVersion"] = int64(v)
		}
		var result map[string]any
		path := fmt.Sprintf("/projects/%s/tasks/%s/reports", projectID, taskID)
		if err := c.Do(cmd.Context(), "POST", path, body, &result, client.NewKey()); err != nil {
			return emit(nil, err)
		}
		return emit(result, nil)
	}}
	cmd.Flags().StringVar(&file, "file", "", "report JSON（text/expectedTaskVersion）")
	cmd.Flags().StringVar(&kind, "kind", "progress", "progress | delivery")
	cmd.Flags().StringVar(&taskID, "task", "", "任务 ID")
	_ = cmd.MarkFlagRequired("task")
	return cmd
}

func proposalDraftCommand() *cobra.Command {
	var file string
	cmd := &cobra.Command{Use: "draft", Short: "创建 typed 草稿", RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return emit(nil, err)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			return emit(nil, err)
		}
		var result map[string]any
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/proposals", body, &result, client.NewKey()); err != nil {
			return emit(nil, err)
		}
		return emit(result, nil)
	}}
	cmd.Flags().StringVar(&file, "file", "", "proposal JSON")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func proposalSubmitCommand() *cobra.Command {
	var id, review string
	cmd := &cobra.Command{Use: "submit ID --review HASH", Short: "提交审批（冻结审阅）", RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var result map[string]any
		path := fmt.Sprintf("/projects/%s/proposals/%s/submit", projectID, id)
		body := map[string]any{"expectedVersion": 1}
		_ = review
		if err := c.Do(cmd.Context(), "POST", path, body, &result, client.NewKey()); err != nil {
			return emit(nil, err)
		}
		return emit(result, nil)
	}}
	cmd.Flags().StringVar(&id, "id", "", "提案 ID")
	cmd.Flags().StringVar(&review, "review", "", "草稿哈希（可选）")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func decisionReviewCommand() *cobra.Command {
	return &cobra.Command{Use: "review ID", Short: "读取固定审阅", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var review map[string]any
		path := fmt.Sprintf("/projects/%s/proposals/%s/review", projectID, args[0])
		if err := c.Do(cmd.Context(), "GET", path, nil, &review, ""); err != nil {
			return emit(nil, err)
		}
		return emit(review, nil)
	}}
}

func taskAcceptCommand() *cobra.Command {
	return &cobra.Command{Use: "accept ID", Short: "取得快照并创建浏览器确认意图", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var review map[string]any
		path := fmt.Sprintf("/projects/%s/tasks/%s/acceptance-review", projectID, args[0])
		if err := c.Do(cmd.Context(), "GET", path, nil, &review, ""); err != nil {
			return emit(nil, err)
		}
		// Human confirmation intent: the browser confirms, CLI reads result.
		intent := map[string]any{
			"operation":  "task.acceptance",
			"objectId":   args[0],
			"reviewHash": review["reviewHash"],
			"payload": map[string]any{
				"decision": "accept", "expectedVersion": review["targetVersion"],
				"reviewId": review["reviewId"],
			},
		}
		var created map[string]any
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/confirmation-intents", intent, &created, client.NewKey()); err != nil {
			return emit(nil, err)
		}
		return emit(created, nil)
	}}
}

func handoffSendCommand() *cobra.Command {
	var version int
	cmd := &cobra.Command{Use: "send SOURCE --version N", Short: "发送来源（经确认意图）", RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		_ = version
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		intent := map[string]any{
			"operation": "handoff.send", "objectId": args[0],
			"reviewHash": fmt.Sprintf("v%d", version),
			"payload":    map[string]any{"summary": "由 CLI 提交"},
		}
		var created map[string]any
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/confirmation-intents", intent, &created, client.NewKey()); err != nil {
			return emit(nil, err)
		}
		return emit(created, nil)
	}}
	cmd.Flags().IntVar(&version, "version", 1, "source version")
	return cmd
}

func handoffReceiveCommand() *cobra.Command {
	var accept bool
	cmd := &cobra.Command{Use: "receive SOURCE", Short: "接收/拒收来源（经确认意图）", RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		decision := "reject"
		if accept {
			decision = "accept"
		}
		intent := map[string]any{
			"operation": "handoff.decision", "objectId": args[0], "reviewHash": "cli",
			"payload": map[string]any{"decision": decision},
		}
		var created map[string]any
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/confirmation-intents", intent, &created, client.NewKey()); err != nil {
			return emit(nil, err)
		}
		return emit(created, nil)
	}}
	cmd.Flags().BoolVar(&accept, "accept", false, "接收（默认拒收）")
	return cmd
}

func eventsWatchCommand() *cobra.Command {
	return &cobra.Command{Use: "watch", Short: "拉取项目事件（只读）", RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var page map[string]any
		if err := c.Do(cmd.Context(), "GET", "/projects/"+projectID+"/topics", nil, &page, ""); err != nil {
			return emit(nil, err)
		}
		return emit(page, nil)
	}}
}

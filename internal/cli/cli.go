// SPDX-License-Identifier: Apache-2.0

// Package cli hosts the judex command tree (docs/plans/v1/07 §2-§4).
// Commands are small and share pkg/client; secrets never enter logs.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kakj-go/Judex/internal/version"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	serverFlag    string
	projectFlag   string
	noWaitFlag    bool
	requestIDFlag string
	profileFlag   string
	jsonFlag      bool
)

// Profile is the non-secret context (.judex/project.json is safe to commit).
type Profile struct {
	UserID     string `json:"userId,omitempty"`
	Server     string `json:"server"`
	ProjectID  string `json:"projectId,omitempty"`
	IdentityID string `json:"identityId,omitempty"`
}

func credentialsPath() string {
	if dir := os.Getenv("JUDEX_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "credentials.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".judex-credentials.json"
	}
	return filepath.Join(home, ".judex", "credentials.json")
}

func profilePath() string {
	if dir := os.Getenv("JUDEX_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "project.json")
	}
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
	stored := map[string]string{}
	if existing, err := os.ReadFile(path); err == nil {
		if err = json.Unmarshal(existing, &stored); err != nil {
			return fmt.Errorf("invalid credentials file: %w", err)
		}
	}
	stored[profileKey()] = token
	stored["server:"+profileKey()] = server
	raw, _ := json.MarshalIndent(stored, "", "  ")
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

func ClearToken() error {
	path := credentialsPath()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var doc map[string]string
	if err = json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	delete(doc, profileKey())
	delete(doc, "server:"+profileKey())
	delete(doc, "access:"+profileKey())
	delete(doc, "accessExpiry:"+profileKey())
	raw, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
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
	if err == nil && os.Getenv("JUDEX_TOKEN") == "" {
		var stored map[string]string
		if raw, readErr := os.ReadFile(credentialsPath()); readErr == nil && json.Unmarshal(raw, &stored) == nil {
			saved := stored["server:"+profileKey()]
			if saved == "" {
				saved = stored["server"]
			}
			if saved != "" && strings.TrimSuffix(saved, "/") != strings.TrimSuffix(server, "/") {
				err = errors.New("stored credential belongs to another server")
			}
		}
	}
	if err == nil {
		if os.Getenv("JUDEX_TOKEN") != "" {
			c.Token = token
		} else {
			c.TokenSource = storedAccessSource(c, server)
		}
		subject := profile.UserID
		if subject == "" || os.Getenv("JUDEX_TOKEN") != "" {
			subject = token
		}
		scope := sha256.Sum256([]byte(server + "\n" + profileKey() + "\n" + subject))
		c.OutboxDirectory = filepath.Join(filepath.Dir(credentialsPath()), "outbox", hex.EncodeToString(scope[:]))
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
		Use:           "judex",
		Short:         "Judex CLI：本地工作与平台协作的客户端",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&noWaitFlag, "no-wait", false, "仅创建确认意图，不等待浏览器结果")
	root.PersistentFlags().StringVar(&requestIDFlag, "request-id", "", "固定业务请求幂等 UUID")
	root.PersistentFlags().StringVar(&projectFlag, "project", "", "项目 ID（不修改已保存选择）")
	root.PersistentFlags().StringVar(&serverFlag, "server", "", "服务器地址（覆盖 profile）")
	root.PersistentFlags().StringVar(&profileFlag, "profile", "default", "凭据 profile")
	root.PersistentFlags().BoolVar(&jsonFlag, "json", false, "机器可读输出")
	root.AddCommand(versionCommand(), statusCommand(), doctorCommand(), pendingCommand())

	authCmd := &cobra.Command{Use: "auth", Short: "登录与凭据"}
	authCmd.AddCommand(authLoginCommand(), authWhoamiCommand(), authLogoutCommand())
	root.AddCommand(authCmd)

	projectCmd := &cobra.Command{Use: "project", Short: "项目"}
	projectCmd.AddCommand(projectListCommand(), projectUseCommand(), projectShowCommand(), projectCreateCommand())
	root.AddCommand(projectCmd)
	identityCmd := &cobra.Command{Use: "identity", Short: "本人职责"}
	identityCmd.AddCommand(identityListCommand(), identityUseCommand())
	root.AddCommand(identityCmd)
	planCmd := &cobra.Command{Use: "plan", Short: "计划"}
	planCmd.AddCommand(planAcceptCommand())
	root.AddCommand(planCmd)
	bugCmd := &cobra.Command{Use: "bug", Short: "问题草稿"}
	bugCmd.AddCommand(fileCommand("create", "创建 Bug 草稿", "/tasks", func(body map[string]any) { body["kind"] = "bug" }))
	root.AddCommand(bugCmd)
	releaseCmd := &cobra.Command{Use: "release", Short: "部署事实"}
	releaseCmd.AddCommand(fileCommand("report", "记录本地部署结果", "/release-reports", nil))
	root.AddCommand(releaseCmd)
	runCmd := &cobra.Command{Use: "run", Short: "平台分析运行"}
	runCmd.AddCommand(runCommand("show"), runCommand("watch"), runCommand("cancel"))
	root.AddCommand(runCmd)

	inboxCmd := &cobra.Command{Use: "inbox", Short: "待办"}
	inboxCmd.AddCommand(inboxCommand())
	root.AddCommand(inboxCmd)

	materialCmd := &cobra.Command{Use: "material", Short: "资料"}
	materialCmd.AddCommand(materialListCommand(), materialUploadCommand(), materialDownloadCommand())
	root.AddCommand(materialCmd, bundleCommand())

	root.AddCommand(submitCommand(), reportCommand())

	proposalCmd := &cobra.Command{Use: "proposal", Short: "提案"}
	proposalCmd.AddCommand(proposalDraftCommand(), proposalSubmitCommand())
	root.AddCommand(proposalCmd)

	decisionCmd := &cobra.Command{Use: "decision", Short: "审批"}
	decisionCmd.AddCommand(decisionReviewCommand(), decisionCommand(true), decisionCommand(false), intentResultCommand())
	contextCmd := &cobra.Command{Use: "context", Short: "任务上下文"}
	contextCmd.AddCommand(contextCommand())
	root.AddCommand(contextCmd)
	root.AddCommand(decisionCmd)

	taskCmd := &cobra.Command{Use: "task", Short: "任务"}
	taskCmd.AddCommand(taskAcceptCommand(), taskReopenCommand())
	root.AddCommand(taskCmd)

	handoffCmd := &cobra.Command{Use: "handoff", Short: "交接"}
	handoffCmd.AddCommand(handoffSendCommand(), handoffReceiveCommand(), handoffRejectCommand())
	root.AddCommand(handoffCmd)

	eventsCmd := &cobra.Command{Use: "events", Short: "事件"}
	eventsCmd.AddCommand(eventsWatchCommand())
	root.AddCommand(eventsCmd)

	skillCmd := &cobra.Command{Use: "skill", Short: "发行技能"}
	skillCmd.AddCommand(skillInstallCommand(), skillUninstallCommand())
	root.AddCommand(skillCmd)
	return root
}

func versionCommand() *cobra.Command {
	return &cobra.Command{Use: "version", Short: "协议与客户端版本", RunE: func(cmd *cobra.Command, args []string) error {
		return emit(map[string]string{"cli": version.Version, "commit": version.Commit, "protocol": "1"}, nil)
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
		token, err := c.DeviceLogin(cmd.Context(), "judex-cli@"+runtime.GOOS, scopes)
		if err != nil {
			return emit(nil, err)
		}
		if err := SaveTokenPair(c.Server, client.TokenPair{AccessToken: c.Token, AccessExpiresAt: c.AccessExpiresAt, RefreshToken: token}); err != nil {
			return emit(nil, err)
		}
		profile := LoadProfile()
		profile.Server = c.Server
		var session struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		}
		if err = c.Do(cmd.Context(), "GET", "/auth/session", nil, &session, ""); err != nil {
			return emit(nil, err)
		}
		profile.UserID = session.User.ID
		if err = SaveProfile(profile); err != nil {
			return emit(nil, err)
		}
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
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var session struct {
			GrantID string `json:"grantId"`
		}
		err = c.Do(cmd.Context(), "GET", "/auth/session", nil, &session, "")
		if err == nil && session.GrantID != "" {
			err = c.Do(cmd.Context(), "DELETE", "/me/client-grants/"+session.GrantID, nil, nil, requestKey())
		}
		var ce *client.CLIError
		if err != nil && !(errors.As(err, &ce) && ce.Status == 401) {
			return emit(nil, err)
		}
		if err = ClearToken(); err != nil {
			return emit(nil, err)
		}
		return emit(map[string]string{"status": "logged_out"}, nil)
	}}
}

func projectListCommand() *cobra.Command {
	return listCommand("我的项目", func() (string, error) { return "/projects", nil })
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
	if projectFlag != "" {
		return projectFlag, nil
	}
	profile := LoadProfile()
	if profile.ProjectID == "" {
		return "", errors.New("未选择项目：运行 judex project use ID")
	}
	return profile.ProjectID, nil
}

func inboxCommand() *cobra.Command {
	return listCommand("我的待办", func() (string, error) {
		if projectFlag != "" {
			return "/me/actions?projectId=" + projectFlag, nil
		}
		return "/me/actions", nil
	})
}

func materialListCommand() *cobra.Command {
	return listCommand("资料", func() (string, error) {
		project, err := currentProject()
		return "/projects/" + project + "/materials", err
	})
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

		var result map[string]any
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/submissions", body, &result, requestKey()); err != nil {
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
	cmd := &cobra.Command{Use: "report [progress|delivery]", Args: cobra.MaximumNArgs(1), Short: "progress/delivery 上报", RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			kind = args[0]
		}
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
		var fields map[string]any
		if err = json.Unmarshal(raw, &fields); err != nil {
			return emit(nil, err)
		}
		fields["kind"] = kind
		if fields["expectedTaskVersion"] == nil {
			return emit(nil, fmt.Errorf("expectedTaskVersion required; read task context first"))
		}
		body := fields
		var result map[string]any
		path := fmt.Sprintf("/projects/%s/tasks/%s/reports", projectID, taskID)
		if err := c.Do(cmd.Context(), "POST", path, body, &result, requestKey()); err != nil {
			return emit(nil, err)
		}
		return emit(result, nil)
	}}
	cmd.Flags().StringVar(&file, "file", "", "report JSON（text/expectedTaskVersion）")
	cmd.Flags().StringVar(&kind, "kind", "progress", "progress | delivery")
	cmd.Flags().StringVar(&taskID, "task", "", "任务 ID")
	_ = cmd.MarkFlagRequired("task")
	_ = cmd.MarkFlagRequired("file")
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
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/proposals", body, &result, requestKey()); err != nil {
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
	cmd := &cobra.Command{Use: "submit ID", Args: cobra.MaximumNArgs(1), Short: "提交提案（浏览器确认）", RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			id = args[0]
		}
		if id == "" {
			return fmt.Errorf("proposal ID required")
		}
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var snapshot map[string]any
		if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/proposals/"+id+"/review", nil, &snapshot, ""); err != nil {
			return emit(nil, err)
		}
		if review != "" && review != snapshot["reviewHash"] {
			return emit(nil, fmt.Errorf("review hash changed"))
		}
		version, ok := snapshot["version"].(float64)
		if !ok || version < 1 {
			return emit(nil, fmt.Errorf("proposal review lacks a version"))
		}
		var result map[string]any
		err = c.Do(cmd.Context(), "POST", "/projects/"+project+"/confirmation-intents", map[string]any{"operation": "proposal.submit", "objectId": id, "reviewHash": snapshot["reviewHash"], "payload": map[string]any{"expectedVersion": version}}, &result, requestKey())
		if err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, c, "/projects/"+project+"/confirmation-intents", result)
	}}
	cmd.Flags().StringVar(&id, "id", "", "proposal ID")
	cmd.Flags().StringVar(&review, "review", "", "review hash")
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
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/confirmation-intents", intent, &created, requestKey()); err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, c, "/projects/"+projectID+"/confirmation-intents", created)
	}}
}

func handoffSendCommand() *cobra.Command {
	var version int
	cmd := &cobra.Command{Use: "send SOURCE --version N", Short: "发送来源（经确认意图）", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		_ = version
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		source, err := loadSource(cmd, c, projectID, args[0])
		if err != nil {
			return emit(nil, err)
		}
		if source.CurrentVersion != nil && *source.CurrentVersion != int64(version) {
			return emit(nil, fmt.Errorf("source version changed"))
		}
		var task map[string]any
		if err = c.Do(cmd.Context(), "GET", "/projects/"+projectID+"/tasks/"+source.TaskID, nil, &task, ""); err != nil {
			return emit(nil, err)
		}
		if task["latestReportId"] == nil {
			return emit(nil, fmt.Errorf("source has no delivery report"))
		}
		intent := map[string]any{
			"operation": "handoff.send", "objectId": args[0],
			"reviewHash": task["latestReportId"],
			"payload":    map[string]any{"summary": "由 CLI 提交"},
		}
		var created map[string]any
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/confirmation-intents", intent, &created, requestKey()); err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, c, "/projects/"+projectID+"/confirmation-intents", created)
	}}
	cmd.Flags().IntVar(&version, "version", 1, "source version")
	return cmd
}

func handoffReceiveCommand() *cobra.Command { return handoffDecisionCommand(true) }
func handoffRejectCommand() *cobra.Command  { return handoffDecisionCommand(false) }
func handoffDecisionCommand(defaultAccept bool) *cobra.Command {
	accept := defaultAccept
	name := "receive"
	if !defaultAccept {
		name = "reject"
	}
	var reason string
	cmd := &cobra.Command{Use: name + " SOURCE", Short: "接收/拒收来源（经确认意图）", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		projectID, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		source, err := loadSource(cmd, c, projectID, args[0])
		if err != nil {
			return emit(nil, err)
		}
		if source.CurrentVersionID == nil {
			return emit(nil, fmt.Errorf("source has not been sent"))
		}
		if !accept && reason == "" {
			return emit(nil, fmt.Errorf("rejection reason required"))
		}
		decision := "reject"
		if accept {
			decision = "accept"
		}
		intent := map[string]any{
			"operation": "handoff.decision", "objectId": args[0], "reviewHash": *source.CurrentVersionID,
			"payload": map[string]any{"decision": decision, "reason": reason},
		}
		var created map[string]any
		if err := c.Do(cmd.Context(), "POST", "/projects/"+projectID+"/confirmation-intents", intent, &created, requestKey()); err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, c, "/projects/"+projectID+"/confirmation-intents", created)
	}}
	cmd.Flags().StringVar(&reason, "reason", "", "rejection reason")
	if defaultAccept {
		cmd.Flags().BoolVar(&accept, "accept", true, "接收来源")
	}
	return cmd
}

func skillInstallCommand() *cobra.Command {
	var target string
	var path string
	cmd := &cobra.Command{Use: "install", Short: "安装发行技能到宿主目录", RunE: func(cmd *cobra.Command, args []string) error {
		source, err := findSkillSource()
		if err != nil {
			return emit(nil, err)
		}
		dest, err := skillTarget(target, path)
		if err != nil {
			return emit(nil, err)
		}
		if err := copySkill(source, dest); err != nil {
			return emit(nil, err)
		}
		return emit(map[string]string{"installedTo": dest, "source": source}, nil)
	}}
	cmd.Flags().StringVar(&target, "target", "codex", "codex | claude-code | path")
	cmd.Flags().StringVar(&path, "path", "", "自定义安装路径（--target path 时必填）")
	return cmd
}

func skillUninstallCommand() *cobra.Command {
	var target string
	var path string
	cmd := &cobra.Command{Use: "uninstall", Short: "按安装清单移除发行技能", RunE: func(cmd *cobra.Command, args []string) error {
		dest, err := skillTarget(target, path)
		if err != nil {
			return emit(nil, err)
		}
		if err := removeSkill(dest); err != nil {
			return emit(nil, err)
		}
		return emit(map[string]string{"removed": dest}, nil)
	}}
	cmd.Flags().StringVar(&target, "target", "codex", "codex | claude-code | path")
	cmd.Flags().StringVar(&path, "path", "", "自定义路径")
	return cmd
}

func findSkillSource() (string, error) {
	// Prefer a skills/ dir next to the binary (release layout), fall back to
	// the repository checkout.
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "skills", "judex")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	home, _ := os.UserHomeDir()
	for _, base := range []string{".judex", "go/src/github.com/kakj-go/Judex"} {
		candidate := filepath.Join(home, base, "skills", "judex")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	wd, _ := os.Getwd()
	candidate := filepath.Join(wd, "skills", "judex")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", errors.New("未找到 skills/judex（在发行包或仓库根目录运行）")
}

func skillTarget(target, custom string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch target {
	case "codex":
		return filepath.Join(home, ".agents", "skills", "judex"), nil
	case "claude-code":
		return filepath.Join(home, ".claude", "skills", "judex"), nil
	case "path":
		if custom == "" {
			return "", errors.New("--target path 需要 --path")
		}
		return custom, nil
	default:
		return "", errors.New("未知 target：" + target)
	}
}

func doctorCommand() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "诊断兼容性（协议/凭据/网络）", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var system map[string]any
		if err := c.Do(cmd.Context(), "GET", "/system", nil, &system, ""); err != nil {
			return emit(map[string]any{"ok": false, "server": c.Server, "issue": "system 端点不可达"}, err)
		}
		protocol, _ := system["protocolVersion"].(string)
		token, tokenErr := LoadToken()
		return emit(map[string]any{
			"ok": true, "server": c.Server, "serverVersion": system["version"],
			"serverProtocol": protocol, "cliProtocol": "1",
			"protocolCompatible": protocol == "1",
			"authenticated":      tokenErr == nil && token != "",
		}, nil)
	}}
}

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
	UserID    string `json:"userId,omitempty"`
	Server    string `json:"server"`
	ProjectID string `json:"projectId,omitempty"`
	GroupID   string `json:"groupId,omitempty"`
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
		server = "http://127.0.0.1:18097"
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
		out := Output{SchemaVersion: "2", OK: err == nil, Data: data}
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
		return &reportedError{cause: err}
	}
	return nil
}

func versionCommand() *cobra.Command {
	return &cobra.Command{Use: "version", Short: "协议与客户端版本", RunE: func(cmd *cobra.Command, args []string) error {
		return emit(map[string]string{"cli": version.Version, "commit": version.Commit, "protocol": "2"}, nil)
	}}
}

func statusCommand() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "服务可达性", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		if err = c.CheckProtocol(cmd.Context()); err != nil {
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
			scopes = []string{"projects:read", "context:read", "materials:read", "materials:write", "messages:write", "agent:request", "events:read"}
		}
		c.Token = ""
		c.TokenSource = nil
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

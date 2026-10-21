package cli

import (
	"errors"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

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

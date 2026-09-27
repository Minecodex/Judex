// SPDX-License-Identifier: Apache-2.0

package main

// Operator account subcommands (docs/plans/v1/02 §3): recovery codes and
// enable/disable. Deployment credentials are the authorization; every call is
// audited with the operator identity and reason. No business superuser.

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

func runAccountCommand() error {
	if flag.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: judex-server account <recovery-code|disable|enable> --email ... --reason ... [--operator name]")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("account commands require JUDEX_DATABASE_URL")
	}
	pool, err := postgres.Open(context.Background(), postgres.Options{URL: cfg.DatabaseURL}, nil)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Migrate(context.Background()); err != nil {
		return fmt.Errorf("migrations must be applied first: %w", err)
	}
	svc := identity.NewService(pool, nil, identity.Options{}, nil)

	sub := flag.NewFlagSet("account "+flag.Arg(1), flag.ExitOnError)
	email := sub.String("email", "", "target account email")
	reason := sub.String("reason", "", "audited reason (required)")
	operator := sub.String("operator", defaultOperator(), "operator identity recorded in the audit trail")
	if err := sub.Parse(flag.Args()[2:]); err != nil {
		return err
	}
	if *email == "" || *reason == "" {
		return fmt.Errorf("--email and --reason are required")
	}
	ctx := context.Background()
	switch flag.Arg(1) {
	case "recovery-code":
		code, err := svc.IssueRecoveryCode(ctx, *email, *operator, *reason)
		if err != nil {
			return err
		}
		fmt.Println("一次性恢复码（15 分钟内有效，仅本次显示，不自动登录）：")
		fmt.Println(code)
		fmt.Println("用户在 /recover 输入该码与新密码后，所有旧会话与授权将被撤销。")
		return nil
	case "disable":
		if err := svc.DisableAccount(ctx, *email, *operator, *reason); err != nil {
			return err
		}
		fmt.Println("账号已停用：登录拒绝，活跃会话与授权已撤销，历史保留。")
		return nil
	case "enable":
		if err := svc.EnableAccount(ctx, *email, *operator, *reason); err != nil {
			return err
		}
		fmt.Println("账号已恢复启用。")
		return nil
	default:
		return fmt.Errorf("unknown account subcommand %q", flag.Arg(1))
	}
}

func defaultOperator() string {
	if host, err := os.Hostname(); err == nil {
		if user := os.Getenv("USERNAME"); user != "" {
			return user + "@" + host
		}
		return host
	}
	return "unknown-operator"
}

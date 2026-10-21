package app

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/groupvolume"
	"github.com/kakj-go/Judex/internal/infrastructure/opensandbox"
	"strings"
	"time"
)

func (a *Application) VerifyGroupSandbox(ctx context.Context, group uuid.UUID) (map[string]any, error) {
	var snapshot int64
	if err := a.pool.QueryRow(ctx, `SELECT version FROM context_heads WHERE group_id=$1`, group).Scan(&snapshot); err != nil {
		return nil, err
	}
	if snapshot == 0 {
		return nil, fmt.Errorf("run a role first to create a shared context snapshot")
	}
	files, err := a.groupFiles(ctx, group, snapshot)
	if err != nil {
		return nil, err
	}
	cfg := a.Config.Sandbox
	projection, err := groupvolume.InCluster(cfg.MaterialNamespace, cfg.MaterialImage, cfg.MaterialStorageClass)
	if err != nil {
		return nil, err
	}
	run := uuid.New()
	defer projection.Remove(context.WithoutCancel(ctx), run)
	if err = projection.Prepare(ctx, group, run, files); err != nil {
		return nil, err
	}
	client := opensandbox.New(opensandbox.Config{Endpoint: cfg.Endpoint, APIKey: cfg.APIKey, Image: cfg.Image, CPU: cfg.CPU, Memory: cfg.Memory, GroupPVC: groupvolume.Claim(run)})
	external, err := client.Create(ctx, run, group)
	if err != nil {
		return nil, err
	}
	defer client.Kill(context.WithoutCancel(ctx), external)
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	script := "set -eu\n"
	for _, file := range files {
		script += "printf '%s  %s\\n' " + quote(file.SHA256) + " " + quote("/workspace/group/"+file.Path) + " | sha256sum -c - >/dev/null\n"
	}
	script += "if touch /workspace/group/.write-probe 2>/dev/null; then exit 21; fi\ntest ! -e /var/run/secrets/kubernetes.io/serviceaccount/token\nif wget -q -T 3 -O /tmp/network-probe http://1.1.1.1; then exit 22; fi\nprintf boundary-ok\n"
	result, err := client.Exec(ctx, external, script, 30*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Unknown || result.ExitCode != 0 || !strings.Contains(string(result.Stdout), "boundary-ok") {
		return nil, fmt.Errorf("sandbox boundary check failed (exit=%d unknown=%t)", result.ExitCode, result.Unknown)
	}
	return map[string]any{"groupId": group, "snapshotVersion": snapshot, "filesVerified": len(files), "readOnly": true, "serviceAccountTokenAbsent": true, "egressProbeBlocked": true}, nil
}

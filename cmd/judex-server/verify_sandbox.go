package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/infrastructure/objectstore"
	"github.com/kakj-go/Judex/internal/infrastructure/opensandbox"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/infrastructure/projectvolume"
	"os"
	"strings"
	"time"
)

// verifySandbox is an operator diagnostic using the same material preparation
// and sandbox adapter as production. It never prints credentials or file data.
func verifySandbox(projectRaw string) error {
	project, err := uuid.Parse(projectRaw)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	pool, err := postgres.Open(ctx, postgres.Options{URL: cfg.DatabaseURL}, nil)
	if err != nil {
		return err
	}
	defer pool.Close()
	store, err := objectstore.New(ctx, objectstore.Options{Endpoint: cfg.ObjectStorage.Endpoint, Region: cfg.ObjectStorage.Region, AccessKeyID: cfg.ObjectStorage.AccessKeyID, SecretAccessKey: cfg.ObjectStorage.SecretAccessKey, Bucket: cfg.ObjectStorage.Bucket, PathStyle: cfg.ObjectStorage.PathStyle})
	if err != nil {
		return err
	}
	projection, err := projectvolume.InCluster(cfg.Sandbox.MaterialNamespace, cfg.Sandbox.MaterialImage, cfg.Sandbox.MaterialStorageClass)
	if err != nil {
		return err
	}
	executor := batch.Executor{Pool: pool.Pool, Objects: store}
	files, err := executor.ProjectFiles(ctx, project)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("upload a material before verifying the sandbox")
	}
	run := uuid.New()
	if err = projection.Prepare(ctx, project, run, files); err != nil {
		return err
	}
	client := opensandbox.New(opensandbox.Config{Endpoint: cfg.Sandbox.Endpoint, APIKey: cfg.Sandbox.APIKey, Image: cfg.Sandbox.Image, CPU: cfg.Sandbox.CPU, Memory: cfg.Sandbox.Memory, ProjectPVC: projectvolume.Claim(project)})
	sandbox, err := client.Create(ctx, run, project)
	if err != nil {
		return err
	}
	defer client.Kill(context.WithoutCancel(ctx), sandbox)
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	script := "set -eu\n"
	for _, file := range files {
		script += "printf '%s  %s\\n' " + quote(file.SHA256) + " " + quote("/workspace/project/"+file.Path) + " | sha256sum -c - >/dev/null\n"
	}
	script += "if touch /workspace/project/.judex-write-probe 2>/dev/null; then exit 21; fi\ntest ! -e /var/run/secrets/kubernetes.io/serviceaccount/token\nif wget -q -T 3 -O /tmp/judex-network-probe http://1.1.1.1; then exit 22; fi\nprintf 'boundary-ok'\n"
	result, err := client.Exec(ctx, sandbox, script, 30*time.Second)
	if err != nil {
		return err
	}
	if result.Unknown || result.ExitCode != 0 || !strings.Contains(string(result.Stdout), "boundary-ok") {
		return fmt.Errorf("sandbox boundary check failed (exit=%d unknown=%t)", result.ExitCode, result.Unknown)
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"projectId": project, "sandboxId": sandbox, "filesVerified": len(files), "readOnly": true, "serviceAccountTokenAbsent": true, "egressProbeBlocked": true})
}

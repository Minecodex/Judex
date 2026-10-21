package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/peer"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/infrastructure/groupvolume"
	"github.com/kakj-go/Judex/internal/infrastructure/opensandbox"
)

func (a *Application) wireSandbox(executor *peer.Executor) error {
	cfg := a.Config.Sandbox
	if cfg.Endpoint == "" {
		return nil
	}
	var projection *groupvolume.Client
	if cfg.MaterialNamespace != "" {
		var err error
		projection, err = groupvolume.InCluster(cfg.MaterialNamespace, cfg.MaterialImage, cfg.MaterialStorageClass)
		if err != nil {
			return err
		}
	}
	executor.RefreshProjection = func(ctx context.Context, p collab.Publication, snapshot int64) error {
		if projection == nil {
			return nil
		}
		files, err := a.groupFiles(ctx, p.GroupID, snapshot)
		if err != nil {
			return err
		}
		tag, err := a.pool.Exec(ctx, `UPDATE role_runs SET projection_claim=$3,projection_removed=false WHERE id=$1 AND fence=$2 AND state='running' AND EXISTS(SELECT 1 FROM discussions d WHERE d.id=role_runs.discussion_id AND d.state='active')`, p.RunID, p.Fence, groupvolume.Claim(p.RunID))
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("projection rejected after run revocation or pause")
		}
		return projection.Prepare(ctx, p.GroupID, p.RunID, files)
	}
	executor.SandboxFactory = func(ctx context.Context, p collab.Publication, snapshot int64) (*opensandbox.RunSandbox, error) {
		options := opensandbox.Config{Endpoint: cfg.Endpoint, APIKey: cfg.APIKey, Image: cfg.Image, CPU: cfg.CPU, Memory: cfg.Memory}
		sandbox := &opensandbox.RunSandbox{RunID: p.RunID, GroupID: p.GroupID}
		if projection != nil {
			// A run receives one immutable group snapshot. The claim never contains a
			// different group's files, or versions granted only after this snapshot.
			options.GroupPVC = groupvolume.Claim(p.RunID)
			sandbox.BeforeCreate = func(ctx context.Context) error { return executor.RefreshProjection(ctx, p, snapshot) }
		}
		sandbox.Client = opensandbox.New(options)
		sandbox.AfterCreate = func(ctx context.Context, id string) error {
			tag, err := a.pool.Exec(ctx, `UPDATE role_runs SET sandbox_id=$3 WHERE id=$1 AND fence=$2 AND state='running' AND EXISTS(SELECT 1 FROM discussions d WHERE d.id=role_runs.discussion_id AND d.state='active')`, p.RunID, p.Fence, id)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("sandbox creation lease lost before command dispatch")
			}
			return nil
		}
		sandbox.AfterClose = func(ctx context.Context, err error) {
			if err == nil {
				_, _ = a.pool.Exec(ctx, `UPDATE role_runs SET sandbox_id=NULL WHERE id=$1 AND sandbox_id=$2`, p.RunID, sandbox.ExternalID())
				if projection != nil {
					if projection.Remove(ctx, p.RunID) == nil {
						_, _ = a.pool.Exec(ctx, `UPDATE role_runs SET projection_removed=true WHERE id=$1`, p.RunID)
					}
				}
			}
		}
		var external *string
		if err := a.pool.QueryRow(ctx, `SELECT sandbox_id FROM role_runs WHERE id=$1`, p.RunID).Scan(&external); err != nil {
			return nil, err
		}
		if external != nil {
			sandbox.Attach(*external)
		}
		return sandbox, nil
	}
	return nil
}
func (a *Application) groupFiles(ctx context.Context, group uuid.UUID, snapshot int64) ([]groupvolume.File, error) {
	signer, ok := a.collab.Objects.(interface {
		PresignGet(context.Context, string, time.Duration) (string, error)
	})
	if !ok {
		return nil, fmt.Errorf("object store cannot issue scoped snapshot read URLs")
	}
	rows, err := a.pool.Query(ctx, `SELECT DISTINCT v.id,v.object_key,v.sha256 FROM knowledge_entries k JOIN file_versions v ON v.id::text=k.ref_id WHERE k.group_id=$1 AND k.kind='file' AND k.seq<=(SELECT source_seq FROM context_snapshots WHERE group_id=$1 AND version=$2) ORDER BY v.id`, group, snapshot)
	if err != nil {
		return nil, err
	}
	type item struct {
		id        uuid.UUID
		key, hash string
	}
	items := []item{}
	for rows.Next() {
		var file item
		if err = rows.Scan(&file.id, &file.key, &file.hash); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, file)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []groupvolume.File{}
	for _, file := range items {
		url, err := signer.PresignGet(ctx, file.key, 10*time.Minute)
		if err != nil {
			return nil, err
		}
		out = append(out, groupvolume.File{Path: "versions/" + file.id.String() + "/original", SHA256: file.hash, Parts: []groupvolume.Part{{URL: url, SHA256: file.hash}}})
	}
	return out, nil
}
func (a *Application) maintain(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.collab.CleanupUploads(ctx); err != nil {
				logger.Warn("upload staging cleanup deferred")
			}
			// Terminal sandboxes left by a process crash are reclaimed independently of
			// model delivery; paused runs retain their workspace for reconciliation.
			if a.Config.Sandbox.Endpoint == "" {
				continue
			}
			rows, err := a.pool.Query(ctx, `SELECT id,COALESCE(sandbox_id,''),projection_claim IS NOT NULL AND NOT projection_removed FROM role_runs WHERE state IN('completed','cancelled') AND (sandbox_id IS NOT NULL OR (projection_claim IS NOT NULL AND NOT projection_removed)) LIMIT 20`)
			if err != nil {
				logger.Warn("sandbox cleanup query failed")
				continue
			}
			type orphan struct {
				id         uuid.UUID
				external   string
				projection bool
			}
			items := []orphan{}
			for rows.Next() {
				var item orphan
				if rows.Scan(&item.id, &item.external, &item.projection) == nil {
					items = append(items, item)
				}
			}
			rows.Close()
			client := opensandbox.New(opensandbox.Config{Endpoint: a.Config.Sandbox.Endpoint, APIKey: a.Config.Sandbox.APIKey})
			for _, item := range items {
				if item.external != "" {
					if err = client.Kill(ctx, item.external); err != nil {
						continue
					}
					_, _ = a.pool.Exec(ctx, `UPDATE role_runs SET sandbox_id=NULL WHERE id=$1 AND sandbox_id=$2`, item.id, item.external)
				}
				if item.projection {
					projection, err := groupvolume.InCluster(a.Config.Sandbox.MaterialNamespace, a.Config.Sandbox.MaterialImage, a.Config.Sandbox.MaterialStorageClass)
					if err == nil && projection.Remove(ctx, item.id) == nil {
						_, _ = a.pool.Exec(ctx, `UPDATE role_runs SET projection_removed=true WHERE id=$1`, item.id)
					}
				}
			}
		}
	}
}

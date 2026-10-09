package material

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/audit"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"strings"
)

type Publication struct {
	Name             string      `json:"name"`
	Kind             string      `json:"kind"`
	Mime             string      `json:"mime"`
	Entrypoint       string      `json:"entrypoint"`
	SourceVersionIDs []uuid.UUID `json:"sourceVersionIds"`
	MaterialID       *uuid.UUID  `json:"materialId"`
	BaseVersionID    *uuid.UUID  `json:"baseVersionId"`
	Key              string      `json:"idempotencyKey"`
}

// PublishInTx is called only after the runtime project/lease/binding guard.
// Object bytes are immutable; a failed DB commit can leave only an unreferenced
// object. A repeated command cannot create another material version.
func (s *Service) PublishInTx(ctx context.Context, tx pgx.Tx, project, run uuid.UUID, in Publication, raw []byte) (MaterialVersion, error) {
	var out MaterialVersion
	if strings.TrimSpace(in.Name) == "" || in.Key == "" || len(in.SourceVersionIDs) == 0 {
		return out, apierrors.Fields("publication", "name/key/sourceVersionIds required")
	}
	if int64(len(raw)) > s.limits.MaxFileBytes {
		return out, apierrors.New(apierrors.PayloadTooLarge, "artifact too large")
	}
	if in.Kind == "" {
		in.Kind = "file"
	}
	if in.Kind != "file" && in.Kind != "html_bundle" {
		return out, apierrors.Fields("kind", "enum")
	}
	if in.Mime == "" {
		in.Mime = "application/octet-stream"
	}
	if in.Kind == "html_bundle" && in.Entrypoint == "" {
		in.Entrypoint = "index.html"
	}
	checksum := sha256.Sum256(raw)
	digest := hex.EncodeToString(checksum[:])
	description, _ := json.Marshal(in)
	requestSum := sha256.Sum256(append(description, checksum[:]...))
	requestHash := hex.EncodeToString(requestSum[:])
	var existing uuid.UUID
	var oldHash string
	err := tx.QueryRow(ctx, `SELECT material_version_id,request_hash FROM artifact_publications WHERE run_id=$1 AND command_key=$2`, run, in.Key).Scan(&existing, &oldHash)
	if err == nil {
		if oldHash != requestHash {
			return out, apierrors.New(apierrors.IdempotencyConflict, "publication key used for different bytes or sources")
		}
		err = s.loadVersionTx(ctx, tx, project, existing, &out)
		return out, err
	}
	if err != pgx.ErrNoRows {
		return out, err
	}
	for _, source := range in.SourceVersionIDs {
		var current bool
		err = tx.QueryRow(ctx, `SELECT m.current_version_id=v.id AND v.state='ready' FROM material_versions v JOIN materials m ON m.id=v.material_id AND m.project_id=v.project_id WHERE m.deleted_at IS NULL AND v.id=$1 AND v.project_id=$2`, source, project).Scan(&current)
		if err != nil || !current {
			return out, apierrors.New(apierrors.SourceVersionConf, "source version is no longer current in project")
		}
	}
	materialID := uuid.New()
	revision := 1
	if in.MaterialID != nil {
		materialID = *in.MaterialID
		var current *uuid.UUID
		var kind string
		err = tx.QueryRow(ctx, `SELECT current_version_id,kind FROM materials WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, materialID, project).Scan(&current, &kind)
		if err != nil || current == nil || in.BaseVersionID == nil || *current != *in.BaseVersionID || kind != in.Kind {
			return out, apierrors.New(apierrors.SourceVersionConf, "target base version changed")
		}
		if err = tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0)+1 FROM material_versions WHERE material_id=$1`, materialID).Scan(&revision); err != nil {
			return out, err
		}
	}
	version := uuid.New()
	staging := "publications/" + project.String() + "/" + run.String() + "/" + version.String()
	objectKey := partKey(staging, 1)
	if s.store == nil {
		return out, apierrors.New(apierrors.DependencyDown, "object storage unavailable")
	}
	if err = s.store.Put(ctx, objectKey, bytes.NewReader(raw), int64(len(raw)), in.Mime); err != nil {
		return out, err
	}
	now := s.now()
	if in.MaterialID == nil {
		if _, err = tx.Exec(ctx, `INSERT INTO materials(project_id,id,title,kind,visibility,created_at,updated_at) VALUES($1,$2,$3,$4,'project',$5,$5)`, project, materialID, in.Name, in.Kind, now); err != nil {
			return out, err
		}
	}
	sources, _ := json.Marshal(map[string]any{"sourceVersionIds": in.SourceVersionIDs, "runId": run})
	var entrypoint *string
	if in.Kind == "html_bundle" {
		entrypoint = &in.Entrypoint
	}
	if _, err = tx.Exec(ctx, `INSERT INTO material_versions(project_id,id,material_id,revision,manifest_key,sha256,size,mime,entrypoint,run_id,source_manifest_json,state,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'ready',$12)`, project, version, materialID, revision, staging+"/manifest.json", digest, len(raw), in.Mime, entrypoint, run, sources, now); err != nil {
		return out, err
	}
	if in.Kind == "html_bundle" {
		if err = s.registerBundle(ctx, tx, project, version, staging, 1, in.Entrypoint); err != nil {
			return out, err
		}
	} else {
		if _, err = tx.Exec(ctx, `INSERT INTO material_entries(project_id,version_id,relative_path,object_key,size,sha256,mime) VALUES($1,$2,'content',$3,$4,$5,$6)`, project, version, objectKey, len(raw), digest, in.Mime); err != nil {
			return out, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE materials SET current_version_id=$2,updated_at=$3 WHERE id=$1`, materialID, version, now); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO artifact_publications(project_id,run_id,command_key,request_hash,material_version_id,created_at) VALUES($1,$2,$3,$4,$5,$6)`, project, run, in.Key, requestHash, version, now); err != nil {
		return out, err
	}
	if err = s.loadVersionTx(ctx, tx, project, version, &out); err != nil {
		return out, err
	}
	manifest, _ := json.Marshal(out)
	if err = s.store.Put(ctx, staging+"/manifest.json", bytes.NewReader(manifest), int64(len(manifest)), "application/json"); err != nil {
		return out, err
	}
	if _, err = events.AppendProjectEvent(ctx, tx, project, "material.ready", "material", materialID.String(), nil, map[string]any{"versionId": version, "runId": run}, now); err != nil {
		return out, err
	}
	err = audit.Append(ctx, tx, audit.Entry{ProjectID: &project, ActorType: audit.ActorAgent, Source: audit.SourceAgent, Operation: "material.publish", ObjectType: "material_version", ObjectID: version.String(), OccurredAt: now})
	return out, err
}

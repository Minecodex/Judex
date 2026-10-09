// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// PersonalPreferences belongs to one person and one position in a project.
// Several identities of the same position use that person's same preference.
type PersonalPreferences struct {
	PositionID uuid.UUID `json:"positionId"`
	Revision   int64     `json:"revision"`
	Prompt     string    `json:"prompt"`
}

// GetMyPreferences includes unsaved positions at revision 0, and never exposes
// another person's preferences or a position the requester no longer holds.
func (s *Service) GetMyPreferences(ctx context.Context, requester, projectID uuid.UUID) ([]PersonalPreferences, error) {
	if _, err := s.MembershipFor(ctx, requester, projectID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, COALESCE(pref.revision,0), COALESCE(pref.prompt,'')
		FROM position_templates t
		LEFT JOIN personal_position_preferences pref
		  ON pref.project_id=t.project_id AND pref.position_id=t.id AND pref.user_id=$2
		WHERE t.project_id=$1 AND t.status='active' AND EXISTS (
		  SELECT 1 FROM agent_identities i JOIN identity_bindings b
		    ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL
		  WHERE i.project_id=t.project_id AND i.template_id=t.id AND i.kind='position'
		    AND i.status='active' AND b.user_id=$2)
		ORDER BY t.created_at,t.id`, projectID, requester)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "preferences failed").Wrap(err)
	}
	defer rows.Close()
	out := []PersonalPreferences{}
	for rows.Next() {
		var preference PersonalPreferences
		if err := rows.Scan(&preference.PositionID, &preference.Revision, &preference.Prompt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "preferences scan failed").Wrap(err)
		}
		out = append(out, preference)
	}
	return out, rows.Err()
}

func (s *Service) UpdateMyPreferences(ctx context.Context, requester, projectID, positionID uuid.UUID, expectedRevision int64, prompt string) (PersonalPreferences, error) {
	if positionID == uuid.Nil {
		return PersonalPreferences{}, apierrors.Fields("positionId", "required")
	}
	if expectedRevision < 0 {
		return PersonalPreferences{}, apierrors.Fields("expectedRevision", "minimum")
	}
	if utf8.RuneCountInString(prompt) > 20000 {
		return PersonalPreferences{}, apierrors.Fields("prompt", "length")
	}
	var out PersonalPreferences
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := s.MembershipForTx(ctx, tx, requester, projectID); err != nil {
			return err
		}
		var assigned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
		  SELECT 1 FROM position_templates t JOIN agent_identities i ON i.template_id=t.id AND i.project_id=t.project_id
		  JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL
		  WHERE t.project_id=$1 AND t.id=$3 AND t.status='active' AND i.kind='position' AND i.status='active' AND b.user_id=$2
		)`, projectID, requester, positionID).Scan(&assigned); err != nil {
			return apierrors.New(apierrors.Internal, "preference position lookup failed").Wrap(err)
		}
		if !assigned {
			return apierrors.New(apierrors.Forbidden, "preferences require a currently assigned position")
		}
		var current int64
		err := tx.QueryRow(ctx, `SELECT revision FROM personal_position_preferences
		  WHERE project_id=$1 AND user_id=$2 AND position_id=$3 FOR UPDATE`, projectID, requester, positionID).Scan(&current)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return apierrors.New(apierrors.Internal, "preference lookup failed").Wrap(err)
		}
		if current != expectedRevision {
			return apierrors.New(apierrors.VersionConflict, "preferences revision conflict")
		}
		now := s.now()
		if _, err := tx.Exec(ctx, `INSERT INTO personal_position_preferences (project_id,user_id,position_id,revision,prompt,updated_at)
		  VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (project_id,user_id,position_id)
		  DO UPDATE SET revision=EXCLUDED.revision,prompt=EXCLUDED.prompt,updated_at=EXCLUDED.updated_at`,
			projectID, requester, positionID, current+1, prompt, now); err != nil {
			return apierrors.New(apierrors.Internal, "preference update failed").Wrap(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO position_preference_revisions (project_id,user_id,position_id,revision,prompt,created_at)
		  VALUES ($1,$2,$3,$4,$5,$6)`, projectID, requester, positionID, current+1, prompt, now); err != nil {
			return apierrors.New(apierrors.Internal, "preference revision insert failed").Wrap(err)
		}
		out = PersonalPreferences{PositionID: positionID, Revision: current + 1, Prompt: prompt}
		return nil
	})
	return out, err
}

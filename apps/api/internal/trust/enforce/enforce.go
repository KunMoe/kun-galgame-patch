package enforce

import (
	"context"
	"log/slog"
	"strconv"

	"kun-galgame-patch-api/internal/trust/dto"

	"gorm.io/gorm"
)

const (
	ActionNone        int16 = 0
	ActionHide        int16 = 1
	ActionRemove      int16 = 2
	ActionWarnUser    int16 = 3
	ActionRestrict    int16 = 4
	ActionEscalateIdp int16 = 5
)

// An Adapter must treat a subject that no longer exists as enforced: a Remove
// for a resource its author deleted first used to 500 through infra's whole
// retry schedule and end dead-lettered.
type Adapter struct {
	Hide     func(ctx context.Context, id int) error
	Remove   func(ctx context.Context, id int) error
	Restore  func(ctx context.Context, id int) error
	AuthorID func(ctx context.Context, id int) (int, error)
}

type Registry map[string]Adapter

type WarnFunc func(ctx context.Context, userID int, reasonCode string) error

type Service struct {
	db       *gorm.DB
	registry Registry
	warn     WarnFunc
}

func NewService(db *gorm.DB, registry Registry, warn WarnFunc) *Service {
	return &Service{db: db, registry: registry, warn: warn}
}

// Apply claims the disposition in the transaction that runs it: a redelivery
// racing this one blocks on the ledger row, and a failed action rolls the claim
// back for infra's retry. Infra retries a failure on a backoff but delivers
// newer dispositions at once, so a retried Hide could land after the Dismiss
// that cleared it; a visibility action older than the newest one applied to its
// subject is acknowledged and not applied.
func (s *Service) Apply(ctx context.Context, cb dto.TrustCallback) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		claim := tx.Exec(
			"INSERT INTO trust_disposition_applied (disposition_id, action) VALUES (?, ?) ON CONFLICT DO NOTHING",
			cb.DispositionID, cb.Action,
		)
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			return nil
		}

		if setsVisibility(cb.Action) {
			newest := tx.Exec(`
				INSERT INTO trust_subject_watermark (subject_kind, subject_id, disposition_id)
				VALUES (?, ?, ?)
				ON CONFLICT (subject_kind, subject_id) DO UPDATE
				SET disposition_id = EXCLUDED.disposition_id, updated_at = now()
				WHERE trust_subject_watermark.disposition_id < EXCLUDED.disposition_id`,
				cb.SubjectKind, cb.SubjectID, cb.DispositionID,
			)
			if newest.Error != nil {
				return newest.Error
			}
			if newest.RowsAffected == 0 {
				slog.Info("trust disposition superseded by a newer one for its subject",
					"disposition_id", cb.DispositionID, "subject_kind", cb.SubjectKind,
					"subject_id", cb.SubjectID, "action", cb.Action)
				return nil
			}
		}

		if err := s.dispatch(ctx, cb); err != nil {
			return err
		}
		slog.Info("trust disposition applied",
			"disposition_id", cb.DispositionID, "subject_kind", cb.SubjectKind,
			"subject_id", cb.SubjectID, "action", cb.Action, "reason_code", cb.ReasonCode)
		return nil
	})
}

func setsVisibility(action int16) bool {
	return action == ActionHide || action == ActionRemove || action == ActionNone
}

func (s *Service) dispatch(ctx context.Context, cb dto.TrustCallback) error {
	id, err := strconv.Atoi(cb.SubjectID)
	if err != nil {
		slog.Warn("trust callback: non-numeric subject_id",
			"subject_id", cb.SubjectID, "disposition_id", cb.DispositionID)
		return nil
	}
	adapter, hasAdapter := s.registry[cb.SubjectKind]

	switch cb.Action {
	case ActionHide:
		if hasAdapter && adapter.Hide != nil {
			return adapter.Hide(ctx, id)
		}
	case ActionRemove:
		if hasAdapter && adapter.Remove != nil {
			return adapter.Remove(ctx, id)
		}
	case ActionWarnUser:
		if hasAdapter && adapter.AuthorID != nil && s.warn != nil {
			authorID, err := adapter.AuthorID(ctx, id)
			if err != nil {
				return err
			}
			if authorID > 0 {
				return s.warn(ctx, authorID, cb.ReasonCode)
			}
		}
	case ActionNone:
		if hasAdapter && adapter.Restore != nil {
			return adapter.Restore(ctx, id)
		}
	case ActionRestrict, ActionEscalateIdp:
	}

	slog.Info("trust callback: no local enforcement",
		"subject_kind", cb.SubjectKind, "action", cb.Action, "disposition_id", cb.DispositionID)
	return nil
}

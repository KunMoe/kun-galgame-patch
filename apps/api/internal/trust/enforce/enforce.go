// Package enforce applies infra Trust & Safety enforcement dispositions to moyu
// content. It is the "thin adapter" half of the design: the dispatch pipeline is
// generic; each content type contributes ONE Adapter entry wiring
// hide/remove/restore/author-lookup to its existing domain service (assembled in
// app.go). Adding a reportable type costs one registry entry — no new subsystem.
package enforce

import (
	"context"
	"log/slog"
	"strconv"

	"kun-galgame-patch-api/internal/trust/dto"

	"gorm.io/gorm"
)

// Trust disposition action codes (the wire enum from the callback body — NEVER
// renumber; mirrors the trust service model constants).
const (
	ActionNone        int16 = 0
	ActionHide        int16 = 1
	ActionRemove      int16 = 2
	ActionWarnUser    int16 = 3
	ActionRestrict    int16 = 4
	ActionEscalateIdp int16 = 5
)

// Adapter applies enforcement to one content type. Every method MUST be
// idempotent and MUST NOT resurrect content the author already deleted (a hide
// on a gone/hidden row is a no-op; remove on a gone row is a no-op).
type Adapter struct {
	// Hide soft-hides the subject (reversible). No-op if gone/hidden.
	Hide func(ctx context.Context, id int) error
	// Remove hard-deletes the subject. No-op if already gone.
	Remove func(ctx context.Context, id int) error
	// Restore reverses a moderation Hide on a DISMISS / action=none callback.
	// Optional: only wire it when the hidden state is UNAMBIGUOUSLY a mod-hide
	// (moyu resources use a dedicated status=2, so restoring can't resurrect an
	// author/verify-hidden row). Leave nil for content whose hidden state is
	// ambiguous (comments share status=1 with verify-pending) — then a dismiss
	// is record-only, never a restore.
	Restore func(ctx context.Context, id int) error
	// AuthorID returns the subject's author user id (for warn_user), 0 if gone.
	AuthorID func(ctx context.Context, id int) (int, error)
}

// Registry maps subject_kind → Adapter. A subject_kind with no adapter (e.g.
// "user") is human-only enforcement (via the IdP) — its callbacks no-op locally.
type Registry map[string]Adapter

// WarnFunc delivers a "your content was actioned" notice to a user.
type WarnFunc func(ctx context.Context, userID int, reasonCode string) error

// Service is the generic enforcement dispatcher.
type Service struct {
	db       *gorm.DB
	registry Registry
	warn     WarnFunc
}

func NewService(db *gorm.DB, registry Registry, warn WarnFunc) *Service {
	return &Service{db: db, registry: registry, warn: warn}
}

// Apply enforces one disposition idempotently. A disposition already recorded in
// trust_disposition_applied is a no-op (replay-safe, matters for warn_user). The
// record is written only AFTER a successful dispatch, so a failed dispatch is
// retried by the trust worker rather than silently marked done.
func (s *Service) Apply(ctx context.Context, cb dto.TrustCallback) error {
	var exists bool
	if err := s.db.WithContext(ctx).
		Raw("SELECT EXISTS(SELECT 1 FROM trust_disposition_applied WHERE disposition_id = ?)", cb.DispositionID).
		Scan(&exists).Error; err != nil {
		return err
	}
	if exists {
		return nil // replay → no-op
	}

	if err := s.dispatch(ctx, cb); err != nil {
		return err // trust worker retries
	}

	return s.db.WithContext(ctx).Exec(
		"INSERT INTO trust_disposition_applied (disposition_id, action) VALUES (?, ?) ON CONFLICT DO NOTHING",
		cb.DispositionID, cb.Action,
	).Error
}

func (s *Service) dispatch(ctx context.Context, cb dto.TrustCallback) error {
	id, err := strconv.Atoi(cb.SubjectID)
	if err != nil {
		// All moyu subject ids are numeric; a non-numeric id is nothing we can act
		// on — log and treat as enforced so the worker doesn't dead-letter.
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
		// A dismiss / release-hold. Restore ONLY for content whose hidden state is
		// unambiguously a mod-hide (adapter.Restore set); otherwise record-only, so
		// a dismiss can't resurrect author/verify-hidden content.
		if hasAdapter && adapter.Restore != nil {
			return adapter.Restore(ctx, id)
		}
	case ActionRestrict, ActionEscalateIdp:
		// Record-only: account-level actions go through the OAuth IdP.
	}

	// Unsupported (subject_kind has no adapter, or the action isn't enforceable on
	// it) → log + succeed so the trust worker doesn't dead-letter.
	slog.Info("trust callback: no local enforcement",
		"subject_kind", cb.SubjectKind, "action", cb.Action, "disposition_id", cb.DispositionID)
	return nil
}

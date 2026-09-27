package activitypush

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type presentationClaim struct {
	AnchorKind int32
	AnchorID   string
	EnqueuedAt time.Time
}

func (c presentationClaim) anchorRef() anchorRef {
	return anchorRef{Kind: c.AnchorKind, ID: c.AnchorID}
}

// presentationSent is what community last accepted per anchor
// (anchor_presentation_sent).
type presentationSent struct {
	AnchorKind int32  `gorm:"primaryKey"`
	AnchorID   string `gorm:"primaryKey"`
	Revision   int64
	Removed    bool
	SentAt     time.Time `gorm:"autoUpdateTime"`
}

func (presentationSent) TableName() string { return "anchor_presentation_sent" }

type presentationQueueRow struct {
	AnchorKind int32  `gorm:"primaryKey"`
	AnchorID   string `gorm:"primaryKey"`
}

func (presentationQueueRow) TableName() string { return "anchor_presentation_queue" }

func (s store) claimPresentations(ctx context.Context, limit int) ([]presentationClaim, error) {
	var out []presentationClaim
	err := s.db.WithContext(ctx).Raw(`
		SELECT anchor_kind, anchor_id, enqueued_at FROM anchor_presentation_queue
		ORDER BY enqueued_at, anchor_kind, anchor_id LIMIT ?`, limit,
	).Scan(&out).Error
	return out, err
}

func (s store) presentationsSent(ctx context.Context, refs []anchorRef) (map[anchorRef]presentationSent, error) {
	out := make(map[anchorRef]presentationSent, len(refs))
	if len(refs) == 0 {
		return out, nil
	}
	pairs := make([][]any, len(refs))
	for i, a := range refs {
		pairs[i] = []any{a.Kind, a.ID}
	}
	var rows []presentationSent
	if err := s.db.WithContext(ctx).Where("(anchor_kind, anchor_id) IN ?", pairs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[anchorRef{Kind: r.AnchorKind, ID: r.AnchorID}] = r
	}
	return out, nil
}

// recordPresentations keeps the newest accepted revision per anchor, as record
// does for activities.
func (s store) recordPresentations(ctx context.Context, rows []presentationSent) error {
	if len(rows) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "anchor_kind"}, {Name: "anchor_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"revision", "removed", "sent_at"}),
		Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "anchor_presentation_sent.revision <= EXCLUDED.revision"}}},
	}).CreateInBatches(rows, 500).Error
}

func (s store) ackPresentations(ctx context.Context, claims []presentationClaim) error {
	if len(claims) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range claims {
			if err := tx.Exec(`DELETE FROM anchor_presentation_queue
				WHERE anchor_kind = ? AND anchor_id = ? AND enqueued_at = ?`,
				c.AnchorKind, c.AnchorID, c.EnqueuedAt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// enqueuePresentations queues anchors the reconcile found drifted.
func (s store) enqueuePresentations(ctx context.Context, refs []anchorRef) error {
	rows := presentationRows(refs)
	if len(rows) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "anchor_kind"}, {Name: "anchor_id"}},
		DoUpdates: clause.Assignments(map[string]any{"enqueued_at": gorm.Expr("clock_timestamp()")}),
	}).CreateInBatches(rows, 500).Error
}

// enqueueUnpresented queues the anchors community holds no live presentation
// for. An anchor already waiting keeps its place.
func (s store) enqueueUnpresented(ctx context.Context, refs []anchorRef) (int, error) {
	sent, err := s.presentationsSent(ctx, refs)
	if err != nil {
		return 0, err
	}
	var missing []anchorRef
	for _, a := range refs {
		if prev, ok := sent[a]; !ok || prev.Removed {
			missing = append(missing, a)
		}
	}
	rows := presentationRows(missing)
	if len(rows) == 0 {
		return 0, nil
	}
	res := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 500)
	return int(res.RowsAffected), res.Error
}

func presentationRows(refs []anchorRef) []presentationQueueRow {
	rows := make([]presentationQueueRow, 0, len(refs))
	seen := make(map[anchorRef]bool, len(refs))
	for _, a := range refs {
		if !seen[a] {
			seen[a] = true
			rows = append(rows, presentationQueueRow{AnchorKind: a.Kind, AnchorID: a.ID})
		}
	}
	slices.SortFunc(rows, func(a, b presentationQueueRow) int {
		return cmp.Or(cmp.Compare(a.AnchorKind, b.AnchorKind), strings.Compare(a.AnchorID, b.AnchorID))
	})
	return rows
}

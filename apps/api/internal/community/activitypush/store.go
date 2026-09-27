package activitypush

import (
	"context"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type claim struct {
	Key        string
	Backfill   bool
	EnqueuedAt time.Time
}

// sentRow is what community last accepted for a key (activity_push_sent).
type sentRow struct {
	Key      string `gorm:"primaryKey"`
	ActorID  int
	Revision int64
	Removed  bool
	SentAt   time.Time `gorm:"autoUpdateTime"`
}

func (sentRow) TableName() string { return "activity_push_sent" }

type store struct{ db *gorm.DB }

func (s store) claim(ctx context.Context, limit int) ([]claim, error) {
	var out []claim
	err := s.db.WithContext(ctx).Raw(
		`SELECT key, backfill, enqueued_at FROM activity_push_queue ORDER BY enqueued_at, key LIMIT ?`, limit,
	).Scan(&out).Error
	return out, err
}

func (s store) sent(ctx context.Context, keys []string) (map[string]sentRow, error) {
	out := make(map[string]sentRow, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	var rows []sentRow
	if err := s.db.WithContext(ctx).Where("key IN ?", keys).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.Key] = r
	}
	return out, nil
}

// record keeps the newest accepted revision per key; the reconcile writes the
// same rows from community's own listing, and neither may move one back.
func (s store) record(ctx context.Context, rows []sentRow) error {
	if len(rows) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"actor_id", "revision", "removed", "sent_at"}),
		Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "activity_push_sent.revision <= EXCLUDED.revision"}}},
	}).CreateInBatches(rows, 500).Error
}

// ack deletes the claimed rows that were not enqueued again while the tick ran:
// a later enqueue moved enqueued_at, and that change still has to be pushed.
func (s store) ack(ctx context.Context, claims []claim) error {
	if len(claims) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range claims {
			if err := tx.Exec(`DELETE FROM activity_push_queue WHERE key = ? AND enqueued_at = ?`,
				c.Key, c.EnqueuedAt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type queueRow struct {
	Key      string `gorm:"primaryKey"`
	Backfill bool
}

func (queueRow) TableName() string { return "activity_push_queue" }

// enqueueBackfill queues keys the reconcile found drifted. A row a trigger
// already queued keeps its backfill=false.
func (s store) enqueueBackfill(ctx context.Context, keys []string) error {
	rows := make([]queueRow, 0, len(keys))
	for _, k := range slices.Compact(slices.Sorted(slices.Values(keys))) {
		rows = append(rows, queueRow{Key: k, Backfill: true})
	}
	if len(rows) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{"enqueued_at": gorm.Expr("clock_timestamp()")}),
	}).CreateInBatches(rows, 500).Error
}

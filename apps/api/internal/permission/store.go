package permission

import (
	"context"
	"encoding/json"
	"time"

	"kun-galgame-patch-api/pkg/perm"

	"gorm.io/gorm"
)

type roleOverride struct {
	Role       string    `gorm:"column:role;primaryKey"`
	Permission string    `gorm:"column:permission;primaryKey"`
	Effect     string    `gorm:"column:effect"`
	UpdatedBy  int       `gorm:"column:updated_by"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (roleOverride) TableName() string { return "role_permission_override" }

type userOverride struct {
	UserID     int       `gorm:"column:user_id;primaryKey"`
	Permission string    `gorm:"column:permission;primaryKey"`
	Effect     string    `gorm:"column:effect"`
	UpdatedBy  int       `gorm:"column:updated_by"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (userOverride) TableName() string { return "user_permission_override" }

type change struct {
	ID           int             `gorm:"column:id;primaryKey;autoIncrement"`
	ActorID      int             `gorm:"column:actor_id"`
	TargetRole   *string         `gorm:"column:target_role"`
	TargetUserID *int            `gorm:"column:target_user_id"`
	Before       json.RawMessage `gorm:"column:before;type:jsonb"`
	After        json.RawMessage `gorm:"column:after;type:jsonb"`
	Reason       string          `gorm:"column:reason"`
	Created      time.Time       `gorm:"column:created;autoCreateTime"`
}

func (change) TableName() string { return "permission_change" }

type ChangeFilter struct {
	Role   string
	UserID int
}

type store struct {
	db *gorm.DB
}

func (s *store) roleOverrides(ctx context.Context, db *gorm.DB) (map[string][]perm.Override, error) {
	var rows []roleOverride
	if err := db.WithContext(ctx).Order("role, permission").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string][]perm.Override{}
	for _, r := range rows {
		out[r.Role] = append(out[r.Role], perm.Override{Permission: perm.Permission(r.Permission), Effect: r.Effect})
	}
	return out, nil
}

func (s *store) userOverrides(ctx context.Context, db *gorm.DB, uid int) (map[int][]perm.Override, error) {
	q := db.WithContext(ctx).Order("user_id, permission")
	if uid > 0 {
		q = q.Where("user_id = ?", uid)
	}
	var rows []userOverride
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[int][]perm.Override{}
	for _, r := range rows {
		out[r.UserID] = append(out[r.UserID], perm.Override{Permission: perm.Permission(r.Permission), Effect: r.Effect})
	}
	return out, nil
}

// The version of a target is the id of its newest change, 0 before the first.
// Role layers share one version: the moderator ⊆ admin rule ties them together.
func (s *store) roleVersion(ctx context.Context, db *gorm.DB) (int, error) {
	var v int
	err := db.WithContext(ctx).Model(&change{}).Where("target_role IS NOT NULL").
		Select("COALESCE(MAX(id), 0)").Scan(&v).Error
	return v, err
}

func (s *store) userVersion(ctx context.Context, db *gorm.DB, uid int) (int, error) {
	var v int
	err := db.WithContext(ctx).Model(&change{}).Where("target_user_id = ?", uid).
		Select("COALESCE(MAX(id), 0)").Scan(&v).Error
	return v, err
}

// lockedTx serialises every override write: the moderator ⊆ admin check reads
// rows of more than one role, so the read, the judgement and the write share
// one lock or two writers can break it between them.
func (s *store) lockedTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext('permission_override'))").Error; err != nil {
			return err
		}
		return fn(tx)
	})
}

func replaceRole(tx *gorm.DB, actor int, reason, role string, before, after []perm.Override) error {
	if err := tx.Where("role = ?", role).Delete(&roleOverride{}).Error; err != nil {
		return err
	}
	now := time.Now()
	for _, ov := range after {
		row := roleOverride{Role: role, Permission: string(ov.Permission), Effect: ov.Effect, UpdatedBy: actor, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return tx.Create(&change{ActorID: actor, TargetRole: &role, Before: deltas(before), After: deltas(after), Reason: reason}).Error
}

func replaceUser(tx *gorm.DB, actor int, reason string, uid int, before, after []perm.Override) error {
	if err := tx.Where("user_id = ?", uid).Delete(&userOverride{}).Error; err != nil {
		return err
	}
	now := time.Now()
	for _, ov := range after {
		row := userOverride{UserID: uid, Permission: string(ov.Permission), Effect: ov.Effect, UpdatedBy: actor, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return tx.Create(&change{ActorID: actor, TargetUserID: &uid, Before: deltas(before), After: deltas(after), Reason: reason}).Error
}

func (s *store) changes(ctx context.Context, f ChangeFilter, offset, limit int) ([]change, int64, error) {
	q := s.db.WithContext(ctx).Model(&change{})
	if f.Role != "" {
		q = q.Where("target_role = ?", f.Role)
	}
	if f.UserID > 0 {
		q = q.Where("target_user_id = ? OR actor_id = ?", f.UserID, f.UserID)
	}
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []change
	err := q.Order("created DESC, id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

func deltas(ovs []perm.Override) json.RawMessage {
	if ovs == nil {
		ovs = []perm.Override{}
	}
	b, _ := json.Marshal(ovs)
	return b
}

func decode(raw json.RawMessage) []perm.Override {
	var out []perm.Override
	_ = json.Unmarshal(raw, &out)
	return nonNil(out)
}

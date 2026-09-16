package service

import (
	"kun-galgame-patch-api/internal/setting/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	KeyCreatorOnly = "creator_only"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) GetBool(key string) bool {
	var row model.SiteSetting
	if err := s.db.Select("value").Where("key = ?", key).First(&row).Error; err != nil {
		return false
	}
	return row.Value == "true"
}

func (s *Service) SetBool(key string, enabled bool, updatedBy int) error {
	val := "false"
	if enabled {
		val = "true"
	}
	row := model.SiteSetting{Key: key, Value: val, UpdatedBy: &updatedBy}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"value":      val,
			"updated_by": updatedBy,
			"updated_at": gorm.Expr("now()"),
		}),
	}).Create(&row).Error
}

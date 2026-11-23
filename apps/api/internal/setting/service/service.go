package service

import (
	"errors"
	"sync"
	"time"

	"kun-galgame-patch-api/internal/setting/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	KeyCreatorOnly        = "creator_only"
	KeyDlsiteLinkTemplate = "dlsite_link_template"
	KeyDlsiteCouponURL    = "dlsite_coupon_url"
)

// stringTTL bounds how long another instance keeps serving a value after an
// admin changes it; the writing instance sees it at once.
const stringTTL = 30 * time.Second

type cachedString struct {
	value string
	at    time.Time
}

type Service struct {
	db *gorm.DB

	mu      sync.Mutex
	strings map[string]cachedString
}

func New(db *gorm.DB) *Service { return &Service{db: db, strings: map[string]cachedString{}} }

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
	return s.set(key, val, updatedBy)
}

// GetString is read on render paths (every game page asks for the DLsite
// template), so it is served from a short cache. A failed read keeps the last
// value rather than turning the feature off.
func (s *Service) GetString(key string) string {
	s.mu.Lock()
	c, ok := s.strings[key]
	s.mu.Unlock()
	if ok && time.Since(c.at) < stringTTL {
		return c.value
	}
	var row model.SiteSetting
	err := s.db.Select("value").Where("key = ?", key).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.value
	}
	s.remember(key, row.Value)
	return row.Value
}

func (s *Service) SetString(key, value string, updatedBy int) error {
	if err := s.set(key, value, updatedBy); err != nil {
		return err
	}
	s.remember(key, value)
	return nil
}

func (s *Service) remember(key, value string) {
	s.mu.Lock()
	s.strings[key] = cachedString{value: value, at: time.Now()}
	s.mu.Unlock()
}

func (s *Service) set(key, value string, updatedBy int) error {
	row := model.SiteSetting{Key: key, Value: value, UpdatedBy: &updatedBy}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"value":      value,
			"updated_by": updatedBy,
			"updated_at": gorm.Expr("now()"),
		}),
	}).Create(&row).Error
}

package repository

import (
	"kun-galgame-patch-api/internal/auth/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AuthRepository is the data layer for the auth module. After the OAuth
// migration this is intentionally tiny: identity is owned by OAuth, so the
// only operations are "lookup local row by id" and "insert empty local row".
type AuthRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// FindUserByID looks up the local user row by id. The id should equal
// OAuth.users.id (aligned by migrate-users) and is taken from
// /oauth/userinfo's `id` field at login time.
func (r *AuthRepository) FindUserByID(id int) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser inserts a new local row. Caller must populate ID with the
// OAuth-side integer id (NOT autoincrement; see migration 005). ON CONFLICT DO
// NOTHING makes the concurrent first-login of a brand-new user idempotent
// (audit F066): the race loser no longer hits a PK unique-violation that the
// handler would surface as a bare 500. Callers should re-fetch the canonical
// row afterward.
func (r *AuthRepository) CreateUser(user *model.User) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(user).Error
}

// UpdateLastLoginTime stamps last_login_time on the local row.
func (r *AuthRepository) UpdateLastLoginTime(userID int, t string) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("last_login_time", t).Error
}

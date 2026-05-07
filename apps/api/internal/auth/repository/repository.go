package repository

import (
	"kun-galgame-patch-api/internal/auth/model"

	"gorm.io/gorm"
)

type AuthRepository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) FindUserByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("LOWER(email) = LOWER(?)", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindUserByNameOrEmail(name string) (*model.User, error) {
	var user model.User
	err := r.db.Where("LOWER(name) = LOWER(?) OR LOWER(email) = LOWER(?)", name, name).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindUserByID(id int) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) CreateUser(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *AuthRepository) UpdateUser(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *AuthRepository) UpdateUserPassword(userID int, hashedPassword string) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("password", hashedPassword).Error
}

func (r *AuthRepository) UpdateLastLoginTime(userID int, t string) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("last_login_time", t).Error
}

func (r *AuthRepository) FindOAuthAccountBySub(sub string) (*model.OAuthAccount, error) {
	var account model.OAuthAccount
	err := r.db.Where("sub = ?", sub).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *AuthRepository) CreateOAuthAccount(account *model.OAuthAccount) error {
	return r.db.Create(account).Error
}

// FindUserByName looks up a user by exact (case-insensitive) name. Used by the
// OAuth provisioning path to detect display-name collisions before insert.
func (r *AuthRepository) FindUserByName(name string) (*model.User, error) {
	var user model.User
	err := r.db.Where("LOWER(name) = LOWER(?)", name).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

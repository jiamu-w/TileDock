package repository

import (
	"context"

	"panel/internal/model"

	"gorm.io/gorm"
)

// UserRepository handles user persistence.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a repository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByUsername returns user by username.
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID returns user by id.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateLoginTime stores the last login time.
func (r *UserRepository) UpdateLoginTime(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("last_login_at", gorm.Expr("CURRENT_TIMESTAMP")).Error
}

// UpdateUsername updates a user's username.
func (r *UserRepository) UpdateUsername(ctx context.Context, id, username string) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Update("username", username).
		Error
}

// UpdatePasswordHash updates a user's password hash.
func (r *UserRepository) UpdatePasswordHash(ctx context.Context, id, passwordHash string) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", id).
		Update("password_hash", passwordHash).
		Error
}

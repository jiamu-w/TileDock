package repository

import (
	"context"

	"panel/internal/model"

	"gorm.io/gorm"
)

// SettingRepository handles settings persistence.
type SettingRepository struct {
	db *gorm.DB
}

// NewSettingRepository creates a repository.
func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

// List returns all settings.
func (r *SettingRepository) List(ctx context.Context) ([]model.Setting, error) {
	var settings []model.Setting
	err := r.db.WithContext(ctx).Order("key asc").Find(&settings).Error
	return settings, err
}

// FindByKey returns a setting by key.
func (r *SettingRepository) FindByKey(ctx context.Context, key string) (*model.Setting, error) {
	var setting model.Setting
	tx := r.db.WithContext(ctx).Where("key = ?", key).Limit(1).Find(&setting)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, nil
	}
	return &setting, nil
}

// Upsert inserts or updates a setting.
func (r *SettingRepository) Upsert(ctx context.Context, setting *model.Setting) error {
	return r.db.WithContext(ctx).Where(model.Setting{Key: setting.Key}).
		Assign(model.Setting{Value: setting.Value}).
		FirstOrCreate(setting).Error
}

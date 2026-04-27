package repository

import (
	"context"

	"panel/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FaviconCacheRepository handles per-domain favicon cache metadata.
type FaviconCacheRepository struct {
	db *gorm.DB
}

// NewFaviconCacheRepository creates a favicon cache repository.
func NewFaviconCacheRepository(db *gorm.DB) *FaviconCacheRepository {
	return &FaviconCacheRepository{db: db}
}

// FindByDomain returns a cached favicon domain record.
func (r *FaviconCacheRepository) FindByDomain(ctx context.Context, domain string) (*model.FaviconCache, error) {
	var cache model.FaviconCache
	if err := r.db.WithContext(ctx).First(&cache, "domain = ?", domain).Error; err != nil {
		return nil, err
	}
	return &cache, nil
}

// Upsert writes cache metadata for a domain.
func (r *FaviconCacheRepository) Upsert(ctx context.Context, cache *model.FaviconCache) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "domain"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"icon_cached_path",
			"theme_accent_color",
			"theme_bg_start_color",
			"theme_bg_end_color",
			"theme_border_color",
			"theme_text_color",
			"icon_status",
			"icon_last_checked_at",
			"icon_next_check_at",
			"icon_fail_count",
			"updated_at",
		}),
	}).Create(cache).Error
}

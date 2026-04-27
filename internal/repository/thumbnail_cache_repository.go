package repository

import (
	"context"

	"panel/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ThumbnailCacheRepository handles per-domain website thumbnail cache metadata.
type ThumbnailCacheRepository struct {
	db *gorm.DB
}

// NewThumbnailCacheRepository creates a thumbnail cache repository.
func NewThumbnailCacheRepository(db *gorm.DB) *ThumbnailCacheRepository {
	return &ThumbnailCacheRepository{db: db}
}

// FindByDomain returns a cached thumbnail domain record.
func (r *ThumbnailCacheRepository) FindByDomain(ctx context.Context, domain string) (*model.ThumbnailCache, error) {
	var cache model.ThumbnailCache
	if err := r.db.WithContext(ctx).First(&cache, "domain = ?", domain).Error; err != nil {
		return nil, err
	}
	return &cache, nil
}

// Upsert writes cache metadata for a domain.
func (r *ThumbnailCacheRepository) Upsert(ctx context.Context, cache *model.ThumbnailCache) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "domain"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"thumbnail_cached_path",
			"thumbnail_status",
			"thumbnail_last_checked_at",
			"thumbnail_next_check_at",
			"thumbnail_fail_count",
			"updated_at",
		}),
	}).Create(cache).Error
}

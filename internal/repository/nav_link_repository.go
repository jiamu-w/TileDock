package repository

import (
	"context"
	"time"

	"panel/internal/model"

	"gorm.io/gorm"
)

// NavLinkRepository handles link persistence.
type NavLinkRepository struct {
	db *gorm.DB
}

// LinkOrderUpdate stores link reorder data.
type LinkOrderUpdate struct {
	ID        string
	GroupID   string
	SortOrder int
}

// NewNavLinkRepository creates a repository.
func NewNavLinkRepository(db *gorm.DB) *NavLinkRepository {
	return &NavLinkRepository{db: db}
}

// Create inserts a link.
func (r *NavLinkRepository) Create(ctx context.Context, link *model.NavLink) error {
	return r.db.WithContext(ctx).Create(link).Error
}

// Update updates a link.
func (r *NavLinkRepository) Update(ctx context.Context, link *model.NavLink) error {
	return r.db.WithContext(ctx).Model(link).
		Select("group_id", "title", "url", "description", "icon", "icon_cached_path", "theme_accent_color", "theme_bg_start_color", "theme_bg_end_color", "theme_border_color", "theme_text_color", "icon_status", "icon_last_checked_at", "icon_next_check_at", "icon_fail_count", "thumbnail_cached_path", "thumbnail_status", "thumbnail_last_checked_at", "thumbnail_next_check_at", "thumbnail_fail_count", "open_in_new", "sort_order").
		Updates(link).Error
}

// Delete deletes a link.
func (r *NavLinkRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.NavLink{}, "id = ?", id).Error
}

// FindByID returns a link by id.
func (r *NavLinkRepository) FindByID(ctx context.Context, id string) (*model.NavLink, error) {
	var link model.NavLink
	if err := r.db.WithContext(ctx).First(&link, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

// FindDueIconLinks returns links that should have their favicon checked.
func (r *NavLinkRepository) FindDueIconLinks(ctx context.Context, now time.Time, limit int) ([]model.NavLink, error) {
	if limit <= 0 {
		limit = 20
	}
	var links []model.NavLink
	err := r.db.WithContext(ctx).
		Where("icon_next_check_at IS NOT NULL AND icon_next_check_at <= ?", now).
		Order("icon_next_check_at asc, updated_at asc").
		Limit(limit).
		Find(&links).Error
	return links, err
}

// UpdateIconState updates favicon cache metadata for a single link.
func (r *NavLinkRepository) UpdateIconState(ctx context.Context, id string, values map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.NavLink{}).Where("id = ?", id).Updates(values).Error
}

// ScheduleMissingIcons marks links without a cached favicon for immediate background scanning.
func (r *NavLinkRepository) ScheduleMissingIcons(ctx context.Context, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.NavLink{}).
		Where("COALESCE(icon_cached_path, '') = ''").
		Updates(map[string]any{
			"icon_status":        "pending",
			"icon_next_check_at": now,
		})
	return result.RowsAffected, result.Error
}

// FindDueThumbnailLinks returns links that should have their website thumbnail checked.
func (r *NavLinkRepository) FindDueThumbnailLinks(ctx context.Context, now time.Time, limit int) ([]model.NavLink, error) {
	if limit <= 0 {
		limit = 5
	}
	var links []model.NavLink
	err := r.db.WithContext(ctx).
		Where("thumbnail_next_check_at IS NOT NULL AND thumbnail_next_check_at <= ?", now).
		Order("thumbnail_next_check_at asc, updated_at asc").
		Limit(limit).
		Find(&links).Error
	return links, err
}

// UpdateThumbnailState updates website thumbnail cache metadata for a single link.
func (r *NavLinkRepository) UpdateThumbnailState(ctx context.Context, id string, values map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.NavLink{}).Where("id = ?", id).Updates(values).Error
}

// ScheduleMissingThumbnails marks links without a cached thumbnail for immediate background scanning.
func (r *NavLinkRepository) ScheduleMissingThumbnails(ctx context.Context, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.NavLink{}).
		Where("COALESCE(thumbnail_cached_path, '') = ''").
		Updates(map[string]any{
			"thumbnail_status":        "pending",
			"thumbnail_next_check_at": now,
		})
	return result.RowsAffected, result.Error
}

// ReuseDomainIcon updates links on the same domain with a cached favicon.
func (r *NavLinkRepository) ReuseDomainIcon(ctx context.Context, domain, iconPath string, now, next time.Time) error {
	return r.db.WithContext(ctx).Model(&model.NavLink{}).
		Where("icon_cached_path = '' AND icon_status <> ? AND url LIKE ?", "success", "%"+domain+"%").
		Updates(map[string]any{
			"icon_cached_path":     iconPath,
			"icon_status":          "success",
			"icon_last_checked_at": now,
			"icon_next_check_at":   next,
			"icon_fail_count":      0,
		}).Error
}

// NextSortOrder returns the next link sort order inside a group.
func (r *NavLinkRepository) NextSortOrder(ctx context.Context, groupID string) (int, error) {
	var link model.NavLink
	err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("sort_order desc").
		Limit(1).
		Find(&link).Error
	if err != nil {
		return 0, err
	}
	return link.SortOrder + 10, nil
}

// UpdateOrders updates link group and sort order in a transaction.
func (r *NavLinkRepository) UpdateOrders(ctx context.Context, updates []LinkOrderUpdate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, update := range updates {
			if err := tx.Model(&model.NavLink{}).
				Where("id = ?", update.ID).
				Updates(map[string]any{
					"group_id":   update.GroupID,
					"sort_order": update.SortOrder,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

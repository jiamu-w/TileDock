package repository

import (
	"context"

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
		Select("group_id", "title", "url", "description", "icon", "open_in_new", "sort_order").
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

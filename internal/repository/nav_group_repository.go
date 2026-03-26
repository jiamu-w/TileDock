package repository

import (
	"context"

	"panel/internal/model"

	"gorm.io/gorm"
)

// NavGroupRepository handles group persistence.
type NavGroupRepository struct {
	db *gorm.DB
}

// NewNavGroupRepository creates a repository.
func NewNavGroupRepository(db *gorm.DB) *NavGroupRepository {
	return &NavGroupRepository{db: db}
}

// List returns all groups with links.
func (r *NavGroupRepository) List(ctx context.Context) ([]model.NavGroup, error) {
	var groups []model.NavGroup
	err := r.db.WithContext(ctx).
		Preload("NavLinks", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("sort_order asc, created_at asc")
		}).
		Order("sort_order asc, created_at asc").
		Find(&groups).Error
	return groups, err
}

// Create inserts a group.
func (r *NavGroupRepository) Create(ctx context.Context, group *model.NavGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

// Update updates a group.
func (r *NavGroupRepository) Update(ctx context.Context, group *model.NavGroup) error {
	return r.db.WithContext(ctx).Model(group).Select("name", "sort_order", "grid_cols", "grid_rows").Updates(group).Error
}

// Delete deletes a group.
func (r *NavGroupRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.NavGroup{}, "id = ?", id).Error
}

// FindByID returns a group by id.
func (r *NavGroupRepository) FindByID(ctx context.Context, id string) (*model.NavGroup, error) {
	var group model.NavGroup
	if err := r.db.WithContext(ctx).First(&group, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// NextSortOrder returns the next group sort order.
func (r *NavGroupRepository) NextSortOrder(ctx context.Context) (int, error) {
	var group model.NavGroup
	err := r.db.WithContext(ctx).
		Order("sort_order desc").
		Limit(1).
		Find(&group).Error
	if err != nil {
		return 0, err
	}
	return group.SortOrder + 10, nil
}

// UpdateSortOrders updates group sort order in a transaction.
func (r *NavGroupRepository) UpdateSortOrders(ctx context.Context, ids []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index, id := range ids {
			if err := tx.Model(&model.NavGroup{}).
				Where("id = ?", id).
				Update("sort_order", index*10).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateGridSpan updates a group's saved grid size.
func (r *NavGroupRepository) UpdateGridSpan(ctx context.Context, id string, cols, rows int) error {
	return r.db.WithContext(ctx).
		Model(&model.NavGroup{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"grid_cols": cols,
			"grid_rows": rows,
		}).Error
}

package service

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"panel/internal/model"
	"panel/internal/repository"
)

// NavigationService handles navigation CRUD.
type NavigationService struct {
	groupRepo *repository.NavGroupRepository
	linkRepo  *repository.NavLinkRepository
}

// ReorderRequest stores drag-sort payload.
type ReorderRequest struct {
	GroupIDs []string      `json:"group_ids"`
	Links    []ReorderLink `json:"links"`
}

// ReorderLink stores single link order payload.
type ReorderLink struct {
	ID      string `json:"id"`
	GroupID string `json:"group_id"`
}

// GroupGridSize stores persisted group span values.
type GroupGridSize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// NewNavigationService creates a service.
func NewNavigationService(groupRepo *repository.NavGroupRepository, linkRepo *repository.NavLinkRepository) *NavigationService {
	return &NavigationService{groupRepo: groupRepo, linkRepo: linkRepo}
}

// NavigationPageData stores navigation management data.
type NavigationPageData struct {
	Groups []model.NavGroup
}

// List returns groups and links.
func (s *NavigationService) List(ctx context.Context) (*NavigationPageData, error) {
	groups, err := s.groupRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	return &NavigationPageData{Groups: groups}, nil
}

// CreateGroup creates a new group.
func (s *NavigationService) CreateGroup(ctx context.Context, name string) error {
	_, err := s.CreateGroupWithID(ctx, name)
	return err
}

// CreateGroupWithID creates a new group and returns its ID.
func (s *NavigationService) CreateGroupWithID(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("group name is required")
	}

	sortOrder, err := s.groupRepo.NextSortOrder(ctx)
	if err != nil {
		return "", err
	}

	group := &model.NavGroup{
		Name:      name,
		SortOrder: sortOrder,
	}
	if err := s.groupRepo.Create(ctx, group); err != nil {
		return "", err
	}
	return group.ID, nil
}

// UpdateGroup updates a group.
func (s *NavigationService) UpdateGroup(ctx context.Context, id, name string) error {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("group name is required")
	}

	group.Name = name

	return s.groupRepo.Update(ctx, group)
}

// DeleteGroup deletes a group.
func (s *NavigationService) DeleteGroup(ctx context.Context, id string) error {
	return s.groupRepo.Delete(ctx, id)
}

// ResizeGroup updates a group's saved grid size.
func (s *NavigationService) ResizeGroup(ctx context.Context, id string, size GroupGridSize) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("group id is required")
	}

	if size.Cols < 8 {
		size.Cols = 8
	}
	if size.Cols > 36 {
		size.Cols = 36
	}

	if size.Rows < 8 {
		size.Rows = 8
	}
	if size.Rows > 28 {
		size.Rows = 28
	}

	return s.groupRepo.UpdateGridSpan(ctx, id, size.Cols, size.Rows)
}

// CreateLink creates a new link.
func (s *NavigationService) CreateLink(ctx context.Context, input LinkInput) error {
	if err := input.Validate(); err != nil {
		return err
	}

	link := &model.NavLink{
		GroupID:     strings.TrimSpace(input.GroupID),
		Title:       strings.TrimSpace(input.Title),
		URL:         strings.TrimSpace(input.URL),
		Description: strings.TrimSpace(input.Description),
		Icon:        strings.TrimSpace(input.Icon),
		OpenInNew:   input.OpenInNew,
		SortOrder:   0,
	}
	sortOrder, err := s.linkRepo.NextSortOrder(ctx, link.GroupID)
	if err != nil {
		return err
	}
	link.SortOrder = sortOrder
	return s.linkRepo.Create(ctx, link)
}

// UpdateLink updates a link.
func (s *NavigationService) UpdateLink(ctx context.Context, id string, input LinkInput) error {
	if err := input.Validate(); err != nil {
		return err
	}

	link, err := s.linkRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	link.GroupID = strings.TrimSpace(input.GroupID)
	link.Title = strings.TrimSpace(input.Title)
	link.URL = strings.TrimSpace(input.URL)
	link.Description = strings.TrimSpace(input.Description)
	link.Icon = strings.TrimSpace(input.Icon)
	link.OpenInNew = input.OpenInNew
	return s.linkRepo.Update(ctx, link)
}

// DeleteLink deletes a link.
func (s *NavigationService) DeleteLink(ctx context.Context, id string) error {
	return s.linkRepo.Delete(ctx, id)
}

// Reorder persists group and link sort order.
func (s *NavigationService) Reorder(ctx context.Context, req ReorderRequest) error {
	cleanGroupIDs := make([]string, 0, len(req.GroupIDs))
	for _, id := range req.GroupIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			cleanGroupIDs = append(cleanGroupIDs, id)
		}
	}

	if len(cleanGroupIDs) > 0 {
		if err := s.groupRepo.UpdateSortOrders(ctx, cleanGroupIDs); err != nil {
			return err
		}
	}

	orderByGroup := make(map[string]int)
	updates := make([]repository.LinkOrderUpdate, 0, len(req.Links))
	for _, item := range req.Links {
		item.ID = strings.TrimSpace(item.ID)
		item.GroupID = strings.TrimSpace(item.GroupID)
		if item.ID == "" || item.GroupID == "" {
			continue
		}

		updates = append(updates, repository.LinkOrderUpdate{
			ID:        item.ID,
			GroupID:   item.GroupID,
			SortOrder: orderByGroup[item.GroupID] * 10,
		})
		orderByGroup[item.GroupID]++
	}

	if len(updates) > 0 {
		if err := s.linkRepo.UpdateOrders(ctx, updates); err != nil {
			return err
		}
	}

	return nil
}

// LinkInput stores link form input.
type LinkInput struct {
	GroupID     string
	Title       string
	URL         string
	Description string
	Icon        string
	OpenInNew   bool
}

// Validate checks link input.
func (i LinkInput) Validate() error {
	i.GroupID = strings.TrimSpace(i.GroupID)
	i.Title = strings.TrimSpace(i.Title)
	i.URL = strings.TrimSpace(i.URL)

	if i.GroupID == "" {
		return errors.New("group is required")
	}
	if i.Title == "" {
		return errors.New("title is required")
	}
	if i.URL == "" {
		return errors.New("url is required")
	}
	if _, err := url.ParseRequestURI(i.URL); err != nil {
		return errors.New("url is invalid")
	}
	return nil
}

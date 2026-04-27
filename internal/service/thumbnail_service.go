package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"panel/internal/model"
	"panel/internal/repository"

	"gorm.io/gorm"
)

const (
	thumbnailBatchLimit = 5
	thumbnailTick       = 2 * time.Minute
)

// ThumbnailService schedules and processes website thumbnail cache refreshes.
type ThumbnailService struct {
	linkRepo    *repository.NavLinkRepository
	cacheRepo   *repository.ThumbnailCacheRepository
	settingRepo *repository.SettingRepository
	uploadDir   string
	log         *slog.Logger
	wake        chan struct{}
	mu          sync.Mutex
	running     map[string]struct{}
}

// NewThumbnailService creates a thumbnail background service.
func NewThumbnailService(linkRepo *repository.NavLinkRepository, cacheRepo *repository.ThumbnailCacheRepository, settingRepo *repository.SettingRepository, uploadDir string, log *slog.Logger) *ThumbnailService {
	return &ThumbnailService{
		linkRepo:    linkRepo,
		cacheRepo:   cacheRepo,
		settingRepo: settingRepo,
		uploadDir:   uploadDir,
		log:         log,
		wake:        make(chan struct{}, 1),
		running:     make(map[string]struct{}),
	}
}

// Start runs the periodic thumbnail processor until ctx is canceled.
func (s *ThumbnailService) Start(ctx context.Context) {
	ticker := time.NewTicker(thumbnailTick)
	defer ticker.Stop()

	s.processDue(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
			s.processDue(ctx)
		case <-ticker.C:
			s.processDue(ctx)
		}
	}
}

// Enabled reports whether website thumbnail backgrounds are enabled.
func (s *ThumbnailService) Enabled(ctx context.Context) bool {
	setting, err := s.settingRepo.FindByKey(ctx, dashboardThumbnailBgKey)
	if err != nil || setting == nil {
		return false
	}
	return settingBool(setting.Value)
}

// EnqueueLink schedules a link for thumbnail fetching if the feature is enabled.
func (s *ThumbnailService) EnqueueLink(ctx context.Context, linkID string) error {
	if !s.Enabled(ctx) {
		return nil
	}
	return s.enqueueLinkNow(ctx, linkID, false)
}

// RefreshLink immediately schedules a link thumbnail and clears previous failure count.
func (s *ThumbnailService) RefreshLink(ctx context.Context, linkID string) error {
	return s.enqueueLinkNow(ctx, linkID, true)
}

// RescanMissing schedules every link without a cached thumbnail for background scanning.
func (s *ThumbnailService) RescanMissing(ctx context.Context) (int64, error) {
	count, err := s.linkRepo.ScheduleMissingThumbnails(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	if count > 0 {
		s.Notify()
	}
	return count, nil
}

// Notify wakes the background worker.
func (s *ThumbnailService) Notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *ThumbnailService) enqueueLinkNow(ctx context.Context, linkID string, clearFailure bool) error {
	linkID = strings.TrimSpace(linkID)
	if linkID == "" {
		return nil
	}

	values := map[string]any{
		"thumbnail_status":        IconStatusPending,
		"thumbnail_next_check_at": time.Now(),
	}
	if clearFailure {
		values["thumbnail_fail_count"] = 0
	}
	if err := s.linkRepo.UpdateThumbnailState(ctx, linkID, values); err != nil {
		return err
	}
	s.Notify()
	return nil
}

func (s *ThumbnailService) processDue(ctx context.Context) {
	if !s.Enabled(ctx) {
		return
	}

	links, err := s.linkRepo.FindDueThumbnailLinks(ctx, time.Now(), thumbnailBatchLimit)
	if err != nil {
		s.log.Warn("list due thumbnails failed", "error", err)
		return
	}

	for _, link := range links {
		if ctx.Err() != nil {
			return
		}
		if !s.markRunning(link.ID) {
			continue
		}
		s.processLink(ctx, link)
		s.unmarkRunning(link.ID)
	}
}

func (s *ThumbnailService) markRunning(linkID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[linkID]; ok {
		return false
	}
	s.running[linkID] = struct{}{}
	return true
}

func (s *ThumbnailService) unmarkRunning(linkID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, linkID)
}

func (s *ThumbnailService) processLink(ctx context.Context, link model.NavLink) {
	domain := NormalizeIconDomain(link.URL)
	if domain == "" {
		s.markLinkFailure(ctx, link, domain)
		return
	}

	now := time.Now()
	successNext := now.Add(30 * 24 * time.Hour)
	if cache, err := s.cacheRepo.FindByDomain(ctx, domain); err == nil {
		if cache.ThumbnailStatus == IconStatusSuccess && strings.TrimSpace(cache.ThumbnailCachedPath) != "" {
			_ = s.linkRepo.UpdateThumbnailState(ctx, link.ID, map[string]any{
				"thumbnail_cached_path":     cache.ThumbnailCachedPath,
				"thumbnail_status":          IconStatusSuccess,
				"thumbnail_last_checked_at": now,
				"thumbnail_next_check_at":   successNext,
				"thumbnail_fail_count":      0,
			})
			return
		}
		if cache.ThumbnailNextCheckAt != nil && cache.ThumbnailNextCheckAt.After(now) {
			_ = s.linkRepo.UpdateThumbnailState(ctx, link.ID, map[string]any{
				"thumbnail_status":          cache.ThumbnailStatus,
				"thumbnail_last_checked_at": cache.ThumbnailLastCheckedAt,
				"thumbnail_next_check_at":   cache.ThumbnailNextCheckAt,
				"thumbnail_fail_count":      cache.ThumbnailFailCount,
			})
			return
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.log.Warn("load thumbnail cache failed", "error", err, "domain", domain)
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	thumbnailPath, err := FetchWebsiteThumbnail(fetchCtx, link.URL, s.uploadDir)
	if err != nil || strings.TrimSpace(thumbnailPath) == "" {
		s.markLinkFailure(ctx, link, domain)
		return
	}

	cache := &model.ThumbnailCache{
		Domain:                 domain,
		ThumbnailCachedPath:    thumbnailPath,
		ThumbnailStatus:        IconStatusSuccess,
		ThumbnailLastCheckedAt: &now,
		ThumbnailNextCheckAt:   &successNext,
		ThumbnailFailCount:     0,
	}
	if err := s.cacheRepo.Upsert(ctx, cache); err != nil {
		s.log.Warn("save thumbnail cache failed", "error", err, "domain", domain)
	}

	if err := s.linkRepo.UpdateThumbnailState(ctx, link.ID, map[string]any{
		"thumbnail_cached_path":     thumbnailPath,
		"thumbnail_status":          IconStatusSuccess,
		"thumbnail_last_checked_at": now,
		"thumbnail_next_check_at":   successNext,
		"thumbnail_fail_count":      0,
	}); err != nil {
		s.log.Warn("update thumbnail link state failed", "error", err, "link_id", link.ID)
	}
}

func (s *ThumbnailService) markLinkFailure(ctx context.Context, link model.NavLink, domain string) {
	now := time.Now()
	failCount := link.ThumbnailFailCount + 1
	next := now.Add(faviconRetryDelay(failCount))

	if domain != "" {
		cache := &model.ThumbnailCache{
			Domain:                 domain,
			ThumbnailStatus:        IconStatusFailed,
			ThumbnailLastCheckedAt: &now,
			ThumbnailNextCheckAt:   &next,
			ThumbnailFailCount:     failCount,
		}
		if err := s.cacheRepo.Upsert(ctx, cache); err != nil {
			s.log.Warn("save failed thumbnail cache state failed", "error", err, "domain", domain)
		}
	}

	if err := s.linkRepo.UpdateThumbnailState(ctx, link.ID, map[string]any{
		"thumbnail_status":          IconStatusFailed,
		"thumbnail_last_checked_at": now,
		"thumbnail_next_check_at":   next,
		"thumbnail_fail_count":      failCount,
	}); err != nil {
		s.log.Warn("update failed thumbnail link state failed", "error", err, "link_id", link.ID)
	}
}

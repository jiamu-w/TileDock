package service

import (
	"context"
	"errors"
	"log/slog"
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"panel/internal/model"
	"panel/internal/repository"

	"gorm.io/gorm"
)

const (
	IconStatusPending = "pending"
	IconStatusSuccess = "success"
	IconStatusFailed  = "failed"

	faviconBatchLimit = 20
	faviconTick       = time.Minute
)

// FaviconService schedules and processes favicon cache refreshes.
type FaviconService struct {
	linkRepo  *repository.NavLinkRepository
	cacheRepo *repository.FaviconCacheRepository
	uploadDir string
	log       *slog.Logger
	wake      chan struct{}
	mu        sync.Mutex
	running   map[string]struct{}
}

// NewFaviconService creates a favicon background service.
func NewFaviconService(linkRepo *repository.NavLinkRepository, cacheRepo *repository.FaviconCacheRepository, uploadDir string, log *slog.Logger) *FaviconService {
	return &FaviconService{
		linkRepo:  linkRepo,
		cacheRepo: cacheRepo,
		uploadDir: uploadDir,
		log:       log,
		wake:      make(chan struct{}, 1),
		running:   make(map[string]struct{}),
	}
}

// Start runs the periodic favicon processor until ctx is canceled.
func (s *FaviconService) Start(ctx context.Context) {
	ticker := time.NewTicker(faviconTick)
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

// EnqueueLink schedules a link for favicon fetching.
func (s *FaviconService) EnqueueLink(ctx context.Context, linkID string) error {
	linkID = strings.TrimSpace(linkID)
	if linkID == "" {
		return nil
	}
	now := time.Now()
	if err := s.linkRepo.UpdateIconState(ctx, linkID, map[string]any{
		"icon_status":        IconStatusPending,
		"icon_next_check_at": now,
	}); err != nil {
		return err
	}
	s.Notify()
	return nil
}

// RefreshLink immediately schedules a link and clears previous failure count.
func (s *FaviconService) RefreshLink(ctx context.Context, linkID string) error {
	now := time.Now()
	link, err := s.linkRepo.FindByID(ctx, linkID)
	if err != nil {
		return err
	}
	if domain := NormalizeIconDomain(link.URL); domain != "" {
		if err := s.cacheRepo.Upsert(ctx, &model.FaviconCache{
			Domain:          domain,
			IconStatus:      IconStatusPending,
			IconNextCheckAt: &now,
			IconFailCount:   0,
		}); err != nil {
			return err
		}
	}
	if err := s.linkRepo.UpdateIconState(ctx, linkID, map[string]any{
		"icon_status":        IconStatusPending,
		"icon_next_check_at": now,
		"icon_fail_count":    0,
	}); err != nil {
		return err
	}
	s.Notify()
	return nil
}

// RescanMissing schedules every link without a cached favicon for background scanning.
func (s *FaviconService) RescanMissing(ctx context.Context) (int64, error) {
	count, err := s.linkRepo.ScheduleMissingIcons(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	if count > 0 {
		s.Notify()
	}
	return count, nil
}

// Notify wakes the background worker.
func (s *FaviconService) Notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *FaviconService) processDue(ctx context.Context) {
	links, err := s.linkRepo.FindDueIconLinks(ctx, time.Now(), faviconBatchLimit)
	if err != nil {
		s.log.Warn("list due favicons failed", "error", err)
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

func (s *FaviconService) markRunning(linkID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[linkID]; ok {
		return false
	}
	s.running[linkID] = struct{}{}
	return true
}

func (s *FaviconService) unmarkRunning(linkID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, linkID)
}

func (s *FaviconService) processLink(ctx context.Context, link model.NavLink) {
	domain := NormalizeIconDomain(link.URL)
	if domain == "" {
		s.markLinkFailure(ctx, link, domain)
		return
	}

	now := time.Now()
	successNext := now.Add(30 * 24 * time.Hour)
	if cache, err := s.cacheRepo.FindByDomain(ctx, domain); err == nil {
		if cache.IconStatus == IconStatusSuccess && strings.TrimSpace(cache.IconCachedPath) != "" {
			theme := LinkTheme{
				AccentColor:  cache.ThemeAccentColor,
				BgStartColor: cache.ThemeBgStartColor,
				BgEndColor:   cache.ThemeBgEndColor,
				BorderColor:  cache.ThemeBorderColor,
				TextColor:    cache.ThemeTextColor,
			}
			if theme.IsZero() {
				theme = BuildLinkTheme(s.uploadDir, cache.IconCachedPath, link.URL, link.Title)
				cache.ThemeAccentColor = theme.AccentColor
				cache.ThemeBgStartColor = theme.BgStartColor
				cache.ThemeBgEndColor = theme.BgEndColor
				cache.ThemeBorderColor = theme.BorderColor
				cache.ThemeTextColor = theme.TextColor
				if err := s.cacheRepo.Upsert(ctx, cache); err != nil {
					s.log.Warn("backfill favicon theme failed", "error", err, "domain", domain)
				}
			}
			_ = s.linkRepo.UpdateIconState(ctx, link.ID, map[string]any{
				"icon_cached_path":     cache.IconCachedPath,
				"theme_accent_color":   theme.AccentColor,
				"theme_bg_start_color": theme.BgStartColor,
				"theme_bg_end_color":   theme.BgEndColor,
				"theme_border_color":   theme.BorderColor,
				"theme_text_color":     theme.TextColor,
				"icon_status":          IconStatusSuccess,
				"icon_last_checked_at": now,
				"icon_next_check_at":   successNext,
				"icon_fail_count":      0,
			})
			return
		}
		if cache.IconNextCheckAt != nil && cache.IconNextCheckAt.After(now) {
			_ = s.linkRepo.UpdateIconState(ctx, link.ID, map[string]any{
				"icon_status":          cache.IconStatus,
				"icon_last_checked_at": cache.IconLastCheckedAt,
				"icon_next_check_at":   cache.IconNextCheckAt,
				"icon_fail_count":      cache.IconFailCount,
			})
			return
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.log.Warn("load favicon cache failed", "error", err, "domain", domain)
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	iconPath, err := FetchWebsiteIcon(fetchCtx, link.URL, s.uploadDir)
	if err != nil || strings.TrimSpace(iconPath) == "" {
		s.markLinkFailure(ctx, link, domain)
		return
	}
	theme := BuildLinkTheme(s.uploadDir, iconPath, link.URL, link.Title)

	cache := &model.FaviconCache{
		Domain:            domain,
		IconCachedPath:    iconPath,
		ThemeAccentColor:  theme.AccentColor,
		ThemeBgStartColor: theme.BgStartColor,
		ThemeBgEndColor:   theme.BgEndColor,
		ThemeBorderColor:  theme.BorderColor,
		ThemeTextColor:    theme.TextColor,
		IconStatus:        IconStatusSuccess,
		IconLastCheckedAt: &now,
		IconNextCheckAt:   &successNext,
		IconFailCount:     0,
	}
	if err := s.cacheRepo.Upsert(ctx, cache); err != nil {
		s.log.Warn("save favicon cache failed", "error", err, "domain", domain)
	}

	if err := s.linkRepo.UpdateIconState(ctx, link.ID, map[string]any{
		"icon_cached_path":     iconPath,
		"theme_accent_color":   theme.AccentColor,
		"theme_bg_start_color": theme.BgStartColor,
		"theme_bg_end_color":   theme.BgEndColor,
		"theme_border_color":   theme.BorderColor,
		"theme_text_color":     theme.TextColor,
		"icon_status":          IconStatusSuccess,
		"icon_last_checked_at": now,
		"icon_next_check_at":   successNext,
		"icon_fail_count":      0,
	}); err != nil {
		s.log.Warn("update favicon link state failed", "error", err, "link_id", link.ID)
	}
}

func (s *FaviconService) markLinkFailure(ctx context.Context, link model.NavLink, domain string) {
	now := time.Now()
	failCount := link.IconFailCount + 1
	next := now.Add(faviconRetryDelay(failCount))

	if domain != "" {
		cache := &model.FaviconCache{
			Domain:            domain,
			IconStatus:        IconStatusFailed,
			IconLastCheckedAt: &now,
			IconNextCheckAt:   &next,
			IconFailCount:     failCount,
		}
		if err := s.cacheRepo.Upsert(ctx, cache); err != nil {
			s.log.Warn("save failed favicon cache state failed", "error", err, "domain", domain)
		}
	}

	if err := s.linkRepo.UpdateIconState(ctx, link.ID, map[string]any{
		"icon_status":          IconStatusFailed,
		"icon_last_checked_at": now,
		"icon_next_check_at":   next,
		"icon_fail_count":      failCount,
	}); err != nil {
		s.log.Warn("update failed favicon link state failed", "error", err, "link_id", link.ID)
	}
}

func faviconRetryDelay(failCount int) time.Duration {
	switch failCount {
	case 1:
		return time.Hour
	case 2:
		return 6 * time.Hour
	case 3:
		return 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

// NormalizeIconDomain returns a stable cache key for a URL host.
func NormalizeIconDomain(rawURL string) string {
	parsed, err := neturl.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.TrimPrefix(host, "www.")
}

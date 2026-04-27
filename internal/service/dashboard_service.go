package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"panel/internal/model"
	"panel/internal/repository"
	"panel/pkg/runtimepath"
)

// DashboardStats stores summary metrics.
type DashboardStats struct {
	GroupCount int
	LinkCount  int
}

// DashboardData stores dashboard view data.
type DashboardData struct {
	Stats                DashboardStats
	Groups               []model.NavGroup
	CurrentUsername      string
	CanManage            bool
	DashboardTagline     string
	DashboardDescription string
	WeatherLocation      string
	ThumbnailBackground  bool
	DashboardBg          string
	DashboardBlur        int
	DashboardOverlay     float64
	DashboardCSS         string
}

// DashboardService handles home page data.
type DashboardService struct {
	groupRepo   *repository.NavGroupRepository
	linkRepo    *repository.NavLinkRepository
	settingRepo *repository.SettingRepository
	uploadDir   string
}

// NewDashboardService creates a service.
func NewDashboardService(
	groupRepo *repository.NavGroupRepository,
	linkRepo *repository.NavLinkRepository,
	settingRepo *repository.SettingRepository,
	uploadDir string,
) *DashboardService {
	return &DashboardService{
		groupRepo:   groupRepo,
		linkRepo:    linkRepo,
		settingRepo: settingRepo,
		uploadDir:   uploadDir,
	}
}

// GetDashboardData returns dashboard content.
func (s *DashboardService) GetDashboardData(ctx context.Context, currentUsername, lang, csrfToken string) (*DashboardData, error) {
	groups, err := s.groupRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	totalLinks := 0
	for groupIndex := range groups {
		groups[groupIndex].Lang = strings.TrimSpace(lang)
		groups[groupIndex].CSRFToken = strings.TrimSpace(csrfToken)
		assignGroupGridSpan(&groups[groupIndex])
		for linkIndex := range groups[groupIndex].NavLinks {
			groups[groupIndex].NavLinks[linkIndex].Lang = groups[groupIndex].Lang
			groups[groupIndex].NavLinks[linkIndex].CSRFToken = groups[groupIndex].CSRFToken
			if !HasStoredTheme(groups[groupIndex].NavLinks[linkIndex]) {
				ApplyLinkTheme(&groups[groupIndex].NavLinks[linkIndex], BuildLinkTheme(
					s.uploadDir,
					groups[groupIndex].NavLinks[linkIndex].IconCachedPath,
					groups[groupIndex].NavLinks[linkIndex].URL,
					groups[groupIndex].NavLinks[linkIndex].Title,
				))
			}
		}
		totalLinks += len(groups[groupIndex].NavLinks)
	}

	background, err := s.settingRepo.FindByKey(ctx, dashboardBackgroundKey)
	if err != nil {
		return nil, err
	}
	blur, err := s.settingRepo.FindByKey(ctx, dashboardBlurKey)
	if err != nil {
		return nil, err
	}
	overlay, err := s.settingRepo.FindByKey(ctx, dashboardOverlayOpacityKey)
	if err != nil {
		return nil, err
	}
	taglineSetting, err := s.settingRepo.FindByKey(ctx, dashboardTaglineKey)
	if err != nil {
		return nil, err
	}
	descriptionSetting, err := s.settingRepo.FindByKey(ctx, dashboardDescriptionKey)
	if err != nil {
		return nil, err
	}
	weatherLocationSetting, err := s.settingRepo.FindByKey(ctx, dashboardWeatherLocationKey)
	if err != nil {
		return nil, err
	}
	thumbnailBackgroundSetting, err := s.settingRepo.FindByKey(ctx, dashboardThumbnailBgKey)
	if err != nil {
		return nil, err
	}

	backgroundValue := ""
	if background != nil && background.Value != "" && strings.TrimSpace(background.Value) != "" {
		if runtimepath.IsBackgroundPublicPath(background.Value) {
			backgroundValue = strings.TrimSpace(background.Value)
		}
	}

	blurValue := defaultDashboardBlur
	if blur != nil {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(blur.Value)); parseErr == nil && parsed >= 0 && parsed <= 40 {
			blurValue = parsed
		}
	}

	overlayValue := defaultDashboardOverlay
	if overlay != nil {
		if parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(overlay.Value), 64); parseErr == nil {
			switch {
			case parsed < 0:
				overlayValue = defaultDashboardOverlay
			case parsed > maxDashboardOverlay:
				overlayValue = maxDashboardOverlay
			default:
				overlayValue = parsed
			}
		}
	}

	taglineValue := "TileDock"
	if taglineSetting != nil && strings.TrimSpace(taglineSetting.Value) != "" {
		taglineValue = strings.TrimSpace(taglineSetting.Value)
	}
	descriptionValue := defaultDashboardDescription(lang)
	if descriptionSetting != nil && strings.TrimSpace(descriptionSetting.Value) != "" {
		descriptionValue = strings.TrimSpace(descriptionSetting.Value)
	}
	weatherLocationValue := ""
	if weatherLocationSetting != nil {
		weatherLocationValue = strings.TrimSpace(weatherLocationSetting.Value)
	}
	thumbnailBackgroundValue := false
	if thumbnailBackgroundSetting != nil {
		thumbnailBackgroundValue = settingBool(thumbnailBackgroundSetting.Value)
	}
	if thumbnailBackgroundValue {
		for groupIndex := range groups {
			for linkIndex := range groups[groupIndex].NavLinks {
				groups[groupIndex].NavLinks[linkIndex].UseThumbnailBackground = true
			}
		}
	}

	css := fmt.Sprintf(".dashboard-page::before { background: rgba(0, 0, 0, %.2f); backdrop-filter: blur(%dpx); -webkit-backdrop-filter: blur(%dpx); }", overlayValue, blurValue, blurValue)
	if backgroundValue != "" {
		css = fmt.Sprintf("%s .dashboard-page { background-image: url('%s'); }", css, backgroundValue)
	}

	return &DashboardData{
		Stats: DashboardStats{
			GroupCount: len(groups),
			LinkCount:  totalLinks,
		},
		Groups:               groups,
		CurrentUsername:      strings.TrimSpace(currentUsername),
		CanManage:            strings.TrimSpace(currentUsername) != "",
		DashboardTagline:     taglineValue,
		DashboardDescription: descriptionValue,
		WeatherLocation:      weatherLocationValue,
		ThumbnailBackground:  thumbnailBackgroundValue,
		DashboardBg:          backgroundValue,
		DashboardBlur:        blurValue,
		DashboardOverlay:     overlayValue,
		DashboardCSS:         css,
	}, nil
}

func defaultDashboardDescription(lang string) string {
	switch strings.TrimSpace(lang) {
	case "zh":
		return "集中管理常用网站、AI 工具、课程和本地服务。"
	case "ja":
		return "Frequent sites, AI tools, courses, and local services in one place."
	default:
		return "Frequent sites, AI tools, courses, and local services in one place."
	}
}

func assignGroupGridSpan(group *model.NavGroup) {
	if group.GridCols > 0 && group.GridRows > 0 {
		group.GridCols = normalizeSavedGridCols(group.GridCols)
		group.GridRows = clampInt(group.GridRows, 8, 28)
		return
	}

	linkCount := len(group.NavLinks)
	group.GridCols = 10
	group.GridRows = 10

	switch {
	case linkCount >= 10:
		group.GridCols = 24
		group.GridRows = 20
	case linkCount >= 6:
		group.GridCols = 18
		group.GridRows = 16
	case linkCount >= 3:
		group.GridCols = 12
		group.GridRows = 13
	}

	if linkCount == 0 {
		group.GridRows = maxInt(group.GridRows, 9)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func normalizeSavedGridCols(cols int) int {
	if cols <= 0 {
		return 10
	}
	if cols <= 4 {
		return clampInt(cols*10, 10, 36)
	}
	return clampInt(cols, 8, 36)
}

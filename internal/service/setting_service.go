package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"panel/internal/model"
	"panel/internal/repository"
)

const (
	dashboardBackgroundKey      = "dashboard_background"
	dashboardBlurKey            = "dashboard_background_blur"
	dashboardOverlayOpacityKey  = "dashboard_overlay_opacity"
	dashboardTaglineKey         = "dashboard_tagline"
	dashboardWeatherLocationKey = "dashboard_weather_location"
	defaultDashboardBlur        = 8
	defaultDashboardOverlay     = 0.38
	maxDashboardOverlay         = 0.85
)

// SettingService handles system settings.
type SettingService struct {
	repo *repository.SettingRepository
}

// NewSettingService creates a service.
func NewSettingService(repo *repository.SettingRepository) *SettingService {
	return &SettingService{repo: repo}
}

// SettingsPageData stores settings page data.
type SettingsPageData struct {
	DashboardBackground string
	DashboardBlur       int
	DashboardOverlay    float64
	DashboardTagline    string
	WeatherLocation     string
	Settings            []model.Setting
}

// List returns settings.
func (s *SettingService) List(ctx context.Context) (*SettingsPageData, error) {
	settings, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	background, err := s.repo.FindByKey(ctx, dashboardBackgroundKey)
	if err != nil {
		return nil, err
	}
	blur, err := s.repo.FindByKey(ctx, dashboardBlurKey)
	if err != nil {
		return nil, err
	}
	overlay, err := s.repo.FindByKey(ctx, dashboardOverlayOpacityKey)
	if err != nil {
		return nil, err
	}
	tagline, err := s.repo.FindByKey(ctx, dashboardTaglineKey)
	if err != nil {
		return nil, err
	}
	weatherLocation, err := s.repo.FindByKey(ctx, dashboardWeatherLocationKey)
	if err != nil {
		return nil, err
	}

	data := &SettingsPageData{
		Settings:         settings,
		DashboardBlur:    defaultDashboardBlur,
		DashboardOverlay: defaultDashboardOverlay,
	}
	if background != nil {
		data.DashboardBackground = background.Value
	}
	if blur != nil {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(blur.Value)); parseErr == nil && parsed >= 0 && parsed <= 40 {
			data.DashboardBlur = parsed
		}
	}
	if overlay != nil {
		if parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(overlay.Value), 64); parseErr == nil && parsed >= 0 && parsed <= maxDashboardOverlay {
			data.DashboardOverlay = parsed
		}
	}
	if tagline != nil {
		data.DashboardTagline = strings.TrimSpace(tagline.Value)
	}
	if weatherLocation != nil {
		data.WeatherLocation = strings.TrimSpace(weatherLocation.Value)
	}
	return data, nil
}

// Save stores a setting key and value.
func (s *SettingService) Save(ctx context.Context, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("setting key is required")
	}

	return s.repo.Upsert(ctx, &model.Setting{
		Key:   key,
		Value: strings.TrimSpace(value),
	})
}

// SaveDashboardBackground stores dashboard background.
func (s *SettingService) SaveDashboardBackground(ctx context.Context, value string) error {
	return s.repo.Upsert(ctx, &model.Setting{
		Key:   dashboardBackgroundKey,
		Value: strings.TrimSpace(value),
	})
}

// SaveDashboardAppearance stores dashboard blur and overlay settings.
func (s *SettingService) SaveDashboardAppearance(ctx context.Context, blur int, overlay float64) error {
	if blur < 0 || blur > 40 {
		return errors.New("背景虚化范围必须在 0 到 40 之间")
	}
	if overlay < 0 || overlay > maxDashboardOverlay {
		return fmt.Errorf("黑色蒙板透明度范围必须在 0 到 %.2f 之间", maxDashboardOverlay)
	}

	if err := s.repo.Upsert(ctx, &model.Setting{
		Key:   dashboardBlurKey,
		Value: strconv.Itoa(blur),
	}); err != nil {
		return err
	}

	return s.repo.Upsert(ctx, &model.Setting{
		Key:   dashboardOverlayOpacityKey,
		Value: fmt.Sprintf("%.2f", overlay),
	})
}

// SaveDashboardBranding stores dashboard tagline.
func (s *SettingService) SaveDashboardBranding(ctx context.Context, tagline string) error {
	return s.repo.Upsert(ctx, &model.Setting{
		Key:   dashboardTaglineKey,
		Value: strings.TrimSpace(tagline),
	})
}

// SaveDashboardWeatherLocation stores the dashboard weather location.
func (s *SettingService) SaveDashboardWeatherLocation(ctx context.Context, location string) error {
	return s.repo.Upsert(ctx, &model.Setting{
		Key:   dashboardWeatherLocationKey,
		Value: strings.TrimSpace(location),
	})
}

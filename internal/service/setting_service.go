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
	dashboardDescriptionKey     = "dashboard_description"
	dashboardWeatherLocationKey = "dashboard_weather_location"
	dashboardThumbnailBgKey     = "dashboard_thumbnail_background_enabled"
	aiEnabledKey                = "ai_enabled"
	aiProviderKey               = "ai_provider"
	aiBaseURLKey                = "ai_base_url"
	aiAPIKeyKey                 = "ai_api_key"
	aiModelKey                  = "ai_model"
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
	DashboardBackground  string
	DashboardBlur        int
	DashboardOverlay     float64
	DashboardTagline     string
	DashboardDescription string
	WeatherLocation      string
	ThumbnailBackground  bool
	AI                   AIConfig
	Settings             []model.Setting
}

// AIConfig stores the configured AI provider.
type AIConfig struct {
	Enabled  bool
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
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
	description, err := s.repo.FindByKey(ctx, dashboardDescriptionKey)
	if err != nil {
		return nil, err
	}
	weatherLocation, err := s.repo.FindByKey(ctx, dashboardWeatherLocationKey)
	if err != nil {
		return nil, err
	}
	thumbnailBackground, err := s.repo.FindByKey(ctx, dashboardThumbnailBgKey)
	if err != nil {
		return nil, err
	}
	aiConfig, err := s.LoadAIConfig(ctx)
	if err != nil {
		return nil, err
	}

	data := &SettingsPageData{
		Settings:         settings,
		DashboardBlur:    defaultDashboardBlur,
		DashboardOverlay: defaultDashboardOverlay,
		AI:               aiConfig,
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
	if description != nil {
		data.DashboardDescription = strings.TrimSpace(description.Value)
	}
	if weatherLocation != nil {
		data.WeatherLocation = strings.TrimSpace(weatherLocation.Value)
	}
	if thumbnailBackground != nil {
		data.ThumbnailBackground = settingBool(thumbnailBackground.Value)
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

// SaveDashboardDescription stores the dashboard summary text.
func (s *SettingService) SaveDashboardDescription(ctx context.Context, description string) error {
	return s.repo.Upsert(ctx, &model.Setting{
		Key:   dashboardDescriptionKey,
		Value: strings.TrimSpace(description),
	})
}

// SaveDashboardWeatherLocation stores the dashboard weather location.
func (s *SettingService) SaveDashboardWeatherLocation(ctx context.Context, location string) error {
	return s.repo.Upsert(ctx, &model.Setting{
		Key:   dashboardWeatherLocationKey,
		Value: strings.TrimSpace(location),
	})
}

// SaveDashboardThumbnailBackground stores whether link cards use cached website thumbnails.
func (s *SettingService) SaveDashboardThumbnailBackground(ctx context.Context, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return s.repo.Upsert(ctx, &model.Setting{
		Key:   dashboardThumbnailBgKey,
		Value: value,
	})
}

// SaveAIConfig stores AI provider settings.
func (s *SettingService) SaveAIConfig(ctx context.Context, cfg AIConfig) error {
	cfg = NormalizeAIConfig(cfg)
	provider := cfg.Provider
	values := map[string]string{
		aiEnabledKey:  boolString(cfg.Enabled && provider != "disabled"),
		aiProviderKey: provider,
		aiBaseURLKey:  strings.TrimSpace(cfg.BaseURL),
		aiAPIKeyKey:   strings.TrimSpace(cfg.APIKey),
		aiModelKey:    strings.TrimSpace(cfg.Model),
	}
	for key, value := range values {
		if err := s.repo.Upsert(ctx, &model.Setting{Key: key, Value: value}); err != nil {
			return err
		}
	}
	return nil
}

// LoadAIConfig returns normalized AI settings.
func (s *SettingService) LoadAIConfig(ctx context.Context) (AIConfig, error) {
	cfg := AIConfig{Provider: "disabled"}
	settings, err := s.repo.List(ctx)
	if err != nil {
		return cfg, err
	}
	for _, setting := range settings {
		switch strings.TrimSpace(setting.Key) {
		case aiEnabledKey:
			cfg.Enabled = settingBool(setting.Value)
		case aiProviderKey:
			cfg.Provider = normalizeAIProvider(setting.Value)
		case aiBaseURLKey:
			cfg.BaseURL = strings.TrimSpace(setting.Value)
		case aiAPIKeyKey:
			cfg.APIKey = strings.TrimSpace(setting.Value)
		case aiModelKey:
			cfg.Model = strings.TrimSpace(setting.Value)
		}
	}
	cfg.Provider = normalizeAIProvider(cfg.Provider)
	if cfg.Provider == "disabled" {
		cfg.Enabled = false
	}
	return NormalizeAIConfig(cfg), nil
}

// NormalizeAIConfig fills provider defaults and normalizes user input.
func NormalizeAIConfig(cfg AIConfig) AIConfig {
	cfg.Provider = normalizeAIProvider(cfg.Provider)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Provider == "disabled" {
		cfg.Enabled = false
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultAIBaseURL(cfg.Provider)
	}
	if cfg.Model == "" {
		cfg.Model = defaultAIModel(cfg.Provider)
	}
	return cfg
}

func normalizeAIProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "ollama", "lmstudio", "custom":
		return strings.ToLower(strings.TrimSpace(provider))
	default:
		return "disabled"
	}
}

func defaultAIBaseURL(provider string) string {
	switch normalizeAIProvider(provider) {
	case "openai":
		return "https://api.openai.com/v1"
	case "ollama":
		return "http://localhost:11434/v1"
	case "lmstudio":
		return "http://localhost:1234/v1"
	default:
		return ""
	}
}

func defaultAIModel(provider string) string {
	switch normalizeAIProvider(provider) {
	case "openai":
		return "gpt-4o-mini"
	case "ollama":
		return "llama3.1"
	case "lmstudio":
		return "local-model"
	default:
		return ""
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func settingBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

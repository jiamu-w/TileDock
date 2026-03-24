package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"panel/internal/i18n"
	"panel/internal/repository"
)

const weatherCacheTTL = 10 * time.Minute

// WeatherData stores the public weather payload for the dashboard.
type WeatherData struct {
	Location    string `json:"location"`
	Temperature string `json:"temperature"`
	Condition   string `json:"condition"`
	Icon        string `json:"icon"`
	UpdatedAt   string `json:"updated_at"`
}

type cachedWeather struct {
	data      *WeatherData
	expiresAt time.Time
}

// WeatherService fetches current weather for the configured dashboard location.
type WeatherService struct {
	settingRepo *repository.SettingRepository
	client      *http.Client

	mu    sync.Mutex
	cache map[string]cachedWeather
}

// NewWeatherService creates a weather service.
func NewWeatherService(settingRepo *repository.SettingRepository) *WeatherService {
	return &WeatherService{
		settingRepo: settingRepo,
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
		cache: make(map[string]cachedWeather),
	}
}

// GetConfiguredLocation returns the saved dashboard weather location.
func (s *WeatherService) GetConfiguredLocation(ctx context.Context) (string, error) {
	setting, err := s.settingRepo.FindByKey(ctx, dashboardWeatherLocationKey)
	if err != nil {
		return "", err
	}
	if setting == nil {
		return "", nil
	}
	return strings.TrimSpace(setting.Value), nil
}

// GetCurrent returns the current weather for the configured location.
func (s *WeatherService) GetCurrent(ctx context.Context, lang string) (*WeatherData, error) {
	location, err := s.GetConfiguredLocation(ctx)
	if err != nil {
		return nil, err
	}
	if location == "" {
		return nil, nil
	}

	cacheKey := i18n.Normalize(lang) + ":" + strings.ToLower(location)
	if data := s.readCache(cacheKey); data != nil {
		return data, nil
	}

	geo, err := s.lookupLocation(ctx, location, lang)
	if err != nil {
		return nil, err
	}

	current, err := s.lookupCurrentWeather(ctx, geo.Latitude, geo.Longitude)
	if err != nil {
		return nil, err
	}

	condition, icon := describeWeather(current.WeatherCode, lang)
	data := &WeatherData{
		Location:    geo.DisplayName,
		Temperature: fmt.Sprintf("%.0f°C", current.Temperature),
		Condition:   condition,
		Icon:        icon,
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}

	s.writeCache(cacheKey, data)
	return data, nil
}

func (s *WeatherService) readCache(key string) *WeatherData {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(s.cache, key)
		return nil
	}
	return entry.data
}

func (s *WeatherService) writeCache(key string, data *WeatherData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[key] = cachedWeather{
		data:      data,
		expiresAt: time.Now().Add(weatherCacheTTL),
	}
}

type geocodeResponse struct {
	Results []struct {
		Name      string  `json:"name"`
		Admin1    string  `json:"admin1"`
		Country   string  `json:"country"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"results"`
}

type geocodeResult struct {
	DisplayName string
	Latitude    float64
	Longitude   float64
}

func (s *WeatherService) lookupLocation(ctx context.Context, location, lang string) (*geocodeResult, error) {
	query := url.Values{}
	query.Set("name", location)
	query.Set("count", "1")
	query.Set("language", i18n.Normalize(lang))
	query.Set("format", "json")

	endpoint := "https://geocoding-api.open-meteo.com/v1/search?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather geocoding returned status %d", resp.StatusCode)
	}

	var payload geocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Results) == 0 {
		return nil, errors.New("weather location not found")
	}

	result := payload.Results[0]
	parts := []string{strings.TrimSpace(result.Name)}
	if admin := strings.TrimSpace(result.Admin1); admin != "" && !strings.EqualFold(admin, result.Name) {
		parts = append(parts, admin)
	}
	if country := strings.TrimSpace(result.Country); country != "" {
		parts = append(parts, country)
	}

	return &geocodeResult{
		DisplayName: strings.Join(parts, ", "),
		Latitude:    result.Latitude,
		Longitude:   result.Longitude,
	}, nil
}

type forecastResponse struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		WeatherCode int     `json:"weather_code"`
	} `json:"current"`
}

type forecastCurrent struct {
	Temperature float64
	WeatherCode int
}

func (s *WeatherService) lookupCurrentWeather(ctx context.Context, latitude, longitude float64) (*forecastCurrent, error) {
	query := url.Values{}
	query.Set("latitude", fmt.Sprintf("%.6f", latitude))
	query.Set("longitude", fmt.Sprintf("%.6f", longitude))
	query.Set("current", "temperature_2m,weather_code")
	query.Set("timezone", "auto")

	endpoint := "https://api.open-meteo.com/v1/forecast?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather forecast returned status %d", resp.StatusCode)
	}

	var payload forecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return &forecastCurrent{
		Temperature: payload.Current.Temperature,
		WeatherCode: payload.Current.WeatherCode,
	}, nil
}

func describeWeather(code int, lang string) (string, string) {
	switch code {
	case 0:
		return weatherText(lang, "clear"), "☀️"
	case 1:
		return weatherText(lang, "mainly_clear"), "🌤️"
	case 2:
		return weatherText(lang, "partly_cloudy"), "⛅"
	case 3:
		return weatherText(lang, "overcast"), "☁️"
	case 45, 48:
		return weatherText(lang, "fog"), "🌫️"
	case 51, 53, 55:
		return weatherText(lang, "drizzle"), "🌦️"
	case 56, 57:
		return weatherText(lang, "freezing_drizzle"), "🌧️"
	case 61, 63, 65:
		return weatherText(lang, "rain"), "🌧️"
	case 66, 67:
		return weatherText(lang, "freezing_rain"), "🌧️"
	case 71, 73, 75, 77:
		return weatherText(lang, "snow"), "🌨️"
	case 80, 81, 82:
		return weatherText(lang, "rain_showers"), "🌦️"
	case 85, 86:
		return weatherText(lang, "snow_showers"), "🌨️"
	case 95, 96, 99:
		return weatherText(lang, "thunderstorm"), "⛈️"
	default:
		return weatherText(lang, "unknown"), "🌡️"
	}
}

func weatherText(lang, key string) string {
	normalized := i18n.Normalize(lang)

	translations := map[string]map[string]string{
		"zh": {
			"clear":            "晴朗",
			"mainly_clear":     "大致晴朗",
			"partly_cloudy":    "局部多云",
			"overcast":         "阴天",
			"fog":              "有雾",
			"drizzle":          "毛毛雨",
			"freezing_drizzle": "冻毛毛雨",
			"rain":             "下雨",
			"freezing_rain":    "冻雨",
			"snow":             "下雪",
			"rain_showers":     "阵雨",
			"snow_showers":     "阵雪",
			"thunderstorm":     "雷暴",
			"unknown":          "天气未知",
		},
		"en": {
			"clear":            "Clear",
			"mainly_clear":     "Mostly Clear",
			"partly_cloudy":    "Partly Cloudy",
			"overcast":         "Overcast",
			"fog":              "Fog",
			"drizzle":          "Drizzle",
			"freezing_drizzle": "Freezing Drizzle",
			"rain":             "Rain",
			"freezing_rain":    "Freezing Rain",
			"snow":             "Snow",
			"rain_showers":     "Showers",
			"snow_showers":     "Snow Showers",
			"thunderstorm":     "Thunderstorm",
			"unknown":          "Unknown",
		},
		"ja": {
			"clear":            "快晴",
			"mainly_clear":     "おおむね晴れ",
			"partly_cloudy":    "ところにより曇り",
			"overcast":         "曇天",
			"fog":              "霧",
			"drizzle":          "霧雨",
			"freezing_drizzle": "着氷性の霧雨",
			"rain":             "雨",
			"freezing_rain":    "着氷性の雨",
			"snow":             "雪",
			"rain_showers":     "にわか雨",
			"snow_showers":     "にわか雪",
			"thunderstorm":     "雷雨",
			"unknown":          "不明",
		},
	}

	if values, ok := translations[normalized]; ok {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return translations[i18n.DefaultLang][key]
}

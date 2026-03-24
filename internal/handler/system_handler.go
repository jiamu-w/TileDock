package handler

import (
	"log/slog"
	"net/http"

	"panel/internal/i18n"
	"panel/internal/service"

	"github.com/gin-gonic/gin"
)

// SystemHandler handles infra endpoints.
type SystemHandler struct {
	log     *slog.Logger
	weather *service.WeatherService
}

// NewSystemHandler creates a handler.
func NewSystemHandler(log *slog.Logger, weather *service.WeatherService) *SystemHandler {
	return &SystemHandler{log: log, weather: weather}
}

// Healthz returns server health.
func (h *SystemHandler) Healthz(c *gin.Context) {
	h.log.Debug("healthz called")
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// CurrentWeather returns the current weather for the configured location.
func (h *SystemHandler) CurrentWeather(c *gin.Context) {
	lang := i18n.FromContext(c)
	if h.weather == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}

	data, err := h.weather.GetCurrent(c.Request.Context(), lang)
	if err != nil {
		h.log.Warn("fetch weather failed", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"enabled": false,
			"error":   i18n.T(lang, "weather.unavailable"),
		})
		return
	}
	if data == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":     true,
		"location":    data.Location,
		"temperature": data.Temperature,
		"condition":   data.Condition,
		"icon":        data.Icon,
		"updated_at":  data.UpdatedAt,
	})
}

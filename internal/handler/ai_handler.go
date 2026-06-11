package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"panel/internal/i18n"
	"panel/internal/service"

	"github.com/gin-gonic/gin"
)

// AIHandler handles optional AI-assisted APIs.
type AIHandler struct {
	settings *service.SettingService
	nav      *service.NavigationService
	ai       *service.AIService
	log      *slog.Logger
}

// NewAIHandler creates an AI handler.
func NewAIHandler(settings *service.SettingService, nav *service.NavigationService, ai *service.AIService, log *slog.Logger) *AIHandler {
	return &AIHandler{settings: settings, nav: nav, ai: ai, log: log}
}

// EnrichLink returns AI-generated link metadata.
func (h *AIHandler) EnrichLink(c *gin.Context) {
	var input service.LinkEnrichRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	cfg, err := h.settings.LoadAIConfig(c.Request.Context())
	if err != nil || !h.ai.Enabled(cfg) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI is not configured"})
		return
	}
	pageData, err := h.nav.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load groups"})
		return
	}
	input.Lang = i18n.FromContext(c)
	result, err := h.ai.EnrichLink(c.Request.Context(), cfg, input, pageData.Groups)
	if err != nil {
		h.log.Warn("AI enrich failed", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// Search returns AI-ranked link ids.
func (h *AIHandler) Search(c *gin.Context) {
	var input struct {
		Query string `json:"query"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Query) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
		return
	}
	cfg, err := h.settings.LoadAIConfig(c.Request.Context())
	if err != nil || !h.ai.Enabled(cfg) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI is not configured"})
		return
	}
	pageData, err := h.nav.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load links"})
		return
	}

	items := make([]service.AISearchItem, 0)
	for _, group := range pageData.Groups {
		for _, link := range group.NavLinks {
			items = append(items, service.AISearchItem{
				ID:          link.ID,
				Title:       link.Title,
				Description: link.Description,
				GroupName:   group.Name,
				URL:         link.URL,
			})
		}
	}
	result, err := h.ai.SearchLinks(c.Request.Context(), cfg, input.Query, items, i18n.FromContext(c))
	if err != nil {
		h.log.Warn("AI search failed", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// TestConfig checks the provider settings submitted from the settings form.
func (h *AIHandler) TestConfig(c *gin.Context) {
	var input service.AIConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	existing, _ := h.settings.LoadAIConfig(c.Request.Context())
	if strings.TrimSpace(input.APIKey) == "" {
		input.APIKey = existing.APIKey
	}
	cfg := service.NormalizeAIConfig(input)
	if err := h.ai.TestConnection(c.Request.Context(), cfg); err != nil {
		h.log.Warn("AI config test failed", "error", err, "provider", cfg.Provider)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "provider": cfg.Provider, "model": cfg.Model})
}

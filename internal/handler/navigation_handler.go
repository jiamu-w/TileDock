package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"panel/internal/i18n"
	"panel/internal/service"
	"panel/internal/view"
	"panel/pkg/runtimepath"

	"github.com/gin-gonic/gin"
)

// NavigationHandler handles navigation CRUD pages.
type NavigationHandler struct {
	renderer         *view.Renderer
	service          *service.NavigationService
	faviconService   *service.FaviconService
	thumbnailService *service.ThumbnailService
	settingService   *service.SettingService
	aiService        *service.AIService
	log              *slog.Logger
	uploadDir        string
}

// NewNavigationHandler creates a handler.
func NewNavigationHandler(renderer *view.Renderer, service *service.NavigationService, faviconService *service.FaviconService, thumbnailService *service.ThumbnailService, settingService *service.SettingService, aiService *service.AIService, log *slog.Logger, uploadDir string) *NavigationHandler {
	return &NavigationHandler{renderer: renderer, service: service, faviconService: faviconService, thumbnailService: thumbnailService, settingService: settingService, aiService: aiService, log: log, uploadDir: uploadDir}
}

// CreateGroup handles group creation.
func (h *NavigationHandler) CreateGroup(c *gin.Context) {
	name := c.PostForm("name")
	if err := h.service.CreateGroup(c.Request.Context(), c.PostForm("name")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	auditLog(h.log, c, "group.create", "name", name)
	redirectBack(c, "/")
}

// UpdateGroup handles group updates.
func (h *NavigationHandler) UpdateGroup(c *gin.Context) {
	groupID := c.Param("id")
	name := c.PostForm("name")
	if err := h.service.UpdateGroup(c.Request.Context(), c.Param("id"), c.PostForm("name")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	auditLog(h.log, c, "group.update", "group_id", groupID, "name", name)
	redirectBack(c, "/")
}

// DeleteGroup handles group deletion.
func (h *NavigationHandler) DeleteGroup(c *gin.Context) {
	groupID := c.Param("id")
	if err := h.service.DeleteGroup(c.Request.Context(), groupID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	auditLog(h.log, c, "group.delete", "group_id", groupID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// CreateLink handles link creation.
func (h *NavigationHandler) CreateLink(c *gin.Context) {
	iconPath, err := h.resolveCreateIcon(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := service.LinkInput{
		GroupID:        c.PostForm("group_id"),
		Title:          c.PostForm("title"),
		URL:            c.PostForm("url"),
		Description:    c.PostForm("description"),
		Icon:           iconPath,
		IconCachedPath: cachedIconPath(iconPath),
		OpenInNew:      c.PostForm("open_in_new") == "on",
	}
	h.applyAIEnrichment(c, &input)
	theme := service.BuildLinkTheme(h.uploadDir, input.IconCachedPath, input.URL, input.Title)
	input.ThemeAccentColor = theme.AccentColor
	input.ThemeBgStartColor = theme.BgStartColor
	input.ThemeBgEndColor = theme.BgEndColor
	input.ThemeBorderColor = theme.BorderColor
	input.ThemeTextColor = theme.TextColor
	linkID, err := h.service.CreateLink(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.IconCachedPath == "" && h.faviconService != nil {
		if err := h.faviconService.EnqueueLink(c.Request.Context(), linkID); err != nil {
			h.log.Warn("enqueue favicon failed", "error", err, "link_id", linkID)
		}
	}
	if h.thumbnailService != nil {
		if err := h.thumbnailService.EnqueueLink(c.Request.Context(), linkID); err != nil {
			h.log.Warn("enqueue thumbnail failed", "error", err, "link_id", linkID)
		}
	}
	auditLog(h.log, c, "link.create", "group_id", input.GroupID, "title", input.Title, "url", input.URL)
	redirectBack(c, "/")
}

func (h *NavigationHandler) applyAIEnrichment(c *gin.Context, input *service.LinkInput) {
	if h.aiService == nil || h.settingService == nil || input == nil || strings.TrimSpace(input.URL) == "" {
		return
	}
	cfg, err := h.settingService.LoadAIConfig(c.Request.Context())
	if err != nil || !h.aiService.Enabled(cfg) {
		return
	}
	pageData, err := h.service.List(c.Request.Context())
	if err != nil {
		h.log.Warn("load groups for AI enrichment failed", "error", err)
		return
	}
	result, err := h.aiService.EnrichLink(c.Request.Context(), cfg, service.LinkEnrichRequest{
		URL:         input.URL,
		Title:       input.Title,
		Description: input.Description,
		GroupID:     input.GroupID,
		Lang:        i18n.FromContext(c),
	}, pageData.Groups)
	if err != nil {
		h.log.Warn("AI link enrichment failed", "error", err)
		return
	}
	if strings.TrimSpace(input.Title) == "" && strings.TrimSpace(result.Title) != "" {
		input.Title = result.Title
	}
	if strings.TrimSpace(input.Description) == "" && strings.TrimSpace(result.Description) != "" {
		input.Description = result.Description
	}
	if strings.TrimSpace(result.GroupID) != "" {
		input.GroupID = result.GroupID
	}
}

// UpdateLink handles link updates.
func (h *NavigationHandler) UpdateLink(c *gin.Context) {
	iconPath, iconCachedPath, removeOld, scheduleIconFetch, err := h.resolveUpdateIcon(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	urlChanged := c.PostForm("existing_url") != "" && strings.TrimSpace(c.PostForm("existing_url")) != strings.TrimSpace(c.PostForm("url"))
	thumbnailCachedPath := sanitizeThumbnailPath(h.uploadDir, c.PostForm("existing_thumbnail_cached_path"))
	if urlChanged {
		thumbnailCachedPath = ""
	}

	input := service.LinkInput{
		GroupID:                c.PostForm("group_id"),
		Title:                  c.PostForm("title"),
		URL:                    c.PostForm("url"),
		Description:            c.PostForm("description"),
		Icon:                   iconPath,
		IconCachedPath:         iconCachedPath,
		ThumbnailCachedPath:    thumbnailCachedPath,
		ScheduleIconFetch:      scheduleIconFetch,
		ScheduleThumbnailFetch: urlChanged,
		OpenInNew:              c.PostForm("open_in_new") == "on",
	}
	theme := service.BuildLinkTheme(h.uploadDir, input.IconCachedPath, input.URL, input.Title)
	input.ThemeAccentColor = theme.AccentColor
	input.ThemeBgStartColor = theme.BgStartColor
	input.ThemeBgEndColor = theme.BgEndColor
	input.ThemeBorderColor = theme.BorderColor
	input.ThemeTextColor = theme.TextColor
	if err := h.service.UpdateLink(c.Request.Context(), c.Param("id"), input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if removeOld != "" {
		if err := removeIconFile(h.uploadDir, removeOld); err != nil {
			h.log.Warn("remove old icon failed", "error", err, "path", removeOld)
		}
	}
	if scheduleIconFetch && h.faviconService != nil {
		if err := h.faviconService.EnqueueLink(c.Request.Context(), c.Param("id")); err != nil {
			h.log.Warn("enqueue favicon failed", "error", err, "link_id", c.Param("id"))
		}
	}
	if input.ScheduleThumbnailFetch && h.thumbnailService != nil {
		if err := h.thumbnailService.EnqueueLink(c.Request.Context(), c.Param("id")); err != nil {
			h.log.Warn("enqueue thumbnail failed", "error", err, "link_id", c.Param("id"))
		}
	}
	auditLog(h.log, c, "link.update", "link_id", c.Param("id"), "group_id", input.GroupID, "title", input.Title, "url", input.URL)
	redirectBack(c, "/")
}

// RefreshLinkIcon schedules an immediate favicon refresh.
func (h *NavigationHandler) RefreshLinkIcon(c *gin.Context) {
	linkID := c.Param("id")
	if h.faviconService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "favicon service unavailable"})
		return
	}
	if err := h.faviconService.RefreshLink(c.Request.Context(), linkID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	auditLog(h.log, c, "link.favicon.refresh", "link_id", linkID)
	redirectBack(c, "/")
}

// RefreshLinkThumbnail schedules an immediate website thumbnail refresh.
func (h *NavigationHandler) RefreshLinkThumbnail(c *gin.Context) {
	linkID := c.Param("id")
	if h.thumbnailService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "thumbnail service unavailable"})
		return
	}
	if err := h.thumbnailService.RefreshLink(c.Request.Context(), linkID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	auditLog(h.log, c, "link.thumbnail.refresh", "link_id", linkID)
	redirectBack(c, "/")
}

// DeleteLink handles link deletion.
func (h *NavigationHandler) DeleteLink(c *gin.Context) {
	linkID := c.Param("id")
	if err := h.service.DeleteLink(c.Request.Context(), linkID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	auditLog(h.log, c, "link.delete", "link_id", linkID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Reorder handles drag-sort updates.
func (h *NavigationHandler) Reorder(c *gin.Context) {
	var req service.ReorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reorder payload"})
		return
	}

	if err := h.service.Reorder(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	auditLog(h.log, c, "navigation.reorder", "group_count", len(req.GroupIDs), "link_count", len(req.Links))

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ResizeGroup handles saved group tile size updates.
func (h *NavigationHandler) ResizeGroup(c *gin.Context) {
	groupID := c.Param("id")
	var size service.GroupGridSize
	if err := c.ShouldBindJSON(&size); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resize payload"})
		return
	}

	if err := h.service.ResizeGroup(c.Request.Context(), groupID, size); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	auditLog(h.log, c, "group.resize", "group_id", groupID, "cols", size.Cols, "rows", size.Rows)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func redirectBack(c *gin.Context, fallback string) {
	referer := c.GetHeader("Referer")
	if referer == "" {
		c.Redirect(http.StatusFound, fallback)
		return
	}
	c.Redirect(http.StatusFound, referer)
}

func (h *NavigationHandler) resolveCreateIcon(c *gin.Context) (string, error) {
	file, err := c.FormFile("icon_file")
	if err != nil && err != http.ErrMissingFile {
		return "", err
	}
	if err == nil && file != nil && file.Filename != "" {
		return saveUploadedIcon(h.uploadDir, file)
	}

	iconPath := sanitizeIconPath(h.uploadDir, c.PostForm("icon"))
	if strings.TrimSpace(iconPath) != "" {
		return iconPath, nil
	}

	return "", nil
}

func (h *NavigationHandler) resolveUpdateIcon(c *gin.Context) (string, string, string, bool, error) {
	existingIcon := sanitizeIconPath(h.uploadDir, c.PostForm("existing_icon"))
	existingCachedIcon := sanitizeIconPath(h.uploadDir, c.PostForm("existing_icon_cached_path"))
	iconPath := sanitizeIconPath(h.uploadDir, c.PostForm("icon"))
	clearIcon := c.PostForm("clear_icon") == "on"

	file, err := c.FormFile("icon_file")
	if err != nil && err != http.ErrMissingFile {
		return "", "", "", false, err
	}
	if err == nil && file != nil && file.Filename != "" {
		savedPath, saveErr := saveUploadedIcon(h.uploadDir, file)
		if saveErr != nil {
			return "", "", "", false, saveErr
		}
		removeOld := ""
		if existingIcon != "" && existingIcon != savedPath {
			removeOld = existingIcon
		}
		return savedPath, savedPath, removeOld, false, nil
	}

	if clearIcon {
		return "", "", existingIcon, true, nil
	}

	if iconPath != "" {
		removeOld := ""
		if existingIcon != "" && existingIcon != iconPath {
			removeOld = existingIcon
		}
		return iconPath, cachedIconPath(iconPath), removeOld, cachedIconPath(iconPath) == "", nil
	}

	return existingIcon, existingCachedIcon, "", strings.TrimSpace(existingCachedIcon) == "", nil
}

func cachedIconPath(iconPath string) string {
	iconPath = strings.TrimSpace(iconPath)
	if runtimepath.IsIconPublicPath(iconPath) {
		return iconPath
	}
	return ""
}

func sanitizeThumbnailPath(uploadDir, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if runtimepath.IsThumbnailPublicPath(raw) && runtimepath.LocalUploadPathFromPublic(uploadDir, raw) != "" {
		return raw
	}
	return ""
}

package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"panel/internal/service"
	"panel/internal/view"

	"github.com/gin-gonic/gin"
)

// NavigationHandler handles navigation CRUD pages.
type NavigationHandler struct {
	renderer  *view.Renderer
	service   *service.NavigationService
	log       *slog.Logger
	uploadDir string
}

// NewNavigationHandler creates a handler.
func NewNavigationHandler(renderer *view.Renderer, service *service.NavigationService, log *slog.Logger, uploadDir string) *NavigationHandler {
	return &NavigationHandler{renderer: renderer, service: service, log: log, uploadDir: uploadDir}
}

// CreateGroup handles group creation.
func (h *NavigationHandler) CreateGroup(c *gin.Context) {
	name := c.PostForm("name")
	if err := h.service.CreateGroup(c.Request.Context(), c.PostForm("name"), c.PostForm("description")); err != nil {
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
	if err := h.service.UpdateGroup(c.Request.Context(), c.Param("id"), c.PostForm("name"), c.PostForm("description")); err != nil {
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
		GroupID:     c.PostForm("group_id"),
		Title:       c.PostForm("title"),
		URL:         c.PostForm("url"),
		Description: c.PostForm("description"),
		Icon:        iconPath,
		OpenInNew:   c.PostForm("open_in_new") == "on",
	}
	if err := h.service.CreateLink(c.Request.Context(), input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	auditLog(h.log, c, "link.create", "group_id", input.GroupID, "title", input.Title, "url", input.URL)
	redirectBack(c, "/")
}

// UpdateLink handles link updates.
func (h *NavigationHandler) UpdateLink(c *gin.Context) {
	iconPath, removeOld, err := h.resolveUpdateIcon(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := service.LinkInput{
		GroupID:     c.PostForm("group_id"),
		Title:       c.PostForm("title"),
		URL:         c.PostForm("url"),
		Description: c.PostForm("description"),
		Icon:        iconPath,
		OpenInNew:   c.PostForm("open_in_new") == "on",
	}
	if err := h.service.UpdateLink(c.Request.Context(), c.Param("id"), input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if removeOld != "" {
		if err := removeIconFile(h.uploadDir, removeOld); err != nil {
			h.log.Warn("remove old icon failed", "error", err, "path", removeOld)
		}
	}
	auditLog(h.log, c, "link.update", "link_id", c.Param("id"), "group_id", input.GroupID, "title", input.Title, "url", input.URL)
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

	return service.FetchWebsiteIcon(c.Request.Context(), c.PostForm("url"), h.uploadDir)
}

func (h *NavigationHandler) resolveUpdateIcon(c *gin.Context) (string, string, error) {
	existingIcon := sanitizeIconPath(h.uploadDir, c.PostForm("existing_icon"))
	iconPath := sanitizeIconPath(h.uploadDir, c.PostForm("icon"))
	clearIcon := c.PostForm("clear_icon") == "on"

	file, err := c.FormFile("icon_file")
	if err != nil && err != http.ErrMissingFile {
		return "", "", err
	}
	if err == nil && file != nil && file.Filename != "" {
		savedPath, saveErr := saveUploadedIcon(h.uploadDir, file)
		if saveErr != nil {
			return "", "", saveErr
		}
		removeOld := ""
		if existingIcon != "" && existingIcon != savedPath {
			removeOld = existingIcon
		}
		return savedPath, removeOld, nil
	}

	if clearIcon {
		fetchedPath, fetchErr := service.FetchWebsiteIcon(c.Request.Context(), c.PostForm("url"), h.uploadDir)
		if fetchErr != nil {
			return "", "", fetchErr
		}
		return fetchedPath, existingIcon, nil
	}

	if iconPath != "" {
		removeOld := ""
		if existingIcon != "" && existingIcon != iconPath {
			removeOld = existingIcon
		}
		return iconPath, removeOld, nil
	}

	return existingIcon, "", nil
}

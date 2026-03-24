package handler

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"panel/internal/i18n"
	"panel/internal/service"
	"panel/internal/view"
	"panel/pkg/runtimepath"

	"github.com/chai2010/webp"
	"github.com/gin-gonic/gin"
	xdraw "golang.org/x/image/draw"
)

const maxRestoreArchiveSize = 128 << 20
const maxBookmarkImportSize = 16 << 20
const (
	maxBackgroundWidth    = 1920
	maxBackgroundHeight   = 1080
	backgroundWebPQuality = 82
)

// SettingHandler handles settings pages.
type SettingHandler struct {
	renderer  *view.Renderer
	service   *service.SettingService
	auth      *service.AuthService
	backup    *service.BackupService
	bookmarks *service.BookmarkImportService
	uploadDir string
	log       *slog.Logger
}

// NewSettingHandler creates a handler.
func NewSettingHandler(
	renderer *view.Renderer,
	service *service.SettingService,
	auth *service.AuthService,
	backup *service.BackupService,
	bookmarks *service.BookmarkImportService,
	uploadDir string,
	log *slog.Logger,
) *SettingHandler {
	return &SettingHandler{
		renderer:  renderer,
		service:   service,
		auth:      auth,
		backup:    backup,
		bookmarks: bookmarks,
		uploadDir: uploadDir,
		log:       log,
	}
}

// Index renders the settings page.
func (h *SettingHandler) Index(c *gin.Context) {
	c.Redirect(http.StatusFound, "/")
}

// Save handles setting updates.
func (h *SettingHandler) Save(c *gin.Context) {
	lang := i18n.FromContext(c)
	previousBackgroundPath := sanitizeBackgroundPath(h.uploadDir, c.PostForm("existing_background"))
	backgroundPath := previousBackgroundPath
	if c.PostForm("clear_background") == "on" {
		backgroundPath = ""
	}

	file, err := c.FormFile("dashboard_background_file")
	if err != nil && err != http.ErrMissingFile {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(lang, "settings.error.read_upload")})
		return
	}

	if err == nil && file != nil && file.Filename != "" {
		savedPath, saveErr := saveBackgroundFile(c, h.uploadDir, file)
		if saveErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": saveErr.Error()})
			return
		}
		backgroundPath = savedPath
	}

	if err := h.service.SaveDashboardBackground(c.Request.Context(), backgroundPath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	blur, err := strconv.Atoi(c.DefaultPostForm("dashboard_blur", "8"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(lang, "settings.error.invalid_blur")})
		return
	}
	overlay, err := strconv.ParseFloat(c.DefaultPostForm("dashboard_overlay", "0.38"), 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.T(lang, "settings.error.invalid_overlay")})
		return
	}
	if err := h.service.SaveDashboardAppearance(c.Request.Context(), blur, overlay); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.SaveDashboardBranding(
		c.Request.Context(),
		c.PostForm("dashboard_tagline"),
	); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.SaveDashboardWeatherLocation(c.Request.Context(), c.PostForm("weather_location")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if backgroundPath != previousBackgroundPath {
		if err := removeBackgroundFile(h.uploadDir, previousBackgroundPath); err != nil {
			h.log.Warn("remove old background failed", "error", err, "path", previousBackgroundPath)
		}
	}
	auditLog(h.log, c, "settings.save", "background_changed", backgroundPath != previousBackgroundPath, "weather_location", c.PostForm("weather_location"))
	redirectWithPanelMessage(c, lang, i18n.T(lang, "settings.success.background_saved"), "")
}

// UpdateProfile updates the current username.
func (h *SettingHandler) UpdateProfile(c *gin.Context) {
	lang := i18n.FromContext(c)
	userID := currentUserID(c)
	if userID == "" {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.user_missing"))
		return
	}

	if err := h.auth.UpdateUsername(c.Request.Context(), userID, c.PostForm("username")); err != nil {
		redirectWithPanelMessage(c, lang, "", translateAuthError(lang, err))
		return
	}

	auditLog(h.log, c, "settings.username.update", "username", c.PostForm("username"))
	redirectWithPanelMessage(c, lang, i18n.T(lang, "settings.success.username_saved"), "")
}

// UpdatePassword updates the current user's password.
func (h *SettingHandler) UpdatePassword(c *gin.Context) {
	lang := i18n.FromContext(c)
	userID := currentUserID(c)
	if userID == "" {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.user_missing"))
		return
	}

	err := h.auth.UpdatePassword(
		c.Request.Context(),
		userID,
		c.PostForm("current_password"),
		c.PostForm("new_password"),
		c.PostForm("confirm_password"),
	)
	if err != nil {
		redirectWithPanelMessage(c, lang, "", translateAuthError(lang, err))
		return
	}

	auditLog(h.log, c, "settings.password.update")
	redirectWithPanelMessage(c, lang, i18n.T(lang, "settings.success.password_saved"), "")
}

// DownloadBackup creates and downloads a backup archive.
func (h *SettingHandler) DownloadBackup(c *gin.Context) {
	lang := i18n.FromContext(c)
	archive, err := h.backup.CreateBackup(c.Request.Context())
	if err != nil {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.backup_failed"))
		return
	}

	auditLog(h.log, c, "backup.download", "archive", archive.Name)
	c.FileAttachment(archive.FilePath, archive.Name)
}

// RestoreBackup restores data from an uploaded backup archive.
func (h *SettingHandler) RestoreBackup(c *gin.Context) {
	lang := i18n.FromContext(c)
	userID := currentUserID(c)
	if userID == "" {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.user_missing"))
		return
	}

	if err := h.auth.VerifyCurrentPassword(c.Request.Context(), userID, c.PostForm("current_password")); err != nil {
		redirectWithPanelMessage(c, lang, "", translateAuthError(lang, err))
		return
	}

	file, err := c.FormFile("backup_archive")
	if err != nil || file == nil {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.pick_backup"))
		return
	}
	if file.Size <= 0 {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.read_backup"))
		return
	}
	if file.Size > maxRestoreArchiveSize {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.backup_too_large"))
		return
	}

	src, err := file.Open()
	if err != nil {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.read_backup"))
		return
	}
	defer src.Close()

	payload := bytes.NewBuffer(nil)
	if _, err := payload.ReadFrom(src); err != nil {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.read_backup"))
		return
	}
	if int64(payload.Len()) > maxRestoreArchiveSize {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.error.backup_too_large"))
		return
	}

	if err := h.backup.RestoreBackup(c.Request.Context(), bytes.NewReader(payload.Bytes()), int64(payload.Len())); err != nil {
		redirectWithPanelMessage(c, lang, "", err.Error())
		return
	}

	auditLog(h.log, c, "backup.restore", "size_bytes", payload.Len())
	redirectWithPanelMessage(c, lang, i18n.T(lang, "settings.success.backup_restored"), "")
}

// ImportBookmarks imports a Chrome/Firefox bookmark HTML file.
func (h *SettingHandler) ImportBookmarks(c *gin.Context) {
	lang := i18n.FromContext(c)
	file, err := c.FormFile("bookmark_file")
	if err != nil || file == nil {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.bookmarks.error.pick_file"))
		return
	}
	if file.Size <= 0 {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.bookmarks.error.read_file"))
		return
	}
	if file.Size > maxBookmarkImportSize {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.bookmarks.error.file_too_large"))
		return
	}

	src, err := file.Open()
	if err != nil {
		redirectWithPanelMessage(c, lang, "", i18n.T(lang, "settings.bookmarks.error.read_file"))
		return
	}
	defer src.Close()

	result, err := h.bookmarks.ImportHTML(c.Request.Context(), src, i18n.T(lang, "settings.bookmarks.default_group"))
	if err != nil {
		redirectWithPanelMessage(c, lang, "", err.Error())
		return
	}

	auditLog(h.log, c, "bookmarks.import", "groups", result.GroupCount, "links", result.LinkCount, "filename", file.Filename)
	redirectWithPanelMessage(c, lang, i18n.T(lang, "settings.bookmarks.success.imported"), "")
}

func saveBackgroundFile(c *gin.Context, uploadDir string, file *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		return "", fmt.Errorf("仅支持 jpg、png、webp、gif 图片")
	}

	backgroundDir := runtimepath.BackgroundsDir(uploadDir)
	if err := os.MkdirAll(backgroundDir, 0o755); err != nil {
		return "", fmt.Errorf("创建上传目录失败: %w", err)
	}

	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("读取背景图片失败: %w", err)
	}
	defer src.Close()

	payload := bytes.NewBuffer(nil)
	if _, err := payload.ReadFrom(src); err != nil {
		return "", fmt.Errorf("读取背景图片失败: %w", err)
	}

	decoded, err := decodeBackgroundImage(payload.Bytes(), ext)
	if err != nil {
		return "", fmt.Errorf("解析背景图片失败: %w", err)
	}

	optimized := resizeBackgroundImage(decoded, maxBackgroundWidth, maxBackgroundHeight)

	filename := fmt.Sprintf("dashboard-%d.webp", time.Now().UnixNano())
	fullPath := filepath.Join(backgroundDir, filename)
	output, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("创建背景图片失败: %w", err)
	}
	defer output.Close()

	if err := webp.Encode(output, optimized, &webp.Options{Lossless: false, Quality: backgroundWebPQuality}); err != nil {
		return "", fmt.Errorf("保存 WebP 背景图片失败: %w", err)
	}

	return runtimepath.PublicUploadPath(filepath.Join(runtimepath.BackgroundsDirName, filename)), nil
}

func decodeBackgroundImage(payload []byte, ext string) (image.Image, error) {
	reader := bytes.NewReader(payload)
	if ext == ".webp" {
		return webp.Decode(reader)
	}
	imageData, _, err := image.Decode(reader)
	return imageData, err
}

func resizeBackgroundImage(src image.Image, maxWidth, maxHeight int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}

	scale := minFloat(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
	if scale >= 1 {
		return src
	}

	targetWidth := maxInt(1, int(float64(width)*scale))
	targetHeight := maxInt(1, int(float64(height)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sanitizeBackgroundPath(uploadDir, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !runtimepath.IsBackgroundPublicPath(raw) {
		return ""
	}
	if runtimepath.LocalUploadPathFromPublic(uploadDir, raw) == "" {
		return ""
	}
	return raw
}

func removeBackgroundFile(uploadDir, publicPath string) error {
	localPath := runtimepath.LocalUploadPathFromPublic(uploadDir, publicPath)
	if localPath == "" {
		return nil
	}
	if !runtimepath.IsBackgroundPublicPath(publicPath) {
		return nil
	}
	if err := os.Remove(localPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func currentUserID(c *gin.Context) string {
	userID, _ := c.Get("current_user_id")
	if value, ok := userID.(string); ok {
		return value
	}
	return ""
}

func redirectWithPanelMessage(c *gin.Context, lang, success, failure string) {
	query := url.Values{}
	query.Set("panel", "settings")
	query.Set("lang", lang)
	if success != "" {
		query.Set("success", success)
	}
	if failure != "" {
		query.Set("error", failure)
	}
	c.Redirect(http.StatusFound, "/?"+query.Encode())
}

func translateAuthError(lang string, err error) string {
	switch {
	case errors.Is(err, service.ErrUsernameRequired):
		return i18n.T(lang, "settings.error.username_required")
	case errors.Is(err, service.ErrUsernameTaken):
		return i18n.T(lang, "settings.error.username_taken")
	case errors.Is(err, service.ErrCurrentPasswordRequired):
		return i18n.T(lang, "settings.error.current_password_required")
	case errors.Is(err, service.ErrPasswordTooShort):
		return i18n.T(lang, "settings.error.password_too_short")
	case errors.Is(err, service.ErrPasswordMismatch):
		return i18n.T(lang, "settings.error.password_mismatch")
	case errors.Is(err, service.ErrCurrentPasswordInvalid):
		return i18n.T(lang, "settings.error.current_password_invalid")
	default:
		return err.Error()
	}
}

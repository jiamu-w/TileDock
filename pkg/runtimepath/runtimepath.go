package runtimepath

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	// PublicUploadsPrefix is the public URL prefix for runtime uploads.
	PublicUploadsPrefix = "/static/uploads/"
	// BackgroundsDirName is the background asset subdirectory.
	BackgroundsDirName = "backgrounds"
	// IconsDirName is the icon asset subdirectory.
	IconsDirName = "icons"
	// ThumbnailsDirName is the website thumbnail asset subdirectory.
	ThumbnailsDirName = "thumbnails"
)

// BackgroundsDir returns the local directory for background uploads.
func BackgroundsDir(uploadDir string) string {
	return filepath.Join(uploadDir, BackgroundsDirName)
}

// IconsDir returns the local directory for icon uploads.
func IconsDir(uploadDir string) string {
	return filepath.Join(uploadDir, IconsDirName)
}

// ThumbnailsDir returns the local directory for website thumbnail uploads.
func ThumbnailsDir(uploadDir string) string {
	return filepath.Join(uploadDir, ThumbnailsDirName)
}

// IsBackgroundPublicPath reports whether the given public path points to a background upload.
func IsBackgroundPublicPath(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, PublicUploadsPrefix) {
		return false
	}

	relative := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(raw, PublicUploadsPrefix)))
	if relative == "." || relative == "" || strings.HasPrefix(relative, "..") {
		return false
	}

	return relative == BackgroundsDirName || strings.HasPrefix(relative, BackgroundsDirName+"/")
}

// IsIconPublicPath reports whether the given public path points to an icon upload.
func IsIconPublicPath(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, PublicUploadsPrefix) {
		return false
	}

	relative := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(raw, PublicUploadsPrefix)))
	if relative == "." || relative == "" || strings.HasPrefix(relative, "..") {
		return false
	}

	return relative == IconsDirName || strings.HasPrefix(relative, IconsDirName+"/")
}

// IsThumbnailPublicPath reports whether the given public path points to a thumbnail upload.
func IsThumbnailPublicPath(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, PublicUploadsPrefix) {
		return false
	}

	relative := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(raw, PublicUploadsPrefix)))
	if relative == "." || relative == "" || strings.HasPrefix(relative, "..") {
		return false
	}

	return relative == ThumbnailsDirName || strings.HasPrefix(relative, ThumbnailsDirName+"/")
}

// PublicUploadPath returns the public URL for a runtime upload.
func PublicUploadPath(relativePath string) string {
	cleaned := filepath.ToSlash(filepath.Clean(relativePath))
	cleaned = strings.TrimPrefix(cleaned, "/")
	return PublicUploadsPrefix + cleaned
}

// LocalUploadPathFromPublic converts a public upload URL to a local path.
func LocalUploadPathFromPublic(uploadDir, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, PublicUploadsPrefix) {
		return ""
	}

	relative := strings.TrimPrefix(raw, PublicUploadsPrefix)
	relative = filepath.Clean(relative)
	if relative == "." || relative == "" || strings.HasPrefix(relative, "..") {
		return ""
	}

	return filepath.Join(uploadDir, relative)
}

// RestorePathFromArchive returns the local path for a backup asset entry.
func RestorePathFromArchive(uploadDir, archivePath string) (string, error) {
	relative := strings.TrimPrefix(filepath.ToSlash(archivePath), "assets/")
	relative = filepath.Clean(relative)
	if relative == "." || relative == "" || strings.HasPrefix(relative, "..") {
		return "", fmt.Errorf("invalid asset path")
	}

	target := filepath.Join(uploadDir, relative)
	cleaned := filepath.Clean(target)
	prefix := filepath.ToSlash(filepath.Clean(uploadDir))
	full := filepath.ToSlash(cleaned)
	if full != prefix && !strings.HasPrefix(full, prefix+"/") {
		return "", fmt.Errorf("invalid restore target")
	}

	return cleaned, nil
}

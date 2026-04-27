package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"panel/internal/model"
	"panel/internal/repository"
	"panel/pkg/runtimepath"

	"gorm.io/gorm"
)

const (
	maxBackupArchiveSize int64 = 128 << 20
	maxBackupFileSize    int64 = 32 << 20
	maxBackupFiles             = 128
	maxBackupGroups            = 200
	maxBackupLinks             = 5000
)

var allowedRestoreSettingKeys = map[string]struct{}{
	dashboardBackgroundKey:      {},
	dashboardBlurKey:            {},
	dashboardOverlayOpacityKey:  {},
	dashboardTaglineKey:         {},
	dashboardDescriptionKey:     {},
	dashboardWeatherLocationKey: {},
	dashboardThumbnailBgKey:     {},
}

// BackupService creates downloadable backup archives.
type BackupService struct {
	groupRepo   *repository.NavGroupRepository
	settingRepo *repository.SettingRepository
	db          *gorm.DB
	dbPath      string
	backupDir   string
	uploadDir   string
}

// BackupArchive stores generated backup metadata.
type BackupArchive struct {
	FilePath string
	Name     string
}

type backupManifest struct {
	GeneratedAt         time.Time        `json:"generated_at"`
	DatabasePath        string           `json:"database_path"`
	IncludedFiles       []string         `json:"included_files"`
	DashboardBackground string           `json:"dashboard_background"`
	Groups              []model.NavGroup `json:"groups"`
	Settings            []model.Setting  `json:"settings"`
}

// NewBackupService creates a backup service.
func NewBackupService(
	db *gorm.DB,
	groupRepo *repository.NavGroupRepository,
	settingRepo *repository.SettingRepository,
	dbPath string,
	backupDir string,
	uploadDir string,
) *BackupService {
	return &BackupService{
		groupRepo:   groupRepo,
		settingRepo: settingRepo,
		db:          db,
		dbPath:      dbPath,
		backupDir:   backupDir,
		uploadDir:   uploadDir,
	}
}

// CreateBackup generates a zip archive and saves it on disk.
func (s *BackupService) CreateBackup(ctx context.Context) (*BackupArchive, error) {
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	name := fmt.Sprintf("panel-backup-%s.zip", time.Now().Format("20060102-150405"))
	filePath := filepath.Join(s.backupDir, name)
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("create backup archive: %w", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	if err := s.writeArchive(ctx, zipWriter); err != nil {
		_ = zipWriter.Close()
		return nil, err
	}
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close backup archive: %w", err)
	}

	return &BackupArchive{
		FilePath: filePath,
		Name:     name,
	}, nil
}

// RestoreBackup restores groups, links, settings and local assets from a zip archive.
func (s *BackupService) RestoreBackup(ctx context.Context, readerAt io.ReaderAt, size int64) error {
	if size <= 0 {
		return errors.New("backup archive is empty")
	}
	if size > maxBackupArchiveSize {
		return fmt.Errorf("backup archive exceeds %d MB", maxBackupArchiveSize>>20)
	}

	zipReader, err := zip.NewReader(readerAt, size)
	if err != nil {
		return fmt.Errorf("read backup archive: %w", err)
	}
	if err := validateBackupArchive(zipReader); err != nil {
		return err
	}

	manifest, err := readBackupManifest(zipReader)
	if err != nil {
		return err
	}
	if err := validateBackupManifest(manifest); err != nil {
		return err
	}

	if err := s.restoreAssets(zipReader); err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&model.NavLink{}).Error; err != nil {
			return fmt.Errorf("clear links: %w", err)
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&model.NavGroup{}).Error; err != nil {
			return fmt.Errorf("clear groups: %w", err)
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Setting{}).Error; err != nil {
			return fmt.Errorf("clear settings: %w", err)
		}

		for _, group := range manifest.Groups {
			links := group.NavLinks
			group.NavLinks = nil
			if err := tx.Create(&group).Error; err != nil {
				return fmt.Errorf("restore group %s: %w", group.Name, err)
			}
			for _, link := range links {
				if err := tx.Create(&link).Error; err != nil {
					return fmt.Errorf("restore link %s: %w", link.Title, err)
				}
			}
		}

		for _, setting := range manifest.Settings {
			if err := tx.Create(&setting).Error; err != nil {
				return fmt.Errorf("restore setting %s: %w", setting.Key, err)
			}
		}

		return nil
	})
}

func (s *BackupService) writeArchive(ctx context.Context, zipWriter *zip.Writer) error {
	groups, err := s.groupRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list groups for backup: %w", err)
	}
	settings, err := s.settingRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list settings for backup: %w", err)
	}

	backgroundValue := ""
	if setting, err := s.settingRepo.FindByKey(ctx, dashboardBackgroundKey); err != nil {
		return fmt.Errorf("load background setting for backup: %w", err)
	} else if setting != nil {
		backgroundValue = strings.TrimSpace(setting.Value)
	}

	includedFiles := make([]string, 0, 8)
	if err := s.addFile(zipWriter, s.dbPath, "database/"+filepath.Base(s.dbPath)); err != nil {
		return err
	}
	includedFiles = append(includedFiles, "database/"+filepath.Base(s.dbPath))

	assetPaths := collectBackupAssetPaths(s.uploadDir, backgroundValue, groups)
	for _, assetPath := range assetPaths {
		archivePath := "assets/" + strings.TrimPrefix(filepath.ToSlash(assetPath), filepath.ToSlash(s.uploadDir)+"/")
		if err := s.addFile(zipWriter, assetPath, archivePath); err != nil {
			return err
		}
		includedFiles = append(includedFiles, archivePath)
	}

	manifest := backupManifest{
		GeneratedAt:         time.Now(),
		DatabasePath:        s.dbPath,
		IncludedFiles:       includedFiles,
		DashboardBackground: backgroundValue,
		Groups:              groups,
		Settings:            settings,
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal backup manifest: %w", err)
	}
	if err := s.addBytes(zipWriter, "export/manifest.json", payload); err != nil {
		return err
	}

	return nil
}

func readBackupManifest(zipReader *zip.Reader) (*backupManifest, error) {
	for _, file := range zipReader.File {
		if filepath.ToSlash(file.Name) != "export/manifest.json" {
			continue
		}

		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open backup manifest: %w", err)
		}
		defer reader.Close()

		payload, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read backup manifest: %w", err)
		}

		var manifest backupManifest
		if err := json.Unmarshal(payload, &manifest); err != nil {
			return nil, fmt.Errorf("parse backup manifest: %w", err)
		}
		return &manifest, nil
	}

	return nil, fmt.Errorf("backup manifest not found")
}

func validateBackupArchive(zipReader *zip.Reader) error {
	if len(zipReader.File) > maxBackupFiles {
		return fmt.Errorf("backup archive contains too many files")
	}

	for _, file := range zipReader.File {
		if file.UncompressedSize64 > uint64(maxBackupFileSize) {
			return fmt.Errorf("backup file %s exceeds %d MB", file.Name, maxBackupFileSize>>20)
		}
	}

	return nil
}

func validateBackupManifest(manifest *backupManifest) error {
	if manifest == nil {
		return errors.New("backup manifest is missing")
	}
	if len(manifest.Groups) > maxBackupGroups {
		return fmt.Errorf("backup contains too many groups")
	}

	linkCount := 0
	for _, group := range manifest.Groups {
		if strings.TrimSpace(group.Name) == "" {
			return errors.New("backup contains a group without name")
		}
		linkCount += len(group.NavLinks)
	}
	if linkCount > maxBackupLinks {
		return fmt.Errorf("backup contains too many links")
	}

	for _, setting := range manifest.Settings {
		if _, ok := allowedRestoreSettingKeys[strings.TrimSpace(setting.Key)]; !ok {
			return fmt.Errorf("backup contains unsupported setting key %q", setting.Key)
		}
	}

	return nil
}

func collectBackupAssetPaths(uploadDir, backgroundValue string, groups []model.NavGroup) []string {
	seen := make(map[string]struct{})
	paths := make([]string, 0, 8)

	add := func(raw string) {
		path := runtimepath.LocalUploadPathFromPublic(uploadDir, raw)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	add(backgroundValue)
	for _, group := range groups {
		for _, link := range group.NavLinks {
			add(link.Icon)
			add(link.IconCachedPath)
			add(link.ThumbnailCachedPath)
		}
	}

	return paths
}

func (s *BackupService) restoreAssets(zipReader *zip.Reader) error {
	for _, file := range zipReader.File {
		name := filepath.ToSlash(file.Name)
		if !strings.HasPrefix(name, "assets/") || strings.HasSuffix(name, "/") {
			continue
		}

		targetPath, err := runtimepath.RestorePathFromArchive(s.uploadDir, name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create asset dir: %w", err)
		}

		reader, err := file.Open()
		if err != nil {
			return fmt.Errorf("open asset %s: %w", name, err)
		}

		payload, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil {
			return fmt.Errorf("read asset %s: %w", name, readErr)
		}

		if err := os.WriteFile(targetPath, payload, 0o644); err != nil {
			return fmt.Errorf("write asset %s: %w", targetPath, err)
		}
	}

	return nil
}

// NewReadSeekerBuffer converts bytes into a ReaderAt-backed buffer.
func NewReadSeekerBuffer(payload []byte) io.ReaderAt {
	return bytes.NewReader(payload)
}

func (s *BackupService) addFile(zipWriter *zip.Writer, sourcePath, archivePath string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open backup source %s: %w", sourcePath, err)
	}
	defer file.Close()

	writer, err := zipWriter.Create(filepath.ToSlash(archivePath))
	if err != nil {
		return fmt.Errorf("create archive entry %s: %w", archivePath, err)
	}
	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("write archive entry %s: %w", archivePath, err)
	}
	return nil
}

func (s *BackupService) addBytes(zipWriter *zip.Writer, archivePath string, payload []byte) error {
	writer, err := zipWriter.Create(filepath.ToSlash(archivePath))
	if err != nil {
		return fmt.Errorf("create archive entry %s: %w", archivePath, err)
	}
	if _, err := writer.Write(payload); err != nil {
		return fmt.Errorf("write archive entry %s: %w", archivePath, err)
	}
	return nil
}

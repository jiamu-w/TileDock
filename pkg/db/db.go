package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"panel/internal/model"
	"panel/internal/service"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// New opens a sqlite database.
func New(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// AutoMigrate creates required tables.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&model.User{}, &model.NavGroup{}, &model.NavLink{}, &model.Setting{})
}

// SeedAdmin initializes the default admin account.
func SeedAdmin(db *gorm.DB, username, password string) error {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return errors.New("bootstrap admin credentials are required for first startup")
	}
	if username == "admin" && password == "admin123" {
		return errors.New("bootstrap admin credentials must not use the insecure default admin/admin123")
	}

	hash, err := service.HashPassword(password)
	if err != nil {
		return err
	}

	return db.Create(&model.User{
		Username:     username,
		PasswordHash: hash,
		Role:         "admin",
	}).Error
}

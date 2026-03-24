package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultConfigPath = "config/config.yaml"

const (
	placeholderSessionSecret = "change-me-in-production"
	minSessionSecretLength   = 32
)

// Config holds application settings.
type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Session  SessionConfig  `yaml:"session"`
	Auth     AuthConfig     `yaml:"auth"`
	Log      LogConfig      `yaml:"log"`
}

// AppConfig stores app-level settings.
type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"`
}

// ServerConfig stores http server settings.
type ServerConfig struct {
	Addr string `yaml:"addr"`
}

// DatabaseConfig stores sqlite settings.
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// StorageConfig stores writable runtime paths.
type StorageConfig struct {
	UploadDir string `yaml:"upload_dir"`
	BackupDir string `yaml:"backup_dir"`
}

// SessionConfig stores session settings.
type SessionConfig struct {
	Name     string `yaml:"name"`
	Secret   string `yaml:"secret"`
	MaxAge   int    `yaml:"max_age"`
	Secure   bool   `yaml:"secure"`
	HTTPOnly bool   `yaml:"http_only"`
}

// AuthConfig stores default admin bootstrap config.
type AuthConfig struct {
	DefaultAdminUser     string `yaml:"default_admin_user"`
	DefaultAdminPassword string `yaml:"default_admin_password"`
}

// LogConfig stores logger settings.
type LogConfig struct {
	Level string `yaml:"level"`
}

// Load reads config from yaml and env.
func Load() (*Config, error) {
	cfg := defaultConfig()
	path := getenv("PANEL_CONFIG", defaultConfigPath)

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config file: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	overrideFromEnv(cfg)

	if err := validateSecurity(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Name: "TileDock",
			Env:  "development",
		},
		Server: ServerConfig{
			Addr: ":8080",
		},
		Database: DatabaseConfig{
			Path: "data/panel.db",
		},
		Storage: StorageConfig{
			UploadDir: "data/uploads",
			BackupDir: "data/backups",
		},
		Session: SessionConfig{
			Name:     "panel_session",
			Secret:   "",
			MaxAge:   86400 * 7,
			Secure:   false,
			HTTPOnly: true,
		},
		Auth: AuthConfig{
			DefaultAdminUser:     "",
			DefaultAdminPassword: "",
		},
		Log: LogConfig{
			Level: "info",
		},
	}
}

func overrideFromEnv(cfg *Config) {
	cfg.App.Env = getenv("PANEL_APP_ENV", cfg.App.Env)
	cfg.Server.Addr = getenv("PANEL_SERVER_ADDR", cfg.Server.Addr)
	cfg.Database.Path = getenv("PANEL_DB_PATH", cfg.Database.Path)
	cfg.Storage.UploadDir = getenv("PANEL_UPLOAD_DIR", cfg.Storage.UploadDir)
	cfg.Storage.BackupDir = getenv("PANEL_BACKUP_DIR", cfg.Storage.BackupDir)
	cfg.Session.Name = getenv("PANEL_SESSION_NAME", cfg.Session.Name)
	cfg.Session.Secret = getenv("PANEL_SESSION_SECRET", cfg.Session.Secret)
	cfg.Auth.DefaultAdminUser = getenv("PANEL_DEFAULT_ADMIN_USER", cfg.Auth.DefaultAdminUser)
	cfg.Auth.DefaultAdminPassword = getenv("PANEL_DEFAULT_ADMIN_PASSWORD", cfg.Auth.DefaultAdminPassword)
	cfg.Log.Level = getenv("PANEL_LOG_LEVEL", cfg.Log.Level)

	if value := os.Getenv("PANEL_SESSION_MAX_AGE"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			cfg.Session.MaxAge = parsed
		}
	}

	if value := os.Getenv("PANEL_SESSION_SECURE"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.Session.Secure = parsed
		}
	}

	if value := os.Getenv("PANEL_SESSION_HTTP_ONLY"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.Session.HTTPOnly = parsed
		}
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func validateSecurity(cfg *Config) error {
	secret := strings.TrimSpace(cfg.Session.Secret)
	isProduction := strings.EqualFold(strings.TrimSpace(cfg.App.Env), "production")

	if !isProduction && (secret == "" || secret == placeholderSessionSecret) {
		generated, err := generateSessionSecret()
		if err != nil {
			return fmt.Errorf("generate development session secret: %w", err)
		}
		cfg.Session.Secret = generated
		secret = generated
	}

	if secret == "" {
		return errors.New("session.secret is required")
	}
	if secret == placeholderSessionSecret {
		return errors.New("session.secret must not use the insecure placeholder value")
	}
	if len(secret) < minSessionSecretLength {
		return fmt.Errorf("session.secret must be at least %d characters", minSessionSecretLength)
	}
	if isProduction && !cfg.Session.Secure {
		return errors.New("session.secure must be true in production")
	}
	return nil
}

func generateSessionSecret() (string, error) {
	payload := make([]byte, 32)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return hex.EncodeToString(payload), nil
}

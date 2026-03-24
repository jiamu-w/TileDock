package config

import "testing"

func TestValidateSecurityGeneratesDevelopmentSecret(t *testing.T) {
	cfg := defaultConfig()
	cfg.Session.Secret = placeholderSessionSecret

	if err := validateSecurity(cfg); err != nil {
		t.Fatalf("expected development secret to be generated, got error: %v", err)
	}
	if cfg.Session.Secret == "" || cfg.Session.Secret == placeholderSessionSecret {
		t.Fatal("expected generated development session secret")
	}
}

func TestValidateSecurityRejectsInsecureProductionCookie(t *testing.T) {
	cfg := defaultConfig()
	cfg.App.Env = "production"
	cfg.Session.Secret = "01234567890123456789012345678901"
	cfg.Session.Secure = false

	if err := validateSecurity(cfg); err == nil {
		t.Fatal("expected insecure production cookie config to be rejected")
	}
}

func TestValidateSecurityRejectsWeakProductionSecret(t *testing.T) {
	cfg := defaultConfig()
	cfg.App.Env = "production"
	cfg.Session.Secret = placeholderSessionSecret
	cfg.Session.Secure = true

	if err := validateSecurity(cfg); err == nil {
		t.Fatal("expected weak production secret to be rejected")
	}
}

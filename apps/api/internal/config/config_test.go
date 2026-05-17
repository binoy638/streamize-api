package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("STREAMIZE_ENV", "")
	t.Setenv("STREAMIZE_HTTP_ADDR", "")
	t.Setenv("STREAMIZE_DATABASE_PATH", "")
	t.Setenv("STREAMIZE_MEDIA_ROOT", "")
	t.Setenv("STREAMIZE_QBITTORRENT_URL", "")
	t.Setenv("STREAMIZE_ADMIN_USERNAME", "")
	t.Setenv("STREAMIZE_ADMIN_PASSWORD", "")
	t.Setenv("STREAMIZE_SESSION_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Environment != EnvironmentDevelopment {
		t.Fatalf("expected default environment %q, got %q", EnvironmentDevelopment, cfg.Environment)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("expected default HTTP addr, got %q", cfg.HTTPAddr)
	}
	if cfg.DatabasePath == "" {
		t.Fatal("expected default database path")
	}
	if cfg.MediaRoot == "" {
		t.Fatal("expected default media root")
	}
	if cfg.AdminUsername != "admin" {
		t.Fatalf("expected default admin username, got %q", cfg.AdminUsername)
	}
	if cfg.AdminPassword == "" {
		t.Fatal("expected default development admin password")
	}
	if cfg.SessionTTL <= 0 {
		t.Fatal("expected default session ttl")
	}
}

func TestLoadProductionRequiresAdminPassword(t *testing.T) {
	t.Setenv("STREAMIZE_ENV", EnvironmentProduction)
	t.Setenv("STREAMIZE_ADMIN_PASSWORD", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected production config without admin password to fail")
	}
}

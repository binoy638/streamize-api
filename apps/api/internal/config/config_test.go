package config

import (
	"log/slog"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("STREAMIZE_ENV", "")
	t.Setenv("STREAMIZE_HTTP_ADDR", "")
	t.Setenv("STREAMIZE_DATABASE_PATH", "")
	t.Setenv("STREAMIZE_MEDIA_ROOT", "")
	t.Setenv("STREAMIZE_QBITTORRENT_URL", "")
	t.Setenv("STREAMIZE_ADMIN_USERNAME", "")
	t.Setenv("STREAMIZE_ADMIN_PASSWORD", "")
	t.Setenv("STREAMIZE_SESSION_TTL", "")
	t.Setenv("STREAMIZE_LOG_LEVEL", "")
	t.Setenv("STREAMIZE_LOG_FORMAT", "")
	t.Setenv("STREAMIZE_WORKER_ENABLED", "")
	t.Setenv("STREAMIZE_WORKER_POLL_INTERVAL", "")
	t.Setenv("STREAMIZE_FFMPEG_PATH", "")

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
	if cfg.LogLevel != slog.LevelDebug {
		t.Fatalf("expected default development log level debug, got %v", cfg.LogLevel)
	}
	if cfg.LogFormat != LogFormatText {
		t.Fatalf("expected default development log format %q, got %q", LogFormatText, cfg.LogFormat)
	}
	if !cfg.WorkerEnabled {
		t.Fatal("expected worker to be enabled by default")
	}
	if cfg.WorkerPollInterval <= 0 {
		t.Fatal("expected default worker poll interval")
	}
	if cfg.FFmpegPath != "ffmpeg" {
		t.Fatalf("expected default ffmpeg path, got %q", cfg.FFmpegPath)
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

func TestLoadProductionDefaultsToJSONLogs(t *testing.T) {
	t.Setenv("STREAMIZE_ENV", EnvironmentProduction)
	t.Setenv("STREAMIZE_ADMIN_PASSWORD", "secure-password")
	t.Setenv("STREAMIZE_LOG_LEVEL", "")
	t.Setenv("STREAMIZE_LOG_FORMAT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("expected default production log level info, got %v", cfg.LogLevel)
	}
	if cfg.LogFormat != LogFormatJSON {
		t.Fatalf("expected default production log format %q, got %q", LogFormatJSON, cfg.LogFormat)
	}
}

func TestLoadAcceptsTextLogFormat(t *testing.T) {
	t.Setenv("STREAMIZE_LOG_FORMAT", "text")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.LogFormat != LogFormatText {
		t.Fatalf("expected log format %q, got %q", LogFormatText, cfg.LogFormat)
	}
}

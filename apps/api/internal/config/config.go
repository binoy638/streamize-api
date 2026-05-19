package config

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"

	LogFormatJSON = "json"
	LogFormatText = "text"
)

type Config struct {
	Environment         string
	HTTPAddr            string
	DatabasePath        string
	MediaRoot           string
	OriginalsDir        string
	HLSDir              string
	SubtitlesDir        string
	ThumbnailsDir       string
	TempDir             string
	QBittorrentURL      string
	QBittorrentUsername string
	QBittorrentPassword string
	SessionCookieName   string
	SessionTTL          time.Duration
	AdminUsername       string
	AdminPassword       string
	AdminStorageQuota   int64
	SecureCookies       bool
	TrustedProxyHeaders bool
	LogLevel            slog.Level
	LogFormat           string
	WorkerEnabled       bool
	WorkerPollInterval  time.Duration
	FFmpegPath          string
	FFprobePath         string
}

func Load() (Config, error) {
	environment := envString("STREAMIZE_ENV", EnvironmentDevelopment)
	mediaRoot := envString("STREAMIZE_MEDIA_ROOT", "./media")
	adminPasswordDefault := "adminadmin"
	logLevelDefault := slog.LevelDebug
	logFormatDefault := LogFormatText
	if environment == EnvironmentProduction {
		adminPasswordDefault = ""
		logLevelDefault = slog.LevelInfo
		logFormatDefault = LogFormatJSON
	}

	cfg := Config{
		Environment:         environment,
		HTTPAddr:            envString("STREAMIZE_HTTP_ADDR", ":8080"),
		DatabasePath:        envString("STREAMIZE_DATABASE_PATH", "./data/streamize.db"),
		MediaRoot:           mediaRoot,
		OriginalsDir:        envString("STREAMIZE_ORIGINALS_DIR", filepath.Join(mediaRoot, "originals")),
		HLSDir:              envString("STREAMIZE_HLS_DIR", filepath.Join(mediaRoot, "hls")),
		SubtitlesDir:        envString("STREAMIZE_SUBTITLES_DIR", filepath.Join(mediaRoot, "subtitles")),
		ThumbnailsDir:       envString("STREAMIZE_THUMBNAILS_DIR", filepath.Join(mediaRoot, "thumbnails")),
		TempDir:             envString("STREAMIZE_TEMP_DIR", filepath.Join(mediaRoot, "tmp")),
		QBittorrentURL:      strings.TrimRight(envString("STREAMIZE_QBITTORRENT_URL", "http://localhost:8081"), "/"),
		QBittorrentUsername: envString("STREAMIZE_QBITTORRENT_USERNAME", "admin"),
		QBittorrentPassword: envString("STREAMIZE_QBITTORRENT_PASSWORD", "adminadmin"),
		SessionCookieName:   envString("STREAMIZE_SESSION_COOKIE_NAME", "streamize_session"),
		SessionTTL:          envDuration("STREAMIZE_SESSION_TTL", 7*24*time.Hour),
		AdminUsername:       envString("STREAMIZE_ADMIN_USERNAME", "admin"),
		AdminPassword:       envString("STREAMIZE_ADMIN_PASSWORD", adminPasswordDefault),
		AdminStorageQuota:   envInt64("STREAMIZE_ADMIN_STORAGE_QUOTA_BYTES", 0),
		SecureCookies:       envBool("STREAMIZE_SECURE_COOKIES", false),
		TrustedProxyHeaders: envBool("STREAMIZE_TRUSTED_PROXY_HEADERS", false),
		LogLevel:            envLogLevel("STREAMIZE_LOG_LEVEL", logLevelDefault),
		LogFormat:           envLogFormat("STREAMIZE_LOG_FORMAT", logFormatDefault),
		WorkerEnabled:       envBool("STREAMIZE_WORKER_ENABLED", true),
		WorkerPollInterval:  envDuration("STREAMIZE_WORKER_POLL_INTERVAL", 5*time.Second),
		FFmpegPath:          envString("STREAMIZE_FFMPEG_PATH", "ffmpeg"),
		FFprobePath:         envString("STREAMIZE_FFPROBE_PATH", "ffprobe"),
	}

	if cfg.Environment == "" {
		return Config{}, errors.New("STREAMIZE_ENV cannot be empty")
	}
	if cfg.HTTPAddr == "" {
		return Config{}, errors.New("STREAMIZE_HTTP_ADDR cannot be empty")
	}
	if cfg.DatabasePath == "" {
		return Config{}, errors.New("STREAMIZE_DATABASE_PATH cannot be empty")
	}
	if cfg.MediaRoot == "" {
		return Config{}, errors.New("STREAMIZE_MEDIA_ROOT cannot be empty")
	}
	if cfg.QBittorrentURL == "" {
		return Config{}, errors.New("STREAMIZE_QBITTORRENT_URL cannot be empty")
	}
	if cfg.SessionCookieName == "" {
		return Config{}, errors.New("STREAMIZE_SESSION_COOKIE_NAME cannot be empty")
	}
	if cfg.SessionTTL <= 0 {
		return Config{}, errors.New("STREAMIZE_SESSION_TTL must be greater than zero")
	}
	if cfg.AdminUsername == "" {
		return Config{}, errors.New("STREAMIZE_ADMIN_USERNAME cannot be empty")
	}
	if cfg.AdminPassword == "" {
		return Config{}, errors.New("STREAMIZE_ADMIN_PASSWORD cannot be empty")
	}
	if len(cfg.AdminPassword) < 8 {
		return Config{}, errors.New("STREAMIZE_ADMIN_PASSWORD must be at least 8 characters")
	}
	if cfg.AdminStorageQuota < 0 {
		return Config{}, errors.New("STREAMIZE_ADMIN_STORAGE_QUOTA_BYTES cannot be negative")
	}
	if cfg.WorkerPollInterval <= 0 {
		return Config{}, errors.New("STREAMIZE_WORKER_POLL_INTERVAL must be greater than zero")
	}
	if cfg.FFmpegPath == "" {
		return Config{}, errors.New("STREAMIZE_FFMPEG_PATH cannot be empty")
	}
	if cfg.FFprobePath == "" {
		return Config{}, errors.New("STREAMIZE_FFPROBE_PATH cannot be empty")
	}

	return cfg, nil
}

func (c Config) EnsureRuntimeDirs() error {
	dirs := []string{
		filepath.Dir(c.DatabasePath),
		c.MediaRoot,
		c.OriginalsDir,
		c.HLSDir,
		c.SubtitlesDir,
		c.ThumbnailsDir,
		c.TempDir,
	}

	for _, dir := range dirs {
		if dir == "." || dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envLogLevel(key string, fallback slog.Level) slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "":
		return fallback
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return fallback
	}
}

func envLogFormat(key string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "":
		return fallback
	case LogFormatJSON:
		return LogFormatJSON
	case LogFormatText:
		return LogFormatText
	default:
		return fallback
	}
}

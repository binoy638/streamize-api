package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/catalog"
	"github.com/binoy638/streamize-api/apps/api/internal/config"
	"github.com/binoy638/streamize-api/apps/api/internal/database"
	"github.com/binoy638/streamize-api/apps/api/internal/httpserver"
	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/metadata"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
	"github.com/binoy638/streamize-api/apps/api/internal/transcoding"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logOptions := &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}
	logHandler := slog.Handler(slog.NewJSONHandler(os.Stdout, logOptions))
	if cfg.LogFormat == config.LogFormatText {
		logHandler = slog.NewTextHandler(os.Stdout, logOptions)
	}

	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	if err := cfg.EnsureRuntimeDirs(); err != nil {
		logger.Error("failed to create runtime directories", "error", err)
		os.Exit(1)
	}

	db, err := database.Open(ctx, cfg.DatabasePath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.Migrate(ctx, db); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	authStore := auth.NewStore(db)
	if err := authStore.BootstrapAdmin(ctx, cfg); err != nil {
		logger.Error("failed to bootstrap admin user", "error", err)
		os.Exit(1)
	}

	jobStore := jobs.NewStore(db)
	if created, err := jobStore.BackfillMetadataIdentifyJobs(ctx, 1000); err != nil {
		logger.Warn("failed to queue metadata backfill jobs", "error", err)
	} else if created > 0 {
		logger.Info("metadata backfill jobs queued", "count", created)
	}

	if cfg.WorkerEnabled {
		worker := transcoding.Worker{
			Jobs:          jobStore,
			Torrents:      torrents.NewStore(db),
			Catalog:       catalog.NewStore(db),
			Metadata:      metadata.Resolver{TMDBAPIKey: cfg.TMDBAPIKey, Language: cfg.MetadataLanguage},
			Prober:        transcoding.FFprobeProber{Binary: cfg.FFprobePath},
			Transcoder:    transcoding.FFmpegTranscoder{Binary: cfg.FFmpegPath},
			Assets:        transcoding.FFmpegAssetProcessor{Binary: cfg.FFmpegPath},
			HLSDir:        cfg.HLSDir,
			SubtitlesDir:  cfg.SubtitlesDir,
			ThumbnailsDir: cfg.ThumbnailsDir,
			PollInterval:  cfg.WorkerPollInterval,
			Logger:        logger,
		}
		go worker.Run(ctx)
	} else {
		logger.Info("media worker disabled")
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpserver.NewRouter(cfg, db, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Environment, "log_level", cfg.LogLevel.String(), "log_format", cfg.LogFormat)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("api stopped")
}

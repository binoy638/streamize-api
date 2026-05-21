package httpserver

import (
	"bufio"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/config"
	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/qbittorrent"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
	"github.com/binoy638/streamize-api/apps/api/internal/watchparty"
	"github.com/binoy638/streamize-api/apps/api/internal/webui"
)

type RouterOption func(*routerOptions)

type routerOptions struct {
	torrentAdder      TorrentAdder
	torrentLister     TorrentLister
	torrentFileLister TorrentFileLister
	torrentDeleter    TorrentDeleter
	torrentResumer    TorrentResumer
	freeDiskBytes     FreeDiskBytesFunc
}

func WithTorrentAdder(adder TorrentAdder) RouterOption {
	return func(options *routerOptions) {
		options.torrentAdder = adder
	}
}

func WithTorrentLister(lister TorrentLister) RouterOption {
	return func(options *routerOptions) {
		options.torrentLister = lister
	}
}

func WithTorrentFileLister(lister TorrentFileLister) RouterOption {
	return func(options *routerOptions) {
		options.torrentFileLister = lister
	}
}

func WithTorrentDeleter(deleter TorrentDeleter) RouterOption {
	return func(options *routerOptions) {
		options.torrentDeleter = deleter
	}
}

func WithTorrentResumer(resumer TorrentResumer) RouterOption {
	return func(options *routerOptions) {
		options.torrentResumer = resumer
	}
}

func WithFreeDiskBytes(fn FreeDiskBytesFunc) RouterOption {
	return func(options *routerOptions) {
		options.freeDiskBytes = fn
	}
}

func NewRouter(cfg config.Config, db *sql.DB, logger *slog.Logger, optionFns ...RouterOption) http.Handler {
	var options routerOptions
	if cfg.QBittorrentURL != "" {
		qbtClient := qbittorrent.NewClient(cfg.QBittorrentURL, cfg.QBittorrentUsername, cfg.QBittorrentPassword)
		options.torrentAdder = qbtClient
		options.torrentLister = qbtClient
		options.torrentFileLister = qbtClient
		options.torrentDeleter = qbtClient
		options.torrentResumer = qbtClient
	}
	for _, optionFn := range optionFns {
		optionFn(&options)
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(requestLogger(logger))
	// Non-/api routes fall through to the embedded SPA; /api/* keeps its own
	// JSON 404 via the subrouter's api.NotFound below.
	router.NotFound(webui.Handler().ServeHTTP)
	router.MethodNotAllowed(methodNotAllowedHandler)

	authStore := auth.NewStore(db)
	torrentStore := torrents.NewStore(db)
	jobStore := jobs.NewStore(db)
	watchPartyStore := watchparty.NewStore(db)
	watchPartyHub := watchparty.NewHub(watchPartyStore, logger)
	authHandler := AuthHandler{
		Config: cfg,
		Store:  authStore,
	}
	adminHandler := AdminHandler{
		Store: authStore,
	}
	healthHandler := HealthHandler{
		Config: cfg,
		DB:     db,
	}
	torrentHandler := TorrentHandler{
		Store:         torrentStore,
		JobStore:      jobStore,
		Adder:         options.torrentAdder,
		Lister:        options.torrentLister,
		FileLister:    options.torrentFileLister,
		Deleter:       options.torrentDeleter,
		Resumer:       options.torrentResumer,
		SavePath:      cfg.OriginalsDir,
		HLSDir:        cfg.HLSDir,
		SubtitlesDir:  cfg.SubtitlesDir,
		ThumbnailsDir: cfg.ThumbnailsDir,
		Logger:        logger,
		FreeDiskBytes: options.freeDiskBytes,
	}
	jobHandler := JobHandler{
		Store:        jobStore,
		TorrentStore: torrentStore,
	}
	playbackHandler := PlaybackHandler{
		Store:         torrentStore,
		OriginalsDir:  cfg.OriginalsDir,
		HLSDir:        cfg.HLSDir,
		SubtitlesDir:  cfg.SubtitlesDir,
		ThumbnailsDir: cfg.ThumbnailsDir,
	}
	watchPartyHandler := WatchPartyHandler{
		Store:        watchPartyStore,
		TorrentStore: torrentStore,
		AuthStore:    authStore,
		Config:       cfg,
		Playback:     playbackHandler,
		Hub:          watchPartyHub,
	}

	router.Route("/api", func(api chi.Router) {
		api.NotFound(notFoundHandler)
		api.MethodNotAllowed(methodNotAllowedHandler)

		api.Get("/health", healthHandler.ServeHTTP)
		api.Post("/auth/sign-in", authHandler.SignIn)
		api.Post("/auth/sign-out", authHandler.SignOut)
		api.Get("/watch-parties/join/{slug}", watchPartyHandler.PublicMetadata)
		api.Post("/watch-parties/join/{slug}", watchPartyHandler.Join)
		api.Get("/watch-parties/join/{slug}/ws", watchPartyHandler.WebSocket)
		api.Get("/watch-parties/join/{slug}/files/{id}/original", watchPartyHandler.ServeOriginalFile)
		api.Get("/watch-parties/join/{slug}/files/{id}/hls/index.m3u8", watchPartyHandler.ServeHLSPlaylist)
		api.Get("/watch-parties/join/{slug}/files/{id}/hls/{segment}", watchPartyHandler.ServeHLSSegment)
		api.Get("/watch-parties/join/{slug}/files/{id}/subtitles", watchPartyHandler.ListSubtitles)
		api.Get("/watch-parties/join/{slug}/files/{id}/subtitles/{subtitleID}/track.vtt", watchPartyHandler.ServeSubtitleTrack)
		api.Get("/watch-parties/join/{slug}/files/{id}/preview/thumbnails.vtt", watchPartyHandler.ServePreviewVTT)
		api.Get("/watch-parties/join/{slug}/files/{id}/preview/{asset}", watchPartyHandler.ServePreviewAsset)

		api.Group(func(protected chi.Router) {
			protected.Use(RequireUser(cfg, authStore))
			protected.Get("/auth/me", authHandler.Me)
			protected.Get("/torrents", torrentHandler.ListTorrents)
			protected.Post("/torrents", torrentHandler.CreateTorrent)
			protected.Get("/torrents/{id}/files", torrentHandler.ListTorrentFiles)
			protected.Delete("/torrents/{id}", torrentHandler.DeleteTorrent)
			protected.Get("/files", torrentHandler.ListFiles)
			protected.Get("/jobs", jobHandler.ListJobs)
			protected.Post("/jobs/{id}/retry", jobHandler.RetryJob)
			protected.Post("/jobs/{id}/cancel", jobHandler.CancelJob)
			protected.Get("/watch-parties", watchPartyHandler.List)
			protected.Post("/watch-parties", watchPartyHandler.Create)
			protected.Post("/watch-parties/{id}/end", watchPartyHandler.End)
			protected.Get("/files/{id}/original", playbackHandler.ServeOriginalFile)
			protected.Get("/files/{id}/hls/index.m3u8", playbackHandler.ServeHLSPlaylist)
			protected.Get("/files/{id}/hls/{segment}", playbackHandler.ServeHLSSegment)
			protected.Get("/files/{id}/subtitles", playbackHandler.ListSubtitles)
			protected.Get("/files/{id}/preview/thumbnails.vtt", playbackHandler.ServePreviewVTT)
			protected.Get("/files/{id}/preview/{asset}", playbackHandler.ServePreviewAsset)
			protected.Get("/subtitles/{id}/track.vtt", playbackHandler.ServeSubtitleTrack)

			protected.Route("/admin", func(admin chi.Router) {
				admin.Use(RequireAdmin)
				admin.Get("/users", adminHandler.ListUsers)
				admin.Post("/users", adminHandler.CreateUser)
			})
		})
	})

	return router
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := &statusRecorder{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(recorder, r)

			level := slog.LevelInfo
			if recorder.status >= http.StatusInternalServerError {
				level = slog.LevelError
			} else if recorder.status >= http.StatusBadRequest {
				level = slog.LevelWarn
			}

			logger.LogAttrs(r.Context(), level, "request completed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", recorder.status),
				slog.String("status_text", http.StatusText(recorder.status)),
				slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
				slog.Int("response_bytes", recorder.bytesWritten),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	written, err := r.ResponseWriter.Write(body)
	r.bytesWritten += written
	return written, err
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "API route not found")
}

func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed for API route")
}

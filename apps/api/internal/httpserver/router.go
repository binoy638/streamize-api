package httpserver

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/config"
)

func NewRouter(cfg config.Config, db *sql.DB, logger *slog.Logger) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(requestLogger(logger))

	authStore := auth.NewStore(db)
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

	router.Route("/api", func(api chi.Router) {
		api.Get("/health", healthHandler.ServeHTTP)
		api.Post("/auth/sign-in", authHandler.SignIn)
		api.Post("/auth/sign-out", authHandler.SignOut)

		api.Group(func(protected chi.Router) {
			protected.Use(RequireUser(cfg, authStore))
			protected.Get("/auth/me", authHandler.Me)

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
			next.ServeHTTP(w, r)
			logger.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

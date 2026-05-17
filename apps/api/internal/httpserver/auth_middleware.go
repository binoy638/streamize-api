package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/config"
)

type userContextKey struct{}

func RequireUser(cfg config.Config, store *auth.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cfg.SessionCookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			user, err := store.FindUserBySessionToken(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, auth.ErrUnauthorized) {
					clearSessionCookie(w, cfg)
					writeError(w, http.StatusUnauthorized, "authentication required")
					return
				}
				writeError(w, http.StatusInternalServerError, "failed to load session")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := CurrentUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if user.Role != auth.RoleAdmin {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func CurrentUser(r *http.Request) (auth.User, bool) {
	user, ok := r.Context().Value(userContextKey{}).(auth.User)
	return user, ok
}

func setSessionCookie(w http.ResponseWriter, cfg config.Config, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(cfg.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

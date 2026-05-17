package httpserver

import (
	"errors"
	"net/http"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/config"
)

type AuthHandler struct {
	Config config.Config
	Store  *auth.Store
}

type signInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authUserResponse struct {
	User auth.User `json:"user"`
}

func (h AuthHandler) SignIn(w http.ResponseWriter, r *http.Request) {
	var request signInRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	user, err := h.Store.Authenticate(r.Context(), request.Username, request.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to sign in")
		return
	}

	token, err := h.Store.CreateSession(r.Context(), user.ID, h.Config.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	setSessionCookie(w, h.Config, token)
	writeJSON(w, http.StatusOK, authUserResponse{User: user})
}

func (h AuthHandler) SignOut(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(h.Config.SessionCookieName); err == nil {
		if err := h.Store.DeleteSessionByToken(r.Context(), cookie.Value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to sign out")
			return
		}
	}

	clearSessionCookie(w, h.Config)
	w.WriteHeader(http.StatusNoContent)
}

func (h AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	writeJSON(w, http.StatusOK, authUserResponse{User: user})
}

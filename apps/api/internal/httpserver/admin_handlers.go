package httpserver

import (
	"errors"
	"net/http"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
)

type AdminHandler struct {
	Store *auth.Store
}

type createUserRequest struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	Role              string `json:"role"`
	StorageQuotaBytes int64  `json:"storageQuotaBytes"`
}

type usersResponse struct {
	Users []auth.User `json:"users"`
}

func (h AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	writeJSON(w, http.StatusOK, usersResponse{Users: users})
}

func (h AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	user, err := h.Store.CreateUser(r.Context(), auth.CreateUserParams{
		Username:          request.Username,
		Password:          request.Password,
		Role:              request.Role,
		StorageQuotaBytes: request.StorageQuotaBytes,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrConflict):
			writeError(w, http.StatusConflict, "username already exists")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, authUserResponse{User: user})
}

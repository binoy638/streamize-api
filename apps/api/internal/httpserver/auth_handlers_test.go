package httpserver

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/config"
	"github.com/binoy638/streamize-api/apps/api/internal/database"
)

func TestAuthAndAdminRoutes(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default())

	signInBody := `{"username":"admin","password":"admin-password"}`
	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", signInBody, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}

	cookie := signInResponse.Result().Cookies()[0]
	if cookie.Name != cfg.SessionCookieName {
		t.Fatalf("expected cookie %q, got %q", cfg.SessionCookieName, cookie.Name)
	}
	if cookie.HttpOnly != true {
		t.Fatal("expected session cookie to be HTTP-only")
	}

	meResponse := performJSONRequest(router, http.MethodGet, "/api/auth/me", "", cookie)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("expected me status %d, got %d: %s", http.StatusOK, meResponse.Code, meResponse.Body.String())
	}

	createUserResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/admin/users",
		`{"username":"viewer","password":"viewer-password","storageQuotaBytes":1000}`,
		cookie,
	)
	if createUserResponse.Code != http.StatusCreated {
		t.Fatalf("expected create user status %d, got %d: %s", http.StatusCreated, createUserResponse.Code, createUserResponse.Body.String())
	}

	listUsersResponse := performJSONRequest(router, http.MethodGet, "/api/admin/users", "", cookie)
	if listUsersResponse.Code != http.StatusOK {
		t.Fatalf("expected list users status %d, got %d: %s", http.StatusOK, listUsersResponse.Code, listUsersResponse.Body.String())
	}

	var users usersResponse
	if err := json.Unmarshal(listUsersResponse.Body.Bytes(), &users); err != nil {
		t.Fatalf("list users response is not valid JSON: %v", err)
	}
	if len(users.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users.Users))
	}
}

func TestAdminRoutesRequireAdmin(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	if _, err := store.CreateUser(ctx, auth.CreateUserParams{
		Username: "viewer",
		Password: "viewer-password",
		Role:     auth.RoleUser,
	}); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default())

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"viewer","password":"viewer-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}

	cookie := signInResponse.Result().Cookies()[0]
	adminResponse := performJSONRequest(router, http.MethodGet, "/api/admin/users", "", cookie)
	if adminResponse.Code != http.StatusForbidden {
		t.Fatalf("expected admin route status %d, got %d: %s", http.StatusForbidden, adminResponse.Code, adminResponse.Body.String())
	}
}

func TestProtectedRoutesRequireSession(t *testing.T) {
	cfg := testConfig()
	db := testDB(t)
	router := NewRouter(cfg, db, slog.Default())

	response := performJSONRequest(router, http.MethodGet, "/api/auth/me", "", nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func testConfig() config.Config {
	return config.Config{
		Environment:       config.EnvironmentDevelopment,
		SessionCookieName: "streamize_test_session",
		SessionTTL:        time.Hour,
		AdminUsername:     "admin",
		AdminPassword:     "admin-password",
		AdminStorageQuota: 0,
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "streamize.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	return db
}

func performJSONRequest(handler http.Handler, method string, path string, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	var requestBody *bytes.Reader
	if body == "" {
		requestBody = bytes.NewReader(nil)
	} else {
		requestBody = bytes.NewReader([]byte(body))
	}

	req := httptest.NewRequest(method, path, requestBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

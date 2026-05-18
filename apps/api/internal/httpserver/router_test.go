package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/config"
	"github.com/binoy638/streamize-api/apps/api/internal/database"
)

func TestUnknownAPIRouteReturnsJSONErrorAndLogsStatus(t *testing.T) {
	var logs bytes.Buffer
	router := newTestRouterWithLogger(t, slog.New(slog.NewJSONHandler(&logs, nil)))

	response := performJSONRequest(router, http.MethodDelete, "/api/does-not-exist", "", nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, response.Code, response.Body.String())
	}

	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.Error != "API route not found" {
		t.Fatalf("expected API route not found error, got %q", body.Error)
	}
	if !strings.Contains(logs.String(), `"status":404`) {
		t.Fatalf("expected request log to include 404 status, got logs: %s", logs.String())
	}
	if !strings.Contains(logs.String(), `"level":"WARN"`) {
		t.Fatalf("expected request log to be warn level, got logs: %s", logs.String())
	}
}

func TestAPIMethodNotAllowedReturnsJSONError(t *testing.T) {
	router := newTestRouterWithLogger(t, slog.Default())

	response := performJSONRequest(router, http.MethodPost, "/api/health", "", nil)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d: %s", http.StatusMethodNotAllowed, response.Code, response.Body.String())
	}

	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.Error != "method not allowed for API route" {
		t.Fatalf("expected method not allowed error, got %q", body.Error)
	}
}

func newTestRouterWithLogger(t *testing.T, logger *slog.Logger) http.Handler {
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

	return NewRouter(config.Config{Environment: config.EnvironmentDevelopment}, db, logger)
}

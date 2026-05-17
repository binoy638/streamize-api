package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/config"
	"github.com/binoy638/streamize-api/apps/api/internal/database"
)

func TestHealthRoute(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "streamize.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	router := NewRouter(config.Config{Environment: config.EnvironmentDevelopment}, db, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("expected health status ok, got %q", body.Status)
	}
	if body.Database != "ok" {
		t.Fatalf("expected database status ok, got %q", body.Database)
	}
}

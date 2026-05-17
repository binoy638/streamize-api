package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/binoy638/streamize-api/apps/api/internal/config"
	"github.com/binoy638/streamize-api/apps/api/internal/database"
)

func TestBootstrapAdminCreatesAdmin(t *testing.T) {
	store := newTestStore(t)
	cfg := config.Config{
		AdminUsername:     "Root",
		AdminPassword:     "root-password",
		AdminStorageQuota: 42,
	}

	if err := store.BootstrapAdmin(context.Background(), cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	user, err := store.FindUserByUsername(context.Background(), "root")
	if err != nil {
		t.Fatalf("FindUserByUsername returned error: %v", err)
	}
	if user.Role != RoleAdmin {
		t.Fatalf("expected role %q, got %q", RoleAdmin, user.Role)
	}
	if user.StorageQuotaBytes != 42 {
		t.Fatalf("expected quota 42, got %d", user.StorageQuotaBytes)
	}
}

func TestAuthenticateAndSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, err := store.CreateUser(ctx, CreateUserParams{
		Username: "viewer",
		Password: "viewer-password",
		Role:     RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	user, err := store.Authenticate(ctx, "viewer", "viewer-password")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if user.ID != created.ID {
		t.Fatalf("expected user id %q, got %q", created.ID, user.ID)
	}

	if _, err := store.Authenticate(ctx, "viewer", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	token, err := store.CreateSession(ctx, user.ID, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	sessionUser, err := store.FindUserBySessionToken(ctx, token)
	if err != nil {
		t.Fatalf("FindUserBySessionToken returned error: %v", err)
	}
	if sessionUser.ID != user.ID {
		t.Fatalf("expected session user %q, got %q", user.ID, sessionUser.ID)
	}

	if err := store.DeleteSessionByToken(ctx, token); err != nil {
		t.Fatalf("DeleteSessionByToken returned error: %v", err)
	}
	if _, err := store.FindUserBySessionToken(ctx, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized after session delete, got %v", err)
	}
}

func newTestStore(t *testing.T) *Store {
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

	return NewStore(db)
}

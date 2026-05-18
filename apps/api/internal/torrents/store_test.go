package torrents

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/database"
)

const testMagnet = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Test%20Video"

func TestCreateAndListTorrent(t *testing.T) {
	ctx := context.Background()
	store, ownerID := newTestStore(t)

	created, err := store.CreateTorrent(ctx, CreateTorrentParams{
		OwnerUserID: ownerID,
		MagnetURI:   testMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}
	if created.OwnerUserID != ownerID {
		t.Fatalf("expected owner %q, got %q", ownerID, created.OwnerUserID)
	}
	if created.InfoHash != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("expected parsed info hash, got %q", created.InfoHash)
	}
	if created.Name != "Test Video" {
		t.Fatalf("expected display name from magnet, got %q", created.Name)
	}
	if created.Status != StatusAdded {
		t.Fatalf("expected status %q, got %q", StatusAdded, created.Status)
	}

	listed, err := store.ListTorrents(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListTorrents returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(listed))
	}
	if listed[0].ID != created.ID {
		t.Fatalf("expected listed torrent %q, got %q", created.ID, listed[0].ID)
	}
}

func TestCreateTorrentRejectsInvalidMagnet(t *testing.T) {
	store, ownerID := newTestStore(t)

	_, err := store.CreateTorrent(context.Background(), CreateTorrentParams{
		OwnerUserID: ownerID,
		MagnetURI:   "https://example.com/file.torrent",
	})
	if !errors.Is(err, ErrInvalidMagnet) {
		t.Fatalf("expected ErrInvalidMagnet, got %v", err)
	}
}

func TestMarkTorrentError(t *testing.T) {
	ctx := context.Background()
	store, ownerID := newTestStore(t)

	created, err := store.CreateTorrent(ctx, CreateTorrentParams{
		OwnerUserID: ownerID,
		MagnetURI:   testMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}

	if err := store.MarkTorrentError(ctx, created.ID, "qbt down"); err != nil {
		t.Fatalf("MarkTorrentError returned error: %v", err)
	}

	updated, err := store.FindTorrentByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindTorrentByID returned error: %v", err)
	}
	if updated.Status != StatusError {
		t.Fatalf("expected status %q, got %q", StatusError, updated.Status)
	}
	if updated.ErrorMessage != "qbt down" {
		t.Fatalf("expected error message, got %q", updated.ErrorMessage)
	}
}

func newTestStore(t *testing.T) (*Store, string) {
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

	authStore := auth.NewStore(db)
	owner, err := authStore.CreateUser(ctx, auth.CreateUserParams{
		Username: "owner",
		Password: "owner-password",
		Role:     auth.RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	return NewStore(db), owner.ID
}

package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrateCreatesCoreTables(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "streamize.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	for _, table := range []string{"users", "torrents", "torrent_files", "jobs", "shares"} {
		var name string
		err := db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
		if err != nil {
			t.Fatalf("expected table %s to exist: %v", table, err)
		}
	}
}

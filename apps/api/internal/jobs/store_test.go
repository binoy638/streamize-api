package jobs

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/binoy638/streamize-api/apps/api/internal/database"
)

func TestCreateHLSTranscodeJobIfMissingIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, inserted, err := store.CreateHLSTranscodeJobIfMissing(ctx, "tfi_123")
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}
	if !inserted {
		t.Fatal("expected first job create to insert")
	}
	if created.Type != TypeHLSTranscode || created.Status != StatusQueued {
		t.Fatalf("unexpected created job: %+v", created)
	}
	if created.DedupeKey != HLSTranscodeDedupeKey("tfi_123") {
		t.Fatalf("unexpected dedupe key %q", created.DedupeKey)
	}

	var payload HLSTranscodePayload
	if err := json.Unmarshal([]byte(created.PayloadJSON), &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if payload.TorrentFileID != "tfi_123" {
		t.Fatalf("expected torrent file id in payload, got %q", payload.TorrentFileID)
	}

	duplicate, inserted, err := store.CreateHLSTranscodeJobIfMissing(ctx, "tfi_123")
	if err != nil {
		t.Fatalf("duplicate CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate job create to reuse existing job")
	}
	if duplicate.ID != created.ID {
		t.Fatalf("expected duplicate to return job %q, got %q", created.ID, duplicate.ID)
	}
}

func TestClaimNextCompleteAndFail(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, _, err := store.CreateHLSTranscodeJobIfMissing(ctx, "tfi_123")
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}

	claimed, ok, err := store.ClaimNext(ctx, "worker-1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimNext returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected queued job to be claimed")
	}
	if claimed.ID != created.ID || claimed.Status != StatusRunning || claimed.Attempts != 1 || claimed.LockedBy != "worker-1" {
		t.Fatalf("unexpected claimed job: %+v", claimed)
	}

	_, ok, err = store.ClaimNext(ctx, "worker-2", time.Minute)
	if err != nil {
		t.Fatalf("second ClaimNext returned error: %v", err)
	}
	if ok {
		t.Fatal("expected running leased job not to be claimed again")
	}

	if err := store.Fail(ctx, claimed.ID, "ffmpeg failed", 0); err != nil {
		t.Fatalf("Fail returned error: %v", err)
	}

	reclaimed, ok, err := store.ClaimNext(ctx, "worker-2", time.Minute)
	if err != nil {
		t.Fatalf("reclaim ClaimNext returned error: %v", err)
	}
	if !ok || reclaimed.ID != created.ID || reclaimed.Attempts != 2 {
		t.Fatalf("expected failed job to be requeued and reclaimed, got ok=%t job=%+v", ok, reclaimed)
	}

	if err := store.Complete(ctx, reclaimed.ID); err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	completed, err := store.FindJobByID(ctx, reclaimed.ID)
	if err != nil {
		t.Fatalf("FindJobByID returned error: %v", err)
	}
	if completed.Status != StatusSucceeded || completed.LockedBy != "" || completed.LeaseUntil != "" {
		t.Fatalf("unexpected completed job: %+v", completed)
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

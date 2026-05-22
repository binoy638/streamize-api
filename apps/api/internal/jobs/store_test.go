package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/database"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
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

func TestBackfillMetadataIdentifyJobsQueuesPendingFiles(t *testing.T) {
	ctx := context.Background()
	store, db := newTestStoreAndDB(t)
	authStore := auth.NewStore(db)
	torrentStore := torrents.NewStore(db)

	owner, err := authStore.CreateUser(ctx, auth.CreateUserParams{
		Username: "owner",
		Password: "owner-password",
		Role:     auth.RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	torrent := createTorrentRecordForJobsTest(t, ctx, torrentStore, owner.ID, "Backfill Show")
	pendingFile := createTorrentFileForJobsTest(t, ctx, torrentStore, torrent.ID, "Backfill.Show.S01E01.mkv")
	failedFile := createTorrentFileForJobsTest(t, ctx, torrentStore, torrent.ID, "Backfill.Show.S01E02.mkv")
	prequeuedFile := createTorrentFileForJobsTest(t, ctx, torrentStore, torrent.ID, "Backfill.Show.S01E03.mkv")

	if _, err := db.ExecContext(ctx, `UPDATE torrent_files SET metadata_status = 'failed' WHERE id = ?`, failedFile.ID); err != nil {
		t.Fatalf("mark failed metadata returned error: %v", err)
	}
	if _, _, err := store.CreateMetadataIdentifyJobIfMissing(ctx, prequeuedFile.ID); err != nil {
		t.Fatalf("CreateMetadataIdentifyJobIfMissing prequeued returned error: %v", err)
	}

	created, err := store.BackfillMetadataIdentifyJobs(ctx, 100)
	if err != nil {
		t.Fatalf("BackfillMetadataIdentifyJobs returned error: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected one backfill job, got %d", created)
	}

	job, err := store.FindJobByDedupeKey(ctx, MetadataIdentifyDedupeKey(pendingFile.ID))
	if err != nil {
		t.Fatalf("FindJobByDedupeKey returned error: %v", err)
	}
	if job.Type != TypeMetadataIdentify || job.Status != StatusQueued {
		t.Fatalf("unexpected metadata job: %+v", job)
	}

	created, err = store.BackfillMetadataIdentifyJobs(ctx, 100)
	if err != nil {
		t.Fatalf("second BackfillMetadataIdentifyJobs returned error: %v", err)
	}
	if created != 0 {
		t.Fatalf("expected no duplicate backfill jobs, got %d", created)
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

func TestListJobsForOwnerOnlyReturnsOwnedTorrentFileJobs(t *testing.T) {
	ctx := context.Background()
	store, db := newTestStoreAndDB(t)
	authStore := auth.NewStore(db)
	torrentStore := torrents.NewStore(db)

	owner, err := authStore.CreateUser(ctx, auth.CreateUserParams{
		Username: "owner",
		Password: "owner-password",
		Role:     auth.RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser owner returned error: %v", err)
	}
	other, err := authStore.CreateUser(ctx, auth.CreateUserParams{
		Username: "other",
		Password: "other-password",
		Role:     auth.RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser other returned error: %v", err)
	}

	ownerTorrent := createTorrentRecordForJobsTest(t, ctx, torrentStore, owner.ID, "Owner Movie")
	ownerFile, _, err := torrentStore.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    ownerTorrent.ID,
		Name:         "Owner Movie/movie.mkv",
		Ext:          ".mkv",
		OriginalPath: "/media/originals/Owner Movie/movie.mkv",
		SizeBytes:    1024,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing owner returned error: %v", err)
	}
	ownerJob, _, err := store.CreateHLSTranscodeJobIfMissing(ctx, ownerFile.ID)
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing owner returned error: %v", err)
	}

	otherTorrent := createTorrentRecordForJobsTest(t, ctx, torrentStore, other.ID, "Other Movie")
	otherFile, _, err := torrentStore.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    otherTorrent.ID,
		Name:         "Other Movie/movie.mkv",
		Ext:          ".mkv",
		OriginalPath: "/media/originals/Other Movie/movie.mkv",
		SizeBytes:    2048,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing other returned error: %v", err)
	}
	if _, _, err := store.CreateHLSTranscodeJobIfMissing(ctx, otherFile.ID); err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing other returned error: %v", err)
	}

	records, err := store.ListJobsForOwner(ctx, owner.ID)
	if err != nil {
		t.Fatalf("ListJobsForOwner returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one owned job, got %+v", records)
	}
	if records[0].ID != ownerJob.ID || records[0].TorrentID != ownerTorrent.ID || records[0].TorrentFileID != ownerFile.ID || records[0].Target != "Owner Movie/movie.mkv" {
		t.Fatalf("unexpected owned job record: %+v", records[0])
	}
}

func TestRetryResetsFailedJobAttempts(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, _, err := store.CreateHLSTranscodeJobIfMissing(ctx, "tfi_123")
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}

	for attempt := 0; attempt < 3; attempt++ {
		claimed, ok, err := store.ClaimNext(ctx, "worker-1", time.Minute)
		if err != nil {
			t.Fatalf("ClaimNext attempt %d returned error: %v", attempt+1, err)
		}
		if !ok {
			t.Fatalf("expected job to be claimable on attempt %d", attempt+1)
		}
		if err := store.Fail(ctx, claimed.ID, "ffmpeg failed", 0); err != nil {
			t.Fatalf("Fail attempt %d returned error: %v", attempt+1, err)
		}
	}

	failed, err := store.FindJobByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindJobByID returned error: %v", err)
	}
	if failed.Status != StatusFailed || failed.Attempts != 3 {
		t.Fatalf("expected failed job after max attempts, got %+v", failed)
	}

	retried, err := store.Retry(ctx, failed.ID)
	if err != nil {
		t.Fatalf("Retry returned error: %v", err)
	}
	if retried.Status != StatusQueued || retried.Attempts != 0 || retried.LastError != "" || retried.LockedBy != "" || retried.LeaseUntil != "" {
		t.Fatalf("unexpected retried job: %+v", retried)
	}
}

func TestCancelKeepsJobUnclaimable(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, _, err := store.CreateHLSTranscodeJobIfMissing(ctx, "tfi_123")
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}
	canceled, err := store.Cancel(ctx, created.ID)
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if canceled.Status != StatusCanceled {
		t.Fatalf("expected canceled status, got %+v", canceled)
	}
	if _, err := store.Cancel(ctx, created.ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition on repeated cancel, got %v", err)
	}

	_, ok, err := store.ClaimNext(ctx, "worker-1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimNext returned error: %v", err)
	}
	if ok {
		t.Fatal("expected canceled job not to be claimable")
	}
}

func TestRequeueStaleRecoversJobsLockedByWorker(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, _, err := store.CreateHLSTranscodeJobIfMissing(ctx, "tfi_stale")
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}

	claimed, ok, err := store.ClaimNext(ctx, "worker-1", 30*time.Minute)
	if err != nil || !ok {
		t.Fatalf("ClaimNext returned ok=%t err=%v", ok, err)
	}

	// A foreign worker must not disturb a job still leased to worker-1.
	recovered, err := store.RequeueStale(ctx, "worker-2")
	if err != nil {
		t.Fatalf("RequeueStale(worker-2) returned error: %v", err)
	}
	if recovered != 0 {
		t.Fatalf("expected no jobs reclaimed by a foreign worker, got %d", recovered)
	}
	if still, err := store.FindJobByID(ctx, claimed.ID); err != nil {
		t.Fatalf("FindJobByID returned error: %v", err)
	} else if still.Status != StatusRunning {
		t.Fatalf("expected job to stay running, got %q", still.Status)
	}

	// The same worker restarting reclaims its own orphaned job.
	recovered, err = store.RequeueStale(ctx, "worker-1")
	if err != nil {
		t.Fatalf("RequeueStale(worker-1) returned error: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 job reclaimed, got %d", recovered)
	}

	requeued, err := store.FindJobByID(ctx, claimed.ID)
	if err != nil {
		t.Fatalf("FindJobByID returned error: %v", err)
	}
	if requeued.Status != StatusQueued || requeued.LockedBy != "" || requeued.LeaseUntil != "" {
		t.Fatalf("unexpected requeued job: %+v", requeued)
	}

	reclaimed, ok, err := store.ClaimNext(ctx, "worker-1", 30*time.Minute)
	if err != nil || !ok {
		t.Fatalf("reclaim ClaimNext returned ok=%t err=%v", ok, err)
	}
	if reclaimed.ID != created.ID || reclaimed.Attempts != 2 {
		t.Fatalf("expected recovered job reclaimed with attempts=2, got %+v", reclaimed)
	}
}

func TestRequeueStaleFailsJobsThatExhaustAttempts(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, _, err := store.CreateHLSTranscodeJobIfMissing(ctx, "tfi_exhaust")
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}

	// Simulate the worker crashing on every attempt until the budget is spent.
	for attempt := 1; attempt <= created.MaxAttempts; attempt++ {
		claimed, ok, err := store.ClaimNext(ctx, "worker-1", 30*time.Minute)
		if err != nil || !ok {
			t.Fatalf("attempt %d ClaimNext returned ok=%t err=%v", attempt, ok, err)
		}
		if claimed.Attempts != attempt {
			t.Fatalf("attempt %d: expected attempts=%d, got %d", attempt, attempt, claimed.Attempts)
		}
		if _, err := store.RequeueStale(ctx, "worker-1"); err != nil {
			t.Fatalf("attempt %d RequeueStale returned error: %v", attempt, err)
		}
	}

	failed, err := store.FindJobByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindJobByID returned error: %v", err)
	}
	if failed.Status != StatusFailed {
		t.Fatalf("expected job failed after exhausting attempts, got %q", failed.Status)
	}

	if _, ok, err := store.ClaimNext(ctx, "worker-1", 30*time.Minute); err != nil || ok {
		t.Fatalf("expected exhausted job not to be claimable, got ok=%t err=%v", ok, err)
	}

	// A user can still retry the recovered-but-failed job through the API.
	retried, err := store.Retry(ctx, created.ID)
	if err != nil {
		t.Fatalf("Retry returned error: %v", err)
	}
	if retried.Status != StatusQueued {
		t.Fatalf("expected retried job queued, got %q", retried.Status)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, _ := newTestStoreAndDB(t)
	return store
}

func newTestStoreAndDB(t *testing.T) (*Store, *sql.DB) {
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

	return NewStore(db), db
}

func createTorrentRecordForJobsTest(t *testing.T, ctx context.Context, store *torrents.Store, ownerUserID string, name string) torrents.Torrent {
	t.Helper()

	record, err := store.CreateTorrent(ctx, torrents.CreateTorrentParams{
		OwnerUserID: ownerUserID,
		MagnetURI:   "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=test",
		Name:        name,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}

	return record
}

func createTorrentFileForJobsTest(t *testing.T, ctx context.Context, store *torrents.Store, torrentID string, name string) torrents.TorrentFile {
	t.Helper()

	file, _, err := store.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    torrentID,
		Name:         name,
		Ext:          filepath.Ext(name),
		OriginalPath: "/media/originals/" + name,
		SizeBytes:    1024,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}

	return file
}

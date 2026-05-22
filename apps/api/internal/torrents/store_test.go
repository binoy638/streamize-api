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

func TestUpdateTorrentTransferState(t *testing.T) {
	ctx := context.Background()
	store, ownerID := newTestStore(t)

	created, err := store.CreateTorrent(ctx, CreateTorrentParams{
		OwnerUserID: ownerID,
		MagnetURI:   testMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}

	err = store.UpdateTorrentTransferState(ctx, UpdateTorrentTransferStateParams{
		ID:                 created.ID,
		QBittorrentHash:    "0123456789ABCDEF0123456789ABCDEF01234567",
		Name:               "Synced Video",
		SizeBytes:          1024,
		Status:             StatusDownloading,
		ProgressPercent:    42.5,
		DownloadSpeedBytes: 2048,
		UploadSpeedBytes:   128,
		ETASeconds:         30,
		Peers:              7,
		Ratio:              1.25,
	})
	if err != nil {
		t.Fatalf("UpdateTorrentTransferState returned error: %v", err)
	}

	updated, err := store.FindTorrentByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindTorrentByID returned error: %v", err)
	}
	if updated.Status != StatusDownloading {
		t.Fatalf("expected status %q, got %q", StatusDownloading, updated.Status)
	}
	if updated.QBittorrentHash != "0123456789ABCDEF0123456789ABCDEF01234567" {
		t.Fatalf("expected qBittorrent hash to be stored, got %q", updated.QBittorrentHash)
	}
	if updated.Name != "Synced Video" {
		t.Fatalf("expected synced name, got %q", updated.Name)
	}
	if updated.SizeBytes != 1024 || updated.ProgressPercent != 42.5 || updated.DownloadSpeedBytes != 2048 || updated.UploadSpeedBytes != 128 || updated.ETASeconds != 30 || updated.Peers != 7 || updated.Ratio != 1.25 {
		t.Fatalf("unexpected transfer state: %+v", updated)
	}
}

func TestCreateTorrentFileIfMissingIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store, ownerID := newTestStore(t)

	torrent, err := store.CreateTorrent(ctx, CreateTorrentParams{
		OwnerUserID: ownerID,
		MagnetURI:   testMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}

	created, inserted, err := store.CreateTorrentFileIfMissing(ctx, CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "Movie/movie.mkv",
		Ext:          ".mkv",
		OriginalPath: "/media/originals/Movie/movie.mkv",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}
	if !inserted {
		t.Fatal("expected first torrent file insert to report inserted")
	}
	if created.TorrentID != torrent.ID || created.Name != "Movie/movie.mkv" || created.Ext != ".mkv" || created.OriginalPath != "/media/originals/Movie/movie.mkv" {
		t.Fatalf("unexpected created torrent file: %+v", created)
	}
	if created.SizeBytes != 4096 || created.Status != FileStatusQueued {
		t.Fatalf("unexpected created torrent file state: %+v", created)
	}

	duplicate, inserted, err := store.CreateTorrentFileIfMissing(ctx, CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "Movie/movie.mkv",
		Ext:          ".mkv",
		OriginalPath: "/media/originals/Movie/movie.mkv",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("duplicate CreateTorrentFileIfMissing returned error: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate torrent file insert to report existing row")
	}
	if duplicate.ID != created.ID {
		t.Fatalf("expected duplicate to return file %q, got %q", created.ID, duplicate.ID)
	}

	files, err := store.ListTorrentFiles(ctx, torrent.ID)
	if err != nil {
		t.Fatalf("ListTorrentFiles returned error: %v", err)
	}
	if len(files) != 1 || files[0].ID != created.ID {
		t.Fatalf("expected one torrent file, got %+v", files)
	}
}

func TestTorrentFileStatusUpdates(t *testing.T) {
	ctx := context.Background()
	store, ownerID := newTestStore(t)

	torrent, err := store.CreateTorrent(ctx, CreateTorrentParams{
		OwnerUserID: ownerID,
		MagnetURI:   testMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}

	file, _, err := store.CreateTorrentFileIfMissing(ctx, CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "movie.mp4",
		Ext:          ".mp4",
		OriginalPath: "/media/originals/movie.mp4",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}

	if err := store.MarkTorrentFileProcessing(ctx, file.ID); err != nil {
		t.Fatalf("MarkTorrentFileProcessing returned error: %v", err)
	}
	processing, err := store.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}
	if processing.Status != FileStatusProcessing || processing.TranscodingPercent != 0 {
		t.Fatalf("unexpected processing file: %+v", processing)
	}

	if err := store.UpdateTorrentFileTranscodingProgress(ctx, file.ID, 42.5); err != nil {
		t.Fatalf("UpdateTorrentFileTranscodingProgress returned error: %v", err)
	}
	progressed, err := store.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}
	if progressed.Status != FileStatusProcessing || progressed.TranscodingPercent != 42.5 {
		t.Fatalf("unexpected progressed file: %+v", progressed)
	}

	if err := store.UpdateTorrentFileMediaMetadata(ctx, UpdateTorrentFileMediaMetadataParams{
		ID:              file.ID,
		Container:       "Matroska,WEBM",
		VideoCodec:      "H264",
		AudioCodec:      "AC3",
		DurationSeconds: 123.5,
		ProcessingMode:  "audio_transcode_hls",
	}); err != nil {
		t.Fatalf("UpdateTorrentFileMediaMetadata returned error: %v", err)
	}
	withMetadata, err := store.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}
	if withMetadata.Container != "matroska,webm" || withMetadata.VideoCodec != "h264" || withMetadata.AudioCodec != "ac3" || withMetadata.DurationSeconds != 123.5 || withMetadata.ProcessingMode != "audio_transcode_hls" {
		t.Fatalf("unexpected media metadata: %+v", withMetadata)
	}

	if err := store.MarkTorrentFilePreviewReady(ctx, file.ID, "/media/thumbnails/tfi_123/sprite_00000.jpg", "/media/thumbnails/tfi_123/thumbnails.vtt"); err != nil {
		t.Fatalf("MarkTorrentFilePreviewReady returned error: %v", err)
	}
	previewed, err := store.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}
	if !previewed.ProgressPreview || previewed.ThumbnailSheetPath == "" || previewed.ThumbnailVTTPath == "" {
		t.Fatalf("unexpected preview metadata: %+v", previewed)
	}

	if err := store.MarkTorrentFileDone(ctx, file.ID, "/media/hls/tfi_123/index.m3u8"); err != nil {
		t.Fatalf("MarkTorrentFileDone returned error: %v", err)
	}
	done, err := store.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}
	if done.Status != FileStatusDone || done.HLSPath != "/media/hls/tfi_123/index.m3u8" || !done.ProgressPreview || done.TranscodingPercent != 100 {
		t.Fatalf("unexpected done file: %+v", done)
	}

	if err := store.ResetTorrentFileForTranscode(ctx, file.ID); err != nil {
		t.Fatalf("ResetTorrentFileForTranscode returned error: %v", err)
	}
	reset, err := store.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}
	if reset.Status != FileStatusQueued || reset.HLSPath != "" || reset.ProgressPreview || reset.ThumbnailSheetPath != "" || reset.ThumbnailVTTPath != "" || reset.TranscodingPercent != 0 || reset.ErrorMessage != "" {
		t.Fatalf("unexpected reset file: %+v", reset)
	}

	if err := store.MarkTorrentFileError(ctx, file.ID, "ffmpeg failed"); err != nil {
		t.Fatalf("MarkTorrentFileError returned error: %v", err)
	}
	errored, err := store.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}
	if errored.Status != FileStatusError || errored.ErrorMessage != "ffmpeg failed" {
		t.Fatalf("unexpected errored file: %+v", errored)
	}
}

func TestSubtitlesAreFileOwnerScoped(t *testing.T) {
	ctx := context.Background()
	store, ownerID := newTestStore(t)

	torrent, err := store.CreateTorrent(ctx, CreateTorrentParams{
		OwnerUserID: ownerID,
		MagnetURI:   testMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}
	file, _, err := store.CreateTorrentFileIfMissing(ctx, CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "movie.mp4",
		Ext:          ".mp4",
		OriginalPath: "/media/originals/movie.mp4",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}

	subtitle, inserted, err := store.CreateSubtitleIfMissing(ctx, CreateSubtitleParams{
		TorrentFileID: file.ID,
		FileName:      "movie.en.vtt",
		Title:         "English",
		Language:      "en",
		Path:          "/media/subtitles/movie.en.vtt",
	})
	if err != nil {
		t.Fatalf("CreateSubtitleIfMissing returned error: %v", err)
	}
	if !inserted {
		t.Fatal("expected subtitle to be inserted")
	}

	duplicate, inserted, err := store.CreateSubtitleIfMissing(ctx, CreateSubtitleParams{
		TorrentFileID: file.ID,
		FileName:      "movie.en.vtt",
		Title:         "English",
		Language:      "en",
		Path:          "/media/subtitles/movie.en.vtt",
	})
	if err != nil {
		t.Fatalf("duplicate CreateSubtitleIfMissing returned error: %v", err)
	}
	if inserted || duplicate.ID != subtitle.ID {
		t.Fatalf("expected duplicate to reuse subtitle, inserted=%t duplicate=%+v", inserted, duplicate)
	}

	subtitles, err := store.ListSubtitlesForFileOwner(ctx, file.ID, ownerID)
	if err != nil {
		t.Fatalf("ListSubtitlesForFileOwner returned error: %v", err)
	}
	if len(subtitles) != 1 || subtitles[0].ID != subtitle.ID {
		t.Fatalf("expected one subtitle, got %+v", subtitles)
	}

	hidden, err := store.ListSubtitlesForFileOwner(ctx, file.ID, "usr_other")
	if err != nil {
		t.Fatalf("ListSubtitlesForFileOwner wrong owner returned error: %v", err)
	}
	if len(hidden) != 0 {
		t.Fatalf("expected subtitles to be hidden from another owner, got %+v", hidden)
	}
}

func TestVideoProgressIsFileOwnerScoped(t *testing.T) {
	ctx := context.Background()
	store, ownerID := newTestStore(t)

	torrent, err := store.CreateTorrent(ctx, CreateTorrentParams{
		OwnerUserID: ownerID,
		MagnetURI:   testMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}
	file, _, err := store.CreateTorrentFileIfMissing(ctx, CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "movie.mp4",
		Ext:          ".mp4",
		OriginalPath: "/media/originals/movie.mp4",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}

	if _, err := store.FindVideoProgressForOwner(ctx, file.ID, ownerID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing progress to return ErrNotFound, got %v", err)
	}

	progress, err := store.UpsertVideoProgressForOwner(ctx, UpdateVideoProgressParams{
		TorrentFileID:   file.ID,
		UserID:          ownerID,
		PositionSeconds: 95,
		DurationSeconds: 90,
	})
	if err != nil {
		t.Fatalf("UpsertVideoProgressForOwner returned error: %v", err)
	}
	if progress.TorrentFileID != file.ID || progress.PositionSeconds != 90 || progress.DurationSeconds != 90 || progress.UpdatedAt == "" {
		t.Fatalf("unexpected saved progress: %+v", progress)
	}

	found, err := store.FindVideoProgressForOwner(ctx, file.ID, ownerID)
	if err != nil {
		t.Fatalf("FindVideoProgressForOwner returned error: %v", err)
	}
	if found.PositionSeconds != 90 || found.DurationSeconds != 90 {
		t.Fatalf("unexpected found progress: %+v", found)
	}

	if _, err := store.UpsertVideoProgressForOwner(ctx, UpdateVideoProgressParams{
		TorrentFileID:   file.ID,
		UserID:          "usr_someone_else",
		PositionSeconds: 12,
		DurationSeconds: 120,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected wrong-owner upsert to return ErrNotFound, got %v", err)
	}
}

func TestDeleteTorrentIsOwnerScoped(t *testing.T) {
	ctx := context.Background()
	store, ownerID := newTestStore(t)

	created, err := store.CreateTorrent(ctx, CreateTorrentParams{
		OwnerUserID: ownerID,
		MagnetURI:   testMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}

	if err := store.DeleteTorrent(ctx, created.ID, "usr_someone_else"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong owner, got %v", err)
	}

	listed, err := store.ListTorrents(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListTorrents returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected wrong-owner delete to keep torrent, got %d rows", len(listed))
	}

	if err := store.DeleteTorrent(ctx, created.ID, ownerID); err != nil {
		t.Fatalf("DeleteTorrent returned error: %v", err)
	}

	listed, err = store.ListTorrents(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListTorrents returned error: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected torrent to be deleted, got %d rows", len(listed))
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

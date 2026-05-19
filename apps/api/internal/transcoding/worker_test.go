package transcoding

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/database"
	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

const testMagnet = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Test%20Video"

func TestWorkerProcessesHLSTranscodeJob(t *testing.T) {
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
	user, err := authStore.CreateUser(ctx, auth.CreateUserParams{
		Username: "owner",
		Password: "owner-password",
		Role:     auth.RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	torrentStore := torrents.NewStore(db)
	torrent, err := torrentStore.CreateTorrent(ctx, torrents.CreateTorrentParams{
		OwnerUserID: user.ID,
		MagnetURI:   testMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}

	file, _, err := torrentStore.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "movie.mp4",
		Ext:          ".mp4",
		OriginalPath: "/media/originals/movie.mp4",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}

	jobStore := jobs.NewStore(db)
	createdJob, _, err := jobStore.CreateHLSTranscodeJobIfMissing(ctx, file.ID)
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}

	transcoder := &fakeTranscoder{}
	var observedProgress float64
	transcoder.afterProgress = func() {
		processingFile, err := torrentStore.FindTorrentFileByID(ctx, file.ID)
		if err != nil {
			t.Fatalf("FindTorrentFileByID during progress returned error: %v", err)
		}
		observedProgress = processingFile.TranscodingPercent
	}
	worker := Worker{
		Jobs:       jobStore,
		Torrents:   torrentStore,
		Transcoder: transcoder,
		HLSDir:     filepath.Join(t.TempDir(), "hls"),
		ID:         "worker-test",
	}

	processed, err := worker.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext returned error: %v", err)
	}
	if !processed {
		t.Fatal("expected queued job to be processed")
	}

	updatedFile, err := torrentStore.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}
	if updatedFile.Status != torrents.FileStatusDone || updatedFile.TranscodingPercent != 100 || updatedFile.HLSPath == "" {
		t.Fatalf("unexpected processed file: %+v", updatedFile)
	}

	completedJob, err := jobStore.FindJobByID(ctx, createdJob.ID)
	if err != nil {
		t.Fatalf("FindJobByID returned error: %v", err)
	}
	if completedJob.Status != jobs.StatusSucceeded {
		t.Fatalf("expected succeeded job, got %+v", completedJob)
	}

	if len(transcoder.calls) != 1 {
		t.Fatalf("expected one transcode call, got %d", len(transcoder.calls))
	}
	if transcoder.calls[0].inputPath != "/media/originals/movie.mp4" {
		t.Fatalf("unexpected input path %q", transcoder.calls[0].inputPath)
	}
	if transcoder.calls[0].outputPlaylistPath != updatedFile.HLSPath {
		t.Fatalf("expected output playlist path %q, got %q", updatedFile.HLSPath, transcoder.calls[0].outputPlaylistPath)
	}
	if observedProgress != 42.5 {
		t.Fatalf("expected intermediate progress to be recorded, got %v", observedProgress)
	}
}

type fakeTranscoder struct {
	calls         []fakeTranscodeCall
	afterProgress func()
}

type fakeTranscodeCall struct {
	inputPath          string
	outputPlaylistPath string
	segmentPattern     string
}

func (f *fakeTranscoder) TranscodeHLS(_ context.Context, inputPath string, outputPlaylistPath string, segmentPattern string, onProgress ProgressReporter) error {
	f.calls = append(f.calls, fakeTranscodeCall{
		inputPath:          inputPath,
		outputPlaylistPath: outputPlaylistPath,
		segmentPattern:     segmentPattern,
	})
	if onProgress != nil {
		if err := onProgress(42.5); err != nil {
			return err
		}
	}
	if f.afterProgress != nil {
		f.afterProgress()
	}
	return nil
}

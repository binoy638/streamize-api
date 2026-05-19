package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

func TestJobRoutesListCancelAndRetryOwnedJobs(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	authStore := auth.NewStore(db)
	if err := authStore.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	user, err := authStore.FindUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("FindUserByUsername returned error: %v", err)
	}

	torrentStore := torrents.NewStore(db)
	torrent, err := torrentStore.CreateTorrent(ctx, torrents.CreateTorrentParams{
		OwnerUserID: user.ID,
		MagnetURI:   testTorrentMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}
	file, _, err := torrentStore.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "Test Video/movie.mkv",
		Ext:          ".mkv",
		OriginalPath: "/media/originals/Test Video/movie.mkv",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}
	createdJob, _, err := jobs.NewStore(db).CreateHLSTranscodeJobIfMissing(ctx, file.ID)
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default())
	cookie := signInForTorrentTest(t, router)

	listResponse := performJSONRequest(router, http.MethodGet, "/api/jobs", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list jobs status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}

	var listBody jobsResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("list jobs response is not valid JSON: %v", err)
	}
	if len(listBody.Jobs) != 1 {
		t.Fatalf("expected one job, got %+v", listBody.Jobs)
	}
	if listBody.Jobs[0].ID != createdJob.ID || listBody.Jobs[0].Target != "Test Video/movie.mkv" || listBody.Jobs[0].TorrentID != torrent.ID {
		t.Fatalf("unexpected listed job: %+v", listBody.Jobs[0])
	}

	cancelResponse := performJSONRequest(router, http.MethodPost, "/api/jobs/"+createdJob.ID+"/cancel", "", cookie)
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("expected cancel job status %d, got %d: %s", http.StatusOK, cancelResponse.Code, cancelResponse.Body.String())
	}
	var cancelBody jobResponse
	if err := json.Unmarshal(cancelResponse.Body.Bytes(), &cancelBody); err != nil {
		t.Fatalf("cancel job response is not valid JSON: %v", err)
	}
	if cancelBody.Job.Status != jobs.StatusCanceled {
		t.Fatalf("expected canceled job, got %+v", cancelBody.Job)
	}

	retryResponse := performJSONRequest(router, http.MethodPost, "/api/jobs/"+createdJob.ID+"/retry", "", cookie)
	if retryResponse.Code != http.StatusOK {
		t.Fatalf("expected retry job status %d, got %d: %s", http.StatusOK, retryResponse.Code, retryResponse.Body.String())
	}
	var retryBody jobResponse
	if err := json.Unmarshal(retryResponse.Body.Bytes(), &retryBody); err != nil {
		t.Fatalf("retry job response is not valid JSON: %v", err)
	}
	if retryBody.Job.Status != jobs.StatusQueued || retryBody.Job.Attempts != 0 {
		t.Fatalf("expected queued retried job, got %+v", retryBody.Job)
	}
}

func TestJobRoutesRejectInvalidTransitions(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	authStore := auth.NewStore(db)
	if err := authStore.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	user, err := authStore.FindUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("FindUserByUsername returned error: %v", err)
	}

	torrentStore := torrents.NewStore(db)
	torrent, err := torrentStore.CreateTorrent(ctx, torrents.CreateTorrentParams{
		OwnerUserID: user.ID,
		MagnetURI:   testTorrentMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}
	file, _, err := torrentStore.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "Test Video/movie.mkv",
		Ext:          ".mkv",
		OriginalPath: "/media/originals/Test Video/movie.mkv",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}
	createdJob, _, err := jobs.NewStore(db).CreateHLSTranscodeJobIfMissing(ctx, file.ID)
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default())
	cookie := signInForTorrentTest(t, router)

	retryResponse := performJSONRequest(router, http.MethodPost, "/api/jobs/"+createdJob.ID+"/retry", "", cookie)
	if retryResponse.Code != http.StatusConflict {
		t.Fatalf("expected retry conflict status %d, got %d: %s", http.StatusConflict, retryResponse.Code, retryResponse.Body.String())
	}
}

func TestJobRetryReprocessesSucceededHLSTranscode(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	authStore := auth.NewStore(db)
	if err := authStore.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	user, err := authStore.FindUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("FindUserByUsername returned error: %v", err)
	}

	torrentStore := torrents.NewStore(db)
	torrent, err := torrentStore.CreateTorrent(ctx, torrents.CreateTorrentParams{
		OwnerUserID: user.ID,
		MagnetURI:   testTorrentMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}
	file, _, err := torrentStore.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "Test Video/movie.mkv",
		Ext:          ".mkv",
		OriginalPath: "/media/originals/Test Video/movie.mkv",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}
	if err := torrentStore.MarkTorrentFileDone(ctx, file.ID, "/media/hls/"+file.ID+"/index.m3u8"); err != nil {
		t.Fatalf("MarkTorrentFileDone returned error: %v", err)
	}

	jobStore := jobs.NewStore(db)
	createdJob, _, err := jobStore.CreateHLSTranscodeJobIfMissing(ctx, file.ID)
	if err != nil {
		t.Fatalf("CreateHLSTranscodeJobIfMissing returned error: %v", err)
	}
	if _, ok, err := jobStore.ClaimNext(ctx, "worker-test", 0); err != nil || !ok {
		t.Fatalf("expected job claim before complete, ok=%t err=%v", ok, err)
	}
	if err := jobStore.Complete(ctx, createdJob.ID); err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default())
	cookie := signInForTorrentTest(t, router)

	retryResponse := performJSONRequest(router, http.MethodPost, "/api/jobs/"+createdJob.ID+"/retry", "", cookie)
	if retryResponse.Code != http.StatusOK {
		t.Fatalf("expected retry job status %d, got %d: %s", http.StatusOK, retryResponse.Code, retryResponse.Body.String())
	}

	updatedFile, err := torrentStore.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}
	if updatedFile.Status != torrents.FileStatusQueued || updatedFile.HLSPath != "" || updatedFile.TranscodingPercent != 0 {
		t.Fatalf("expected file to be reset for reprocessing, got %+v", updatedFile)
	}
}

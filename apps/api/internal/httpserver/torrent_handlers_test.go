package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/qbittorrent"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

const testTorrentMagnet = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Test%20Video"

func TestTorrentRoutesCreateAndList(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.OriginalsDir = "/media/originals"
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	adder := &fakeTorrentAdder{}
	router := NewRouter(cfg, db, slog.Default(), WithTorrentAdder(adder))

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"admin","password":"admin-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}
	cookie := signInResponse.Result().Cookies()[0]

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/torrents",
		`{"magnetUri":"`+testTorrentMagnet+`"}`,
		cookie,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create torrent status %d, got %d: %s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}
	if len(adder.calls) != 1 {
		t.Fatalf("expected 1 qBittorrent call, got %d", len(adder.calls))
	}
	if adder.calls[0].magnetURI != testTorrentMagnet {
		t.Fatalf("expected magnet URI %q, got %q", testTorrentMagnet, adder.calls[0].magnetURI)
	}
	if adder.calls[0].savePath != "/media/originals" {
		t.Fatalf("expected save path %q, got %q", "/media/originals", adder.calls[0].savePath)
	}

	listResponse := performJSONRequest(router, http.MethodGet, "/api/torrents", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}

	var body torrentsResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("list torrents response is not valid JSON: %v", err)
	}
	if len(body.Torrents) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(body.Torrents))
	}
	if body.Torrents[0].InfoHash != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("expected info hash, got %q", body.Torrents[0].InfoHash)
	}
}

func TestCreateTorrentRejectsInvalidMagnet(t *testing.T) {
	cfg := testConfig()
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(context.Background(), cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default(), WithTorrentAdder(&fakeTorrentAdder{}))

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"admin","password":"admin-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}
	cookie := signInResponse.Result().Cookies()[0]

	response := performJSONRequest(router, http.MethodPost, "/api/torrents", `{"magnetUri":"not-a-magnet"}`, cookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid magnet status %d, got %d: %s", http.StatusBadRequest, response.Code, response.Body.String())
	}
}

func TestCreateTorrentRecordsSubmissionFailure(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default(), WithTorrentAdder(&fakeTorrentAdder{err: errors.New("qBittorrent unavailable")}))

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"admin","password":"admin-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}
	cookie := signInResponse.Result().Cookies()[0]

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/torrents",
		`{"magnetUri":"`+testTorrentMagnet+`"}`,
		cookie,
	)
	if createResponse.Code != http.StatusBadGateway {
		t.Fatalf("expected create torrent status %d, got %d: %s", http.StatusBadGateway, createResponse.Code, createResponse.Body.String())
	}

	listResponse := performJSONRequest(router, http.MethodGet, "/api/torrents", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}

	var body torrentsResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("list torrents response is not valid JSON: %v", err)
	}
	if len(body.Torrents) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(body.Torrents))
	}
	if body.Torrents[0].Status != "error" {
		t.Fatalf("expected torrent status error, got %q", body.Torrents[0].Status)
	}
	if body.Torrents[0].ErrorMessage != "qBittorrent unavailable" {
		t.Fatalf("expected submission error to be recorded, got %q", body.Torrents[0].ErrorMessage)
	}
}

func TestListTorrentsSyncsQBittorrentState(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	lister := &fakeTorrentLister{
		torrents: []qbittorrent.TorrentInfo{
			{
				Hash:          "0123456789ABCDEF0123456789ABCDEF01234567",
				Name:          "Synced Test Video",
				Size:          4096,
				Progress:      1,
				State:         "uploading",
				DownloadSpeed: 0,
				UploadSpeed:   256,
				NumSeeds:      4,
				NumLeechs:     1,
				Ratio:         2.5,
				ETA:           8640000,
			},
		},
	}
	router := NewRouter(cfg, db, slog.Default(), WithTorrentAdder(&fakeTorrentAdder{}), WithTorrentLister(lister))

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"admin","password":"admin-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}
	cookie := signInResponse.Result().Cookies()[0]

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/torrents",
		`{"magnetUri":"`+testTorrentMagnet+`"}`,
		cookie,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create torrent status %d, got %d: %s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}

	listResponse := performJSONRequest(router, http.MethodGet, "/api/torrents", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}
	if lister.calls != 1 {
		t.Fatalf("expected 1 qBittorrent list call, got %d", lister.calls)
	}

	var body torrentsResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("list torrents response is not valid JSON: %v", err)
	}
	if len(body.Torrents) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(body.Torrents))
	}
	torrent := body.Torrents[0]
	if torrent.Status != "done" {
		t.Fatalf("expected synced torrent status done, got %q", torrent.Status)
	}
	if torrent.QBittorrentHash != "0123456789ABCDEF0123456789ABCDEF01234567" {
		t.Fatalf("expected qBittorrent hash, got %q", torrent.QBittorrentHash)
	}
	if torrent.Name != "Synced Test Video" || torrent.SizeBytes != 4096 || torrent.ProgressPercent != 100 || torrent.UploadSpeedBytes != 256 || torrent.Peers != 5 || torrent.Ratio != 2.5 || torrent.ETASeconds != -1 {
		t.Fatalf("unexpected synced torrent: %+v", torrent)
	}
}

func TestListTorrentFilesIngestsCompletedVideoFiles(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.OriginalsDir = "/media/originals"
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	lister := &fakeTorrentLister{
		torrents: []qbittorrent.TorrentInfo{
			{
				Hash:     "0123456789ABCDEF0123456789ABCDEF01234567",
				Name:     "Completed Test Video",
				Size:     4096,
				Progress: 1,
				State:    "uploading",
			},
		},
	}
	fileLister := &fakeTorrentFileLister{
		files: []qbittorrent.TorrentFile{
			{Name: "Completed Test Video/movie.mkv", Size: 4096, Priority: 1},
			{Name: "Completed Test Video/readme.txt", Size: 128, Priority: 1},
			{Name: "../outside.mp4", Size: 1024, Priority: 1},
		},
	}
	router := NewRouter(
		cfg,
		db,
		slog.Default(),
		WithTorrentAdder(&fakeTorrentAdder{}),
		WithTorrentLister(lister),
		WithTorrentFileLister(fileLister),
	)

	cookie := signInForTorrentTest(t, router)
	created := createTorrentForTest(t, router, cookie)

	listResponse := performJSONRequest(router, http.MethodGet, "/api/torrents", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}

	filesResponse := performJSONRequest(router, http.MethodGet, "/api/torrents/"+created.Torrent.ID+"/files", "", cookie)
	if filesResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent files status %d, got %d: %s", http.StatusOK, filesResponse.Code, filesResponse.Body.String())
	}

	var body torrentFilesResponse
	if err := json.Unmarshal(filesResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("list torrent files response is not valid JSON: %v", err)
	}
	if len(body.Files) != 1 {
		t.Fatalf("expected one supported safe video file, got %+v", body.Files)
	}

	file := body.Files[0]
	if file.TorrentID != created.Torrent.ID {
		t.Fatalf("expected torrent id %q, got %q", created.Torrent.ID, file.TorrentID)
	}
	if file.Name != "Completed Test Video/movie.mkv" || file.Ext != ".mkv" {
		t.Fatalf("unexpected ingested file name or extension: %+v", file)
	}
	if file.OriginalPath != filepath.Join("/media/originals", "Completed Test Video/movie.mkv") {
		t.Fatalf("unexpected original path %q", file.OriginalPath)
	}
	if file.SizeBytes != 4096 || file.Status != "queued" {
		t.Fatalf("unexpected ingested file state: %+v", file)
	}
	createdJob, err := jobs.NewStore(db).FindJobByDedupeKey(ctx, jobs.HLSTranscodeDedupeKey(file.ID))
	if err != nil {
		t.Fatalf("expected hls transcode job to be queued: %v", err)
	}
	if createdJob.Status != jobs.StatusQueued || createdJob.Type != jobs.TypeHLSTranscode {
		t.Fatalf("unexpected hls transcode job: %+v", createdJob)
	}

	filesResponse = performJSONRequest(router, http.MethodGet, "/api/torrents/"+created.Torrent.ID+"/files", "", cookie)
	if filesResponse.Code != http.StatusOK {
		t.Fatalf("expected second list torrent files status %d, got %d: %s", http.StatusOK, filesResponse.Code, filesResponse.Body.String())
	}
	if err := json.Unmarshal(filesResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("second list torrent files response is not valid JSON: %v", err)
	}
	if len(body.Files) != 1 || body.Files[0].ID != file.ID {
		t.Fatalf("expected idempotent file ingestion, got %+v", body.Files)
	}
}

func TestCreateTorrentQueuesPreflightWithoutStoppingMetadataFetch(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.OriginalsDir = "/media/originals"
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	adder := &fakeTorrentAdder{}
	router := NewRouter(
		cfg,
		db,
		slog.Default(),
		WithTorrentAdder(adder),
		WithTorrentLister(&fakeTorrentLister{}),
		WithTorrentFileLister(&fakeTorrentFileLister{}),
		WithTorrentDeleter(&fakeTorrentDeleter{}),
		WithTorrentResumer(&fakeTorrentResumer{}),
		WithFreeDiskBytes(func(string) (int64, error) { return 100 << 30, nil }),
	)

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"admin","password":"admin-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}
	cookie := signInResponse.Result().Cookies()[0]

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/torrents",
		`{"magnetUri":"`+testTorrentMagnet+`"}`,
		cookie,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create torrent status %d, got %d: %s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}
	if len(adder.calls) != 1 {
		t.Fatalf("expected 1 qBittorrent add call, got %d", len(adder.calls))
	}
	if adder.calls[0].paused {
		t.Fatal("expected qBittorrent add call to allow metadata fetching")
	}

	var created torrentResponse
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("create torrent response is not valid JSON: %v", err)
	}
	if created.Torrent.Status != "queued" {
		t.Fatalf("expected queued status while waiting for preflight, got %q", created.Torrent.Status)
	}
}

func TestListTorrentsPreflightResumesValidVideoTorrent(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	lister := &fakeTorrentLister{
		torrents: []qbittorrent.TorrentInfo{
			{
				Hash:     "0123456789ABCDEF0123456789ABCDEF01234567",
				Name:     "Playable Movie",
				Size:     4096,
				Progress: 0,
				State:    "pausedDL",
			},
		},
	}
	fileLister := &fakeTorrentFileLister{
		files: []qbittorrent.TorrentFile{{Name: "movie.mkv", Size: 4096, Priority: 1}},
	}
	resumer := &fakeTorrentResumer{}
	router := NewRouter(
		cfg,
		db,
		slog.Default(),
		WithTorrentAdder(&fakeTorrentAdder{}),
		WithTorrentLister(lister),
		WithTorrentFileLister(fileLister),
		WithTorrentDeleter(&fakeTorrentDeleter{}),
		WithTorrentResumer(resumer),
		WithFreeDiskBytes(func(string) (int64, error) { return 100 << 30, nil }),
	)

	cookie := signInForTorrentTest(t, router)
	created := createTorrentForTest(t, router, cookie)

	listResponse := performJSONRequest(router, http.MethodGet, "/api/torrents", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}
	// preflight (1) + ingestion for the now-downloading torrent (1)
	if fileLister.calls != 2 {
		t.Fatalf("expected 2 qBittorrent files calls (preflight + ingestion), got %d", fileLister.calls)
	}
	if len(resumer.calls) != 1 || resumer.calls[0] != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("expected torrent resume call, got %#v", resumer.calls)
	}

	var body torrentsResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("list torrents response is not valid JSON: %v", err)
	}
	if len(body.Torrents) != 1 || body.Torrents[0].ID != created.Torrent.ID {
		t.Fatalf("expected listed created torrent, got %+v", body.Torrents)
	}
	if body.Torrents[0].Status != "downloading" {
		t.Fatalf("expected preflighted torrent to resume downloading, got %q", body.Torrents[0].Status)
	}
}

func TestListTorrentsPreflightRejectsNonVideoTorrent(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	deleter := &fakeTorrentDeleter{}
	router := NewRouter(
		cfg,
		db,
		slog.Default(),
		WithTorrentAdder(&fakeTorrentAdder{}),
		WithTorrentLister(&fakeTorrentLister{
			torrents: []qbittorrent.TorrentInfo{{Hash: "0123456789ABCDEF0123456789ABCDEF01234567", Name: "Docs", Size: 4096, State: "pausedDL"}},
		}),
		WithTorrentFileLister(&fakeTorrentFileLister{files: []qbittorrent.TorrentFile{{Name: "readme.txt", Size: 4096, Priority: 1}}}),
		WithTorrentDeleter(deleter),
		WithTorrentResumer(&fakeTorrentResumer{}),
		WithFreeDiskBytes(func(string) (int64, error) { return 100 << 30, nil }),
	)

	cookie := signInForTorrentTest(t, router)
	createTorrentForTest(t, router, cookie)

	listResponse := performJSONRequest(router, http.MethodGet, "/api/torrents", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}
	if len(deleter.calls) != 1 {
		t.Fatalf("expected qBittorrent cleanup call, got %d", len(deleter.calls))
	}
	if !deleter.calls[0].deleteFiles {
		t.Fatal("expected rejected preflight to delete partial qBittorrent files")
	}

	var body torrentsResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("list torrents response is not valid JSON: %v", err)
	}
	if len(body.Torrents) != 1 || body.Torrents[0].Status != "error" {
		t.Fatalf("expected errored torrent, got %+v", body.Torrents)
	}
	if body.Torrents[0].ErrorMessage != "torrent does not contain a supported video file" {
		t.Fatalf("expected unsupported video error, got %q", body.Torrents[0].ErrorMessage)
	}
}

func TestListTorrentsPreflightRejectsInsufficientDiskStorage(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	deleter := &fakeTorrentDeleter{}
	router := NewRouter(
		cfg,
		db,
		slog.Default(),
		WithTorrentAdder(&fakeTorrentAdder{}),
		WithTorrentLister(&fakeTorrentLister{
			torrents: []qbittorrent.TorrentInfo{{Hash: "0123456789ABCDEF0123456789ABCDEF01234567", Name: "Huge Movie", Size: 10 << 30, State: "pausedDL"}},
		}),
		WithTorrentFileLister(&fakeTorrentFileLister{files: []qbittorrent.TorrentFile{{Name: "movie.mp4", Size: 10 << 30, Priority: 1}}}),
		WithTorrentDeleter(deleter),
		WithTorrentResumer(&fakeTorrentResumer{}),
		WithFreeDiskBytes(func(string) (int64, error) { return 2 << 30, nil }),
	)

	cookie := signInForTorrentTest(t, router)
	createTorrentForTest(t, router, cookie)

	listResponse := performJSONRequest(router, http.MethodGet, "/api/torrents", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}
	if len(deleter.calls) != 1 || !deleter.calls[0].deleteFiles {
		t.Fatalf("expected qBittorrent cleanup with files deleted, got %#v", deleter.calls)
	}

	var body torrentsResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &body); err != nil {
		t.Fatalf("list torrents response is not valid JSON: %v", err)
	}
	if len(body.Torrents) != 1 || body.Torrents[0].Status != "error" {
		t.Fatalf("expected errored torrent, got %+v", body.Torrents)
	}
	if body.Torrents[0].ErrorMessage != "not enough free disk storage for torrent" {
		t.Fatalf("expected disk storage error, got %q", body.Torrents[0].ErrorMessage)
	}
}

func TestTorrentRoutesDelete(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.HLSDir = filepath.Join(t.TempDir(), "hls")
	cfg.SubtitlesDir = filepath.Join(t.TempDir(), "subtitles")
	cfg.ThumbnailsDir = filepath.Join(t.TempDir(), "thumbnails")
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	deleter := &fakeTorrentDeleter{}
	router := NewRouter(cfg, db, slog.Default(), WithTorrentAdder(&fakeTorrentAdder{}), WithTorrentDeleter(deleter))

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"admin","password":"admin-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}
	cookie := signInResponse.Result().Cookies()[0]

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/torrents",
		`{"magnetUri":"`+testTorrentMagnet+`"}`,
		cookie,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create torrent status %d, got %d: %s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}

	var created torrentResponse
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("create torrent response is not valid JSON: %v", err)
	}

	torrentStore := torrents.NewStore(db)
	file, _, err := torrentStore.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    created.Torrent.ID,
		Name:         "movie.mp4",
		Ext:          ".mp4",
		OriginalPath: filepath.Join(cfg.OriginalsDir, "movie.mp4"),
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}
	hlsDir := filepath.Join(cfg.HLSDir, file.ID)
	subtitleDir := filepath.Join(cfg.SubtitlesDir, file.ID)
	thumbnailDir := filepath.Join(cfg.ThumbnailsDir, file.ID)
	for _, dir := range []string{hlsDir, subtitleDir, thumbnailDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %q returned error: %v", dir, err)
		}
	}
	playlistPath := filepath.Join(hlsDir, "index.m3u8")
	spritePath := filepath.Join(thumbnailDir, "sprite_00000.jpg")
	vttPath := filepath.Join(thumbnailDir, "thumbnails.vtt")
	for path, body := range map[string]string{
		playlistPath:                         "#EXTM3U\n",
		filepath.Join(subtitleDir, "en.vtt"): "WEBVTT\n",
		spritePath:                           "jpg",
		vttPath:                              "WEBVTT\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile %q returned error: %v", path, err)
		}
	}
	if err := torrentStore.MarkTorrentFileDone(ctx, file.ID, playlistPath); err != nil {
		t.Fatalf("MarkTorrentFileDone returned error: %v", err)
	}
	if err := torrentStore.MarkTorrentFilePreviewReady(ctx, file.ID, spritePath, vttPath); err != nil {
		t.Fatalf("MarkTorrentFilePreviewReady returned error: %v", err)
	}

	deleteResponse := performJSONRequest(router, http.MethodDelete, "/api/torrents/"+created.Torrent.ID, "", cookie)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("expected delete torrent status %d, got %d: %s", http.StatusNoContent, deleteResponse.Code, deleteResponse.Body.String())
	}
	if len(deleter.calls) != 1 {
		t.Fatalf("expected 1 qBittorrent delete call, got %d", len(deleter.calls))
	}
	if deleter.calls[0].hash != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("expected qBittorrent delete hash %q, got %q", "0123456789abcdef0123456789abcdef01234567", deleter.calls[0].hash)
	}
	if deleter.calls[0].deleteFiles {
		t.Fatal("expected qBittorrent delete to keep files")
	}
	for _, dir := range []string{hlsDir, subtitleDir, thumbnailDir} {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected generated media dir %q to be deleted, stat error: %v", dir, err)
		}
	}

	listResponse := performJSONRequest(router, http.MethodGet, "/api/torrents", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}

	var listBody torrentsResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("list torrents response is not valid JSON: %v", err)
	}
	if len(listBody.Torrents) != 0 {
		t.Fatalf("expected 0 torrents after delete, got %d", len(listBody.Torrents))
	}
}

func TestTorrentRoutesDeleteKeepsRecordWhenQBittorrentDeleteFails(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default(), WithTorrentAdder(&fakeTorrentAdder{}), WithTorrentDeleter(&fakeTorrentDeleter{err: errors.New("qBittorrent unavailable")}))

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"admin","password":"admin-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}
	cookie := signInResponse.Result().Cookies()[0]

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/torrents",
		`{"magnetUri":"`+testTorrentMagnet+`"}`,
		cookie,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create torrent status %d, got %d: %s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}

	var created torrentResponse
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("create torrent response is not valid JSON: %v", err)
	}

	deleteResponse := performJSONRequest(router, http.MethodDelete, "/api/torrents/"+created.Torrent.ID, "", cookie)
	if deleteResponse.Code != http.StatusBadGateway {
		t.Fatalf("expected delete torrent status %d, got %d: %s", http.StatusBadGateway, deleteResponse.Code, deleteResponse.Body.String())
	}

	listResponse := performJSONRequest(router, http.MethodGet, "/api/torrents", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list torrent status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}

	var listBody torrentsResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("list torrents response is not valid JSON: %v", err)
	}
	if len(listBody.Torrents) != 1 {
		t.Fatalf("expected torrent row to remain after qBittorrent failure, got %d rows", len(listBody.Torrents))
	}
}

func TestTorrentRoutesDeleteHonorsCleanupChoices(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.OriginalsDir = filepath.Join(t.TempDir(), "originals")
	cfg.HLSDir = filepath.Join(t.TempDir(), "hls")
	cfg.SubtitlesDir = filepath.Join(t.TempDir(), "subtitles")
	cfg.ThumbnailsDir = filepath.Join(t.TempDir(), "thumbnails")
	db := testDB(t)
	store := auth.NewStore(db)
	if err := store.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}

	deleter := &fakeTorrentDeleter{}
	router := NewRouter(cfg, db, slog.Default(), WithTorrentAdder(&fakeTorrentAdder{}), WithTorrentDeleter(deleter))
	cookie := signInForTorrentTest(t, router)
	created := createTorrentForTest(t, router, cookie)

	torrentStore := torrents.NewStore(db)
	originalPath := filepath.Join(cfg.OriginalsDir, "nested", "movie.mp4")
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		t.Fatalf("MkdirAll original dir returned error: %v", err)
	}
	if err := os.WriteFile(originalPath, []byte("mp4"), 0o644); err != nil {
		t.Fatalf("WriteFile original returned error: %v", err)
	}
	file, _, err := torrentStore.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    created.Torrent.ID,
		Name:         "nested/movie.mp4",
		Ext:          ".mp4",
		OriginalPath: originalPath,
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}

	hlsDir := filepath.Join(cfg.HLSDir, file.ID)
	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll hls dir returned error: %v", err)
	}
	playlistPath := filepath.Join(hlsDir, "index.m3u8")
	if err := os.WriteFile(playlistPath, []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatalf("WriteFile playlist returned error: %v", err)
	}
	if err := torrentStore.MarkTorrentFileDone(ctx, file.ID, playlistPath); err != nil {
		t.Fatalf("MarkTorrentFileDone returned error: %v", err)
	}

	deleteResponse := performJSONRequest(router, http.MethodDelete, "/api/torrents/"+created.Torrent.ID+"?deleteFiles=true&deleteGenerated=false", "", cookie)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("expected delete torrent status %d, got %d: %s", http.StatusNoContent, deleteResponse.Code, deleteResponse.Body.String())
	}
	if len(deleter.calls) != 1 || !deleter.calls[0].deleteFiles {
		t.Fatalf("expected qBittorrent deleteFiles=true, got %#v", deleter.calls)
	}
	if _, err := os.Stat(originalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected original file to be deleted, stat error: %v", err)
	}
	if _, err := os.Stat(hlsDir); err != nil {
		t.Fatalf("expected generated HLS dir to remain, stat error: %v", err)
	}
}

func signInForTorrentTest(t *testing.T, router http.Handler) *http.Cookie {
	t.Helper()

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"admin","password":"admin-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}

	return signInResponse.Result().Cookies()[0]
}

func createTorrentForTest(t *testing.T, router http.Handler, cookie *http.Cookie) torrentResponse {
	t.Helper()

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/torrents",
		`{"magnetUri":"`+testTorrentMagnet+`"}`,
		cookie,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create torrent status %d, got %d: %s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}

	var created torrentResponse
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("create torrent response is not valid JSON: %v", err)
	}

	return created
}

type fakeTorrentAdder struct {
	calls []fakeTorrentAddCall
	err   error
}

type fakeTorrentAddCall struct {
	magnetURI string
	savePath  string
	paused    bool
}

func (f *fakeTorrentAdder) AddMagnet(_ context.Context, magnetURI string, savePath string, paused bool) error {
	f.calls = append(f.calls, fakeTorrentAddCall{magnetURI: magnetURI, savePath: savePath, paused: paused})
	return f.err
}

type fakeTorrentLister struct {
	torrents []qbittorrent.TorrentInfo
	calls    int
	err      error
}

func (f *fakeTorrentLister) ListTorrents(context.Context) ([]qbittorrent.TorrentInfo, error) {
	f.calls++
	return f.torrents, f.err
}

type fakeTorrentFileLister struct {
	files []qbittorrent.TorrentFile
	calls int
	err   error
}

func (f *fakeTorrentFileLister) ListTorrentFiles(context.Context, string) ([]qbittorrent.TorrentFile, error) {
	f.calls++
	return f.files, f.err
}

type fakeTorrentDeleter struct {
	calls []fakeTorrentDeleteCall
	err   error
}

type fakeTorrentDeleteCall struct {
	hash        string
	deleteFiles bool
}

func (f *fakeTorrentDeleter) DeleteTorrent(_ context.Context, hash string, deleteFiles bool) error {
	f.calls = append(f.calls, fakeTorrentDeleteCall{hash: hash, deleteFiles: deleteFiles})
	return f.err
}

type fakeTorrentResumer struct {
	calls []string
	err   error
}

func (f *fakeTorrentResumer) ResumeTorrent(_ context.Context, hash string) error {
	f.calls = append(f.calls, hash)
	return f.err
}

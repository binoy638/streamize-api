package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/qbittorrent"
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
	if fileLister.calls != 1 {
		t.Fatalf("expected 1 qBittorrent files call, got %d", fileLister.calls)
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

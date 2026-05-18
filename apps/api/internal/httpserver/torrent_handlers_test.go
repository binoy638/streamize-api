package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
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

type fakeTorrentAdder struct {
	calls []fakeTorrentAddCall
	err   error
}

type fakeTorrentAddCall struct {
	magnetURI string
	savePath  string
}

func (f *fakeTorrentAdder) AddMagnet(_ context.Context, magnetURI string, savePath string) error {
	f.calls = append(f.calls, fakeTorrentAddCall{magnetURI: magnetURI, savePath: savePath})
	return f.err
}

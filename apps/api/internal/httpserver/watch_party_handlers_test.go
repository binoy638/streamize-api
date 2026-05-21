package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
	"github.com/binoy638/streamize-api/apps/api/internal/watchparty"
)

func TestWatchPartyCreateJoinAndPartyScopedPlaylist(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.HLSDir = filepath.Join(t.TempDir(), "hls")
	db := testDB(t)
	authStore := auth.NewStore(db)
	if err := authStore.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	user, err := authStore.FindUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("FindUserByUsername returned error: %v", err)
	}
	file := createPlayableFileForTest(t, ctx, torrents.NewStore(db), user.ID, cfg.HLSDir)
	router := NewRouter(cfg, db, slog.Default())
	cookie := signInForTorrentTest(t, router)

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/watch-parties",
		`{"torrentFileId":"`+file.ID+`","controlMode":"everyone","displayName":"Host"}`,
		cookie,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}
	var created watchPartyResponse
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("create watch party response is not valid JSON: %v", err)
	}
	if created.Party.ControlMode != watchparty.ControlModeEveryone || created.Session == nil || created.Session.Participant.Role != watchparty.RoleHost {
		t.Fatalf("unexpected created watch party response: %+v", created)
	}

	publicResponse := performJSONRequest(router, http.MethodGet, "/api/watch-parties/join/"+created.Party.Slug, "", nil)
	if publicResponse.Code != http.StatusOK {
		t.Fatalf("expected public metadata status %d, got %d: %s", http.StatusOK, publicResponse.Code, publicResponse.Body.String())
	}

	joinResponse := performJSONRequest(router, http.MethodPost, "/api/watch-parties/join/"+created.Party.Slug, `{"displayName":"Guest"}`, nil)
	if joinResponse.Code != http.StatusCreated {
		t.Fatalf("expected join status %d, got %d: %s", http.StatusCreated, joinResponse.Code, joinResponse.Body.String())
	}
	var joined watchPartyResponse
	if err := json.Unmarshal(joinResponse.Body.Bytes(), &joined); err != nil {
		t.Fatalf("join watch party response is not valid JSON: %v", err)
	}
	if joined.Session == nil || joined.Session.Participant.Role != watchparty.RoleGuest {
		t.Fatalf("expected guest session, got %+v", joined.Session)
	}

	query := url.Values{
		"participantId": {joined.Session.Participant.ID},
		"token":         {joined.Session.Token},
	}.Encode()
	playlistResponse := performJSONRequest(
		router,
		http.MethodGet,
		"/api/watch-parties/join/"+created.Party.Slug+"/files/"+file.ID+"/hls/index.m3u8?"+query,
		"",
		nil,
	)
	if playlistResponse.Code != http.StatusOK {
		t.Fatalf("expected playlist status %d, got %d: %s", http.StatusOK, playlistResponse.Code, playlistResponse.Body.String())
	}
	playlist := playlistResponse.Body.String()
	if !strings.Contains(playlist, "/api/watch-parties/join/"+created.Party.Slug+"/files/"+file.ID+"/hls/segment_00000.ts?") {
		t.Fatalf("expected party-scoped segment URL in playlist, got:\n%s", playlist)
	}
	if strings.Contains(playlist, cfg.HLSDir) {
		t.Fatalf("expected playlist not to expose filesystem paths, got:\n%s", playlist)
	}
}

func TestWatchPartyPublicPlaybackRejectsUnselectedFile(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.HLSDir = filepath.Join(t.TempDir(), "hls")
	db := testDB(t)
	authStore := auth.NewStore(db)
	if err := authStore.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	user, err := authStore.FindUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("FindUserByUsername returned error: %v", err)
	}
	store := torrents.NewStore(db)
	selected := createPlayableFileForTest(t, ctx, store, user.ID, cfg.HLSDir)
	other := createPlayableFileForTest(t, ctx, store, user.ID, cfg.HLSDir)
	router := NewRouter(cfg, db, slog.Default())
	cookie := signInForTorrentTest(t, router)

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/watch-parties",
		`{"torrentFileId":"`+selected.ID+`","controlMode":"host_only"}`,
		cookie,
	)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}
	var created watchPartyResponse
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("create watch party response is not valid JSON: %v", err)
	}

	query := url.Values{
		"participantId": {created.Session.Participant.ID},
		"token":         {created.Session.Token},
	}.Encode()
	response := performJSONRequest(
		router,
		http.MethodGet,
		"/api/watch-parties/join/"+created.Party.Slug+"/files/"+other.ID+"/hls/index.m3u8?"+query,
		"",
		nil,
	)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected unselected file status %d, got %d: %s", http.StatusNotFound, response.Code, response.Body.String())
	}
}

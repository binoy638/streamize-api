package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

func TestPlaybackRoutesServeOwnedHLSPlaylistAndSegment(t *testing.T) {
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

	playlistResponse := performJSONRequest(router, http.MethodGet, "/api/files/"+file.ID+"/hls/index.m3u8", "", cookie)
	if playlistResponse.Code != http.StatusOK {
		t.Fatalf("expected playlist status %d, got %d: %s", http.StatusOK, playlistResponse.Code, playlistResponse.Body.String())
	}
	playlist := playlistResponse.Body.String()
	expectedSegmentURL := "/api/files/" + file.ID + "/hls/segment_00000.ts"
	if !strings.Contains(playlist, expectedSegmentURL) {
		t.Fatalf("expected playlist to contain rewritten segment URL %q, got:\n%s", expectedSegmentURL, playlist)
	}
	if strings.Contains(playlist, filepath.Join(cfg.HLSDir, file.ID)) {
		t.Fatalf("expected playlist not to expose filesystem paths, got:\n%s", playlist)
	}

	segmentResponse := performJSONRequest(router, http.MethodGet, "/api/files/"+file.ID+"/hls/segment_00000.ts", "", cookie)
	if segmentResponse.Code != http.StatusOK {
		t.Fatalf("expected segment status %d, got %d: %s", http.StatusOK, segmentResponse.Code, segmentResponse.Body.String())
	}
	if segmentResponse.Body.String() != "segment-bytes" {
		t.Fatalf("expected segment bytes, got %q", segmentResponse.Body.String())
	}
}

func TestPlaybackRoutesHideFilesOwnedByOtherUsers(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.HLSDir = filepath.Join(t.TempDir(), "hls")
	db := testDB(t)
	authStore := auth.NewStore(db)
	if err := authStore.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	admin, err := authStore.FindUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("FindUserByUsername admin returned error: %v", err)
	}
	if _, err := authStore.CreateUser(ctx, auth.CreateUserParams{
		Username: "viewer",
		Password: "viewer-password",
		Role:     auth.RoleUser,
	}); err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	file := createPlayableFileForTest(t, ctx, torrents.NewStore(db), admin.ID, cfg.HLSDir)
	router := NewRouter(cfg, db, slog.Default())

	signInResponse := performJSONRequest(router, http.MethodPost, "/api/auth/sign-in", `{"username":"viewer","password":"viewer-password"}`, nil)
	if signInResponse.Code != http.StatusOK {
		t.Fatalf("expected sign-in status %d, got %d: %s", http.StatusOK, signInResponse.Code, signInResponse.Body.String())
	}
	cookie := signInResponse.Result().Cookies()[0]

	response := performJSONRequest(router, http.MethodGet, "/api/files/"+file.ID+"/hls/index.m3u8", "", cookie)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected not found status %d, got %d: %s", http.StatusNotFound, response.Code, response.Body.String())
	}
}

func TestPlaybackRoutesRejectUnreadyFile(t *testing.T) {
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
		Name:         "movie.mp4",
		Ext:          ".mp4",
		OriginalPath: "/media/originals/movie.mp4",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default())
	cookie := signInForTorrentTest(t, router)

	response := performJSONRequest(router, http.MethodGet, "/api/files/"+file.ID+"/hls/index.m3u8", "", cookie)
	if response.Code != http.StatusConflict {
		t.Fatalf("expected conflict status %d, got %d: %s", http.StatusConflict, response.Code, response.Body.String())
	}
}

func TestPlaybackRouteServesDirectOriginalForCompatibleFile(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.OriginalsDir = filepath.Join(t.TempDir(), "originals")
	db := testDB(t)
	authStore := auth.NewStore(db)
	if err := authStore.BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	user, err := authStore.FindUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("FindUserByUsername returned error: %v", err)
	}

	if err := os.MkdirAll(cfg.OriginalsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll originals returned error: %v", err)
	}
	originalPath := filepath.Join(cfg.OriginalsDir, "movie.mp4")
	if err := os.WriteFile(originalPath, []byte("mp4-bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile original returned error: %v", err)
	}

	store := torrents.NewStore(db)
	torrent, err := store.CreateTorrent(ctx, torrents.CreateTorrentParams{
		OwnerUserID: user.ID,
		MagnetURI:   testTorrentMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}
	file, _, err := store.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "movie.mp4",
		Ext:          ".mp4",
		OriginalPath: originalPath,
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}
	if err := store.UpdateTorrentFileMediaMetadata(ctx, torrents.UpdateTorrentFileMediaMetadataParams{
		ID:         file.ID,
		Container:  "mov,mp4,m4a,3gp,3g2,mj2",
		VideoCodec: "h264",
		AudioCodec: "aac",
	}); err != nil {
		t.Fatalf("UpdateTorrentFileMediaMetadata returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default())
	cookie := signInForTorrentTest(t, router)
	response := performJSONRequest(router, http.MethodGet, "/api/files/"+file.ID+"/original", "", cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("expected original status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	if response.Body.String() != "mp4-bytes" {
		t.Fatalf("expected original bytes, got %q", response.Body.String())
	}
}

func TestPlaybackRoutesServeSubtitlesAndPreviewAssets(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.SubtitlesDir = filepath.Join(t.TempDir(), "subtitles")
	cfg.ThumbnailsDir = filepath.Join(t.TempDir(), "thumbnails")
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
	torrent, err := store.CreateTorrent(ctx, torrents.CreateTorrentParams{
		OwnerUserID: user.ID,
		MagnetURI:   testTorrentMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}
	file, _, err := store.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "movie.mp4",
		Ext:          ".mp4",
		OriginalPath: "/media/originals/movie.mp4",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}

	subtitleDir := filepath.Join(cfg.SubtitlesDir, file.ID)
	if err := os.MkdirAll(subtitleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll subtitle dir returned error: %v", err)
	}
	subtitlePath := filepath.Join(subtitleDir, "movie.en.vtt")
	if err := os.WriteFile(subtitlePath, []byte("WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nHello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile subtitle returned error: %v", err)
	}
	subtitle, _, err := store.CreateSubtitleIfMissing(ctx, torrents.CreateSubtitleParams{
		TorrentFileID: file.ID,
		FileName:      "movie.en.vtt",
		Title:         "English",
		Language:      "en",
		Path:          subtitlePath,
	})
	if err != nil {
		t.Fatalf("CreateSubtitleIfMissing returned error: %v", err)
	}

	previewDir := filepath.Join(cfg.ThumbnailsDir, file.ID)
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatalf("MkdirAll preview dir returned error: %v", err)
	}
	spritePath := filepath.Join(previewDir, "sprite_00000.jpg")
	vttPath := filepath.Join(previewDir, "thumbnails.vtt")
	if err := os.WriteFile(spritePath, []byte("jpg-bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile sprite returned error: %v", err)
	}
	if err := os.WriteFile(vttPath, []byte("WEBVTT\n\n00:00:00.000 --> 00:00:10.000\nsprite_00000.jpg#xywh=0,0,160,90\n"), 0o644); err != nil {
		t.Fatalf("WriteFile preview vtt returned error: %v", err)
	}
	if err := store.MarkTorrentFilePreviewReady(ctx, file.ID, spritePath, vttPath); err != nil {
		t.Fatalf("MarkTorrentFilePreviewReady returned error: %v", err)
	}

	router := NewRouter(cfg, db, slog.Default())
	cookie := signInForTorrentTest(t, router)

	listResponse := performJSONRequest(router, http.MethodGet, "/api/files/"+file.ID+"/subtitles", "", cookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected subtitle list status %d, got %d: %s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), "/api/subtitles/"+subtitle.ID+"/track.vtt") {
		t.Fatalf("expected subtitle URL in list, got %s", listResponse.Body.String())
	}

	trackResponse := performJSONRequest(router, http.MethodGet, "/api/subtitles/"+subtitle.ID+"/track.vtt", "", cookie)
	if trackResponse.Code != http.StatusOK {
		t.Fatalf("expected subtitle track status %d, got %d: %s", http.StatusOK, trackResponse.Code, trackResponse.Body.String())
	}
	if !strings.Contains(trackResponse.Body.String(), "WEBVTT") {
		t.Fatalf("expected subtitle body, got %q", trackResponse.Body.String())
	}

	vttResponse := performJSONRequest(router, http.MethodGet, "/api/files/"+file.ID+"/preview/thumbnails.vtt", "", cookie)
	if vttResponse.Code != http.StatusOK {
		t.Fatalf("expected preview vtt status %d, got %d: %s", http.StatusOK, vttResponse.Code, vttResponse.Body.String())
	}
	if !strings.Contains(vttResponse.Body.String(), "/api/files/"+file.ID+"/preview/sprite_00000.jpg#xywh=0,0,160,90") {
		t.Fatalf("expected rewritten preview vtt, got %s", vttResponse.Body.String())
	}

	spriteResponse := performJSONRequest(router, http.MethodGet, "/api/files/"+file.ID+"/preview/sprite_00000.jpg", "", cookie)
	if spriteResponse.Code != http.StatusOK {
		t.Fatalf("expected sprite status %d, got %d: %s", http.StatusOK, spriteResponse.Code, spriteResponse.Body.String())
	}
	if spriteResponse.Body.String() != "jpg-bytes" {
		t.Fatalf("expected sprite bytes, got %q", spriteResponse.Body.String())
	}
}

func createPlayableFileForTest(t *testing.T, ctx context.Context, store *torrents.Store, ownerUserID string, hlsRoot string) torrents.TorrentFile {
	t.Helper()

	torrent, err := store.CreateTorrent(ctx, torrents.CreateTorrentParams{
		OwnerUserID: ownerUserID,
		MagnetURI:   testTorrentMagnet,
	})
	if err != nil {
		t.Fatalf("CreateTorrent returned error: %v", err)
	}
	file, _, err := store.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
		TorrentID:    torrent.ID,
		Name:         "movie.mp4",
		Ext:          ".mp4",
		OriginalPath: "/media/originals/movie.mp4",
		SizeBytes:    4096,
	})
	if err != nil {
		t.Fatalf("CreateTorrentFileIfMissing returned error: %v", err)
	}

	outputDir := filepath.Join(hlsRoot, file.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "segment_00000.ts"), []byte("segment-bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile segment returned error: %v", err)
	}
	playlist := "#EXTM3U\n#EXT-X-VERSION:3\n" + filepath.Join(outputDir, "segment_00000.ts") + "\n#EXT-X-ENDLIST\n"
	playlistPath := filepath.Join(outputDir, "index.m3u8")
	if err := os.WriteFile(playlistPath, []byte(playlist), 0o644); err != nil {
		t.Fatalf("WriteFile playlist returned error: %v", err)
	}
	if err := store.MarkTorrentFileDone(ctx, file.ID, playlistPath); err != nil {
		t.Fatalf("MarkTorrentFileDone returned error: %v", err)
	}

	updated, err := store.FindTorrentFileByID(ctx, file.ID)
	if err != nil {
		t.Fatalf("FindTorrentFileByID returned error: %v", err)
	}

	return updated
}

func TestRewriteHLSPlaylistPreservesDirectives(t *testing.T) {
	body := "#EXTM3U\n#EXTINF:6.0,\nsegment_00000.ts\n#EXT-X-ENDLIST\n"
	rewritten := string(rewriteHLSPlaylist("tfi_123", body))
	if !strings.Contains(rewritten, "#EXTINF:6.0,") {
		t.Fatalf("expected directive to be preserved, got:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, "/api/files/tfi_123/hls/segment_00000.ts") {
		t.Fatalf("expected segment to be rewritten, got:\n%s", rewritten)
	}
}

func TestRewriteHLSPlaylistRewritesFMP4Assets(t *testing.T) {
	body := "#EXTM3U\n#EXT-X-MAP:URI=\"/media/hls/tfi_123/init.mp4\"\n#EXTINF:6.0,\n/media/hls/tfi_123/segment_00000.m4s\n#EXT-X-ENDLIST\n"
	rewritten := string(rewriteHLSPlaylist("tfi_123", body))

	if !strings.Contains(rewritten, `#EXT-X-MAP:URI="/api/files/tfi_123/hls/init.mp4"`) {
		t.Fatalf("expected init segment URI to be rewritten, got:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, "/api/files/tfi_123/hls/segment_00000.m4s") {
		t.Fatalf("expected media segment to be rewritten, got:\n%s", rewritten)
	}
	if strings.Contains(rewritten, "/media/hls") {
		t.Fatalf("expected filesystem paths to be removed, got:\n%s", rewritten)
	}
}

func TestSafePathInRootRejectsEmptyRoot(t *testing.T) {
	if _, ok := safePathInRoot("", filepath.Join(t.TempDir(), "asset.vtt")); ok {
		t.Fatalf("expected empty root to be rejected")
	}
}

func TestPlaybackPlaylistResponseIsJSONOnError(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.HLSDir = filepath.Join(t.TempDir(), "hls")
	db := testDB(t)
	if err := auth.NewStore(db).BootstrapAdmin(ctx, cfg); err != nil {
		t.Fatalf("BootstrapAdmin returned error: %v", err)
	}
	router := NewRouter(cfg, db, slog.Default())
	cookie := signInForTorrentTest(t, router)

	response := performJSONRequest(router, http.MethodGet, "/api/files/does-not-exist/hls/index.m3u8", "", cookie)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected not found status %d, got %d", http.StatusNotFound, response.Code)
	}
	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON error response: %v", err)
	}
}

package httpserver

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

const (
	shareDefaultTTLHours = 24
	shareMaxTTLHours     = 90 * 24
)

// ShareHandler serves the public share-link feature: authenticated owners
// create/list/revoke links, and anyone holding a slug can stream the shared
// media without logging in until the link expires.
type ShareHandler struct {
	Store    *torrents.Store
	Playback PlaybackHandler
}

type createShareRequest struct {
	TorrentID      string `json:"torrentId"`
	TorrentFileID  string `json:"torrentFileId"`
	ExpiresInHours int    `json:"expiresInHours"`
}

type shareFile struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	SizeBytes       int64   `json:"sizeBytes"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
	VideoCodec      string  `json:"videoCodec,omitempty"`
	AudioCodec      string  `json:"audioCodec,omitempty"`
	HLSReady        bool    `json:"hlsReady"`
	DirectPlayable  bool    `json:"directPlayable"`
	ProgressPreview bool    `json:"progressPreview"`
}

type shareSummary struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Scope     string `json:"scope"` // "file" or "torrent"
	Title     string `json:"title"`
	URL       string `json:"url"`
	FileCount int    `json:"fileCount"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
	Expired   bool   `json:"expired"`
}

type sharePublicResponse struct {
	Slug      string      `json:"slug"`
	Scope     string      `json:"scope"`
	Title     string      `json:"title"`
	ExpiresAt string      `json:"expiresAt"`
	Files     []shareFile `json:"files"`
}

// Create issues a new share link for a torrent or a single file owned by the
// requesting user.
func (h ShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var request createShareRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	torrentID := strings.TrimSpace(request.TorrentID)
	fileID := strings.TrimSpace(request.TorrentFileID)
	if (torrentID == "") == (fileID == "") {
		writeError(w, http.StatusBadRequest, "specify exactly one of torrentId or torrentFileId")
		return
	}

	if fileID != "" {
		file, err := h.Store.FindTorrentFileByIDForOwner(r.Context(), fileID, user.ID)
		if err != nil {
			if errors.Is(err, torrents.ErrNotFound) {
				writeError(w, http.StatusNotFound, "file not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load file")
			return
		}
		if !shareFilePlayable(file) {
			writeError(w, http.StatusConflict, "file is not ready to share")
			return
		}
	} else {
		torrent, err := h.Store.FindTorrentByID(r.Context(), torrentID)
		if err != nil || torrent.OwnerUserID != user.ID {
			if err == nil || errors.Is(err, torrents.ErrNotFound) {
				writeError(w, http.StatusNotFound, "torrent not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load torrent")
			return
		}
		files, err := h.Store.ListTorrentFiles(r.Context(), torrentID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load torrent files")
			return
		}
		if !anyShareFilePlayable(files) {
			writeError(w, http.StatusConflict, "torrent has no files ready to share")
			return
		}
	}

	share, err := h.Store.CreateShare(r.Context(), torrents.CreateShareParams{
		OwnerUserID:   user.ID,
		TorrentID:     torrentID,
		TorrentFileID: fileID,
		ExpiresAt:     time.Now().UTC().Add(shareTTL(request.ExpiresInHours)),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create share")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]shareSummary{"share": h.summarize(r, share)})
}

// List returns every share owned by the requesting user.
func (h ShareHandler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	shares, err := h.Store.ListSharesForOwner(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list shares")
		return
	}

	summaries := make([]shareSummary, 0, len(shares))
	for _, share := range shares {
		summaries = append(summaries, h.summarize(r, share))
	}

	writeJSON(w, http.StatusOK, map[string][]shareSummary{"shares": summaries})
}

// Revoke deletes a share link, immediately disabling public access.
func (h ShareHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := h.Store.DeleteShare(r.Context(), chi.URLParam(r, "id"), user.ID); err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "share not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to revoke share")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PublicMetadata resolves a share slug to its playable files without requiring
// authentication.
func (h ShareHandler) PublicMetadata(w http.ResponseWriter, r *http.Request) {
	share, ok := h.loadActiveShare(w, r)
	if !ok {
		return
	}

	files, err := h.shareFiles(r.Context(), share)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load shared files")
		return
	}

	scope, title := h.scopeAndTitle(r.Context(), share, files)
	payload := make([]shareFile, 0, len(files))
	for _, file := range files {
		if !shareFilePlayable(file) {
			continue
		}
		payload = append(payload, publicShareFile(file))
	}

	writeJSON(w, http.StatusOK, sharePublicResponse{
		Slug:      share.Slug,
		Scope:     scope,
		Title:     title,
		ExpiresAt: share.ExpiresAt,
		Files:     payload,
	})
}

func (h ShareHandler) ServeHLSPlaylist(w http.ResponseWriter, r *http.Request) {
	share, file, ok := h.loadShareFile(w, r)
	if !ok {
		return
	}
	if !hlsReady(file) {
		writeError(w, http.StatusConflict, "HLS output is not ready")
		return
	}

	playlistPath, ok := h.Playback.safeHLSPath(file.HLSPath)
	if !ok {
		writeError(w, http.StatusNotFound, "HLS playlist not found")
		return
	}

	body, err := os.ReadFile(playlistPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "HLS playlist not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read HLS playlist")
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rewriteShareHLSPlaylist(share.Slug, file.ID, string(body)))
}

func (h ShareHandler) ServeHLSSegment(w http.ResponseWriter, r *http.Request) {
	_, file, ok := h.loadShareFile(w, r)
	if !ok {
		return
	}
	if !hlsReady(file) {
		writeError(w, http.StatusConflict, "HLS output is not ready")
		return
	}

	playlistPath, ok := h.Playback.safeHLSPath(file.HLSPath)
	if !ok {
		writeError(w, http.StatusNotFound, "HLS playlist not found")
		return
	}

	assetName := strings.TrimSpace(chi.URLParam(r, "segment"))
	if !isSafeHLSAssetName(assetName) {
		writeError(w, http.StatusBadRequest, "invalid HLS asset")
		return
	}
	segmentPath, ok := h.Playback.safeHLSPath(filepath.Join(filepath.Dir(playlistPath), assetName))
	if !ok {
		writeError(w, http.StatusNotFound, "HLS asset not found")
		return
	}

	w.Header().Set("Content-Type", hlsAssetContentType(segmentPath))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, segmentPath)
}

func (h ShareHandler) ServeOriginalFile(w http.ResponseWriter, r *http.Request) {
	_, file, ok := h.loadShareFile(w, r)
	if !ok {
		return
	}
	if !file.DirectPlayable {
		writeError(w, http.StatusConflict, "original file is not directly playable")
		return
	}

	originalPath, ok := h.Playback.safeOriginalPath(file.OriginalPath)
	if !ok {
		writeError(w, http.StatusNotFound, "original file not found")
		return
	}
	if _, err := os.Stat(originalPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "original file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load original file")
		return
	}

	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(originalPath)))
	if contentType == "" {
		contentType = "video/mp4"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, originalPath)
}

func (h ShareHandler) ListSubtitles(w http.ResponseWriter, r *http.Request) {
	share, file, ok := h.loadShareFile(w, r)
	if !ok {
		return
	}

	subtitles, err := h.Store.ListSubtitlesForFile(r.Context(), file.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list subtitles")
		return
	}
	for index := range subtitles {
		subtitles[index].URL = fmt.Sprintf(
			"/api/shares/view/%s/files/%s/subtitles/%s/track.vtt",
			url.PathEscape(share.Slug),
			url.PathEscape(file.ID),
			url.PathEscape(subtitles[index].ID),
		)
		subtitles[index].Path = ""
	}

	writeJSON(w, http.StatusOK, map[string][]torrents.Subtitle{"subtitles": subtitles})
}

func (h ShareHandler) ServeSubtitleTrack(w http.ResponseWriter, r *http.Request) {
	_, file, ok := h.loadShareFile(w, r)
	if !ok {
		return
	}

	subtitle, err := h.Store.FindSubtitleByID(r.Context(), chi.URLParam(r, "subtitleID"))
	if err != nil || subtitle.TorrentFileID != file.ID {
		if err == nil || errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "subtitle not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load subtitle")
		return
	}
	subtitlePath, ok := h.Playback.safeSubtitlePath(subtitle.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "subtitle file not found")
		return
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, subtitlePath)
}

func (h ShareHandler) ServePreviewVTT(w http.ResponseWriter, r *http.Request) {
	share, file, ok := h.loadShareFile(w, r)
	if !ok {
		return
	}

	vttPath, ok := h.Playback.safeThumbnailPath(file.ThumbnailVTTPath)
	if !ok {
		writeError(w, http.StatusNotFound, "preview thumbnails not found")
		return
	}
	body, err := os.ReadFile(vttPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "preview thumbnails not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read preview thumbnails")
		return
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rewriteSharePreviewVTT(share.Slug, file.ID, string(body)))
}

func (h ShareHandler) ServePreviewAsset(w http.ResponseWriter, r *http.Request) {
	_, file, ok := h.loadShareFile(w, r)
	if !ok {
		return
	}

	vttPath, ok := h.Playback.safeThumbnailPath(file.ThumbnailVTTPath)
	if !ok {
		writeError(w, http.StatusNotFound, "preview thumbnails not found")
		return
	}
	assetName := strings.TrimSpace(chi.URLParam(r, "asset"))
	if !isSafePreviewAssetName(assetName) {
		writeError(w, http.StatusBadRequest, "invalid preview asset")
		return
	}
	assetPath, ok := h.Playback.safeThumbnailPath(filepath.Join(filepath.Dir(vttPath), assetName))
	if !ok {
		writeError(w, http.StatusNotFound, "preview asset not found")
		return
	}

	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(assetPath)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, assetPath)
}

// loadActiveShare resolves the slug URL param to a non-expired share.
func (h ShareHandler) loadActiveShare(w http.ResponseWriter, r *http.Request) (torrents.Share, bool) {
	share, err := h.Store.FindShareBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "share link not found")
			return torrents.Share{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load share")
		return torrents.Share{}, false
	}
	if share.Expired(time.Now()) {
		writeError(w, http.StatusGone, "share link has expired")
		return torrents.Share{}, false
	}
	return share, true
}

// loadShareFile resolves both the share and the requested file, verifying the
// file is actually covered by the share.
func (h ShareHandler) loadShareFile(w http.ResponseWriter, r *http.Request) (torrents.Share, torrents.TorrentFile, bool) {
	share, ok := h.loadActiveShare(w, r)
	if !ok {
		return torrents.Share{}, torrents.TorrentFile{}, false
	}

	file, err := h.Store.FindTorrentFileByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found for share")
			return torrents.Share{}, torrents.TorrentFile{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load file")
		return torrents.Share{}, torrents.TorrentFile{}, false
	}
	if !shareCoversFile(share, file) {
		writeError(w, http.StatusNotFound, "file not found for share")
		return torrents.Share{}, torrents.TorrentFile{}, false
	}

	return share, file, true
}

// shareFiles returns every torrent file the share grants access to.
func (h ShareHandler) shareFiles(ctx context.Context, share torrents.Share) ([]torrents.TorrentFile, error) {
	if share.TorrentFileID != "" {
		file, err := h.Store.FindTorrentFileByID(ctx, share.TorrentFileID)
		if err != nil {
			if errors.Is(err, torrents.ErrNotFound) {
				return []torrents.TorrentFile{}, nil
			}
			return nil, err
		}
		return []torrents.TorrentFile{file}, nil
	}
	return h.Store.ListTorrentFiles(ctx, share.TorrentID)
}

func (h ShareHandler) scopeAndTitle(ctx context.Context, share torrents.Share, files []torrents.TorrentFile) (string, string) {
	if share.TorrentFileID != "" {
		if len(files) > 0 {
			return "file", files[0].Name
		}
		return "file", "Shared file"
	}
	if torrent, err := h.Store.FindTorrentByID(ctx, share.TorrentID); err == nil {
		if name := strings.TrimSpace(torrent.Name); name != "" {
			return "torrent", name
		}
	}
	return "torrent", "Shared torrent"
}

func (h ShareHandler) summarize(r *http.Request, share torrents.Share) shareSummary {
	files, _ := h.shareFiles(r.Context(), share)
	scope, title := h.scopeAndTitle(r.Context(), share, files)
	playable := 0
	for _, file := range files {
		if shareFilePlayable(file) {
			playable++
		}
	}
	return shareSummary{
		ID:        share.ID,
		Slug:      share.Slug,
		Scope:     scope,
		Title:     title,
		URL:       shareURL(r, share.Slug),
		FileCount: playable,
		ExpiresAt: share.ExpiresAt,
		CreatedAt: share.CreatedAt,
		Expired:   share.Expired(time.Now()),
	}
}

func shareCoversFile(share torrents.Share, file torrents.TorrentFile) bool {
	if share.TorrentFileID != "" {
		return file.ID == share.TorrentFileID
	}
	return share.TorrentID != "" && file.TorrentID == share.TorrentID
}

func publicShareFile(file torrents.TorrentFile) shareFile {
	return shareFile{
		ID:              file.ID,
		Name:            file.Name,
		SizeBytes:       file.SizeBytes,
		DurationSeconds: file.DurationSeconds,
		VideoCodec:      file.VideoCodec,
		AudioCodec:      file.AudioCodec,
		HLSReady:        hlsReady(file),
		DirectPlayable:  file.DirectPlayable,
		ProgressPreview: file.ProgressPreview,
	}
}

func shareFilePlayable(file torrents.TorrentFile) bool {
	return file.DirectPlayable || hlsReady(file)
}

func anyShareFilePlayable(files []torrents.TorrentFile) bool {
	for _, file := range files {
		if shareFilePlayable(file) {
			return true
		}
	}
	return false
}

func shareTTL(hours int) time.Duration {
	if hours <= 0 {
		hours = shareDefaultTTLHours
	}
	if hours > shareMaxTTLHours {
		hours = shareMaxTTLHours
	}
	return time.Duration(hours) * time.Hour
}

func shareURL(r *http.Request, slug string) string {
	return fmt.Sprintf("%s://%s/s/%s", requestScheme(r), r.Host, url.PathEscape(slug))
}

func rewriteShareHLSPlaylist(slug string, fileID string, body string) []byte {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		if rewritten, ok := rewriteShareHLSURIAttribute(slug, fileID, line); ok {
			lines[index] = rewritten
			continue
		}
		segmentName, ok := hlsAssetNameFromPlaylistLine(line)
		if !ok {
			continue
		}
		lines[index] = shareHLSAssetURL(slug, fileID, segmentName)
	}
	return []byte(strings.Join(lines, "\n"))
}

func rewriteShareHLSURIAttribute(slug string, fileID string, line string) (string, bool) {
	const marker = `URI="`
	start := strings.Index(line, marker)
	if start < 0 {
		return "", false
	}
	valueStart := start + len(marker)
	valueEnd := strings.Index(line[valueStart:], `"`)
	if valueEnd < 0 {
		return "", false
	}
	valueEnd += valueStart
	assetName, ok := hlsAssetNameFromURI(line[valueStart:valueEnd])
	if !ok {
		return "", false
	}
	return line[:valueStart] + shareHLSAssetURL(slug, fileID, assetName) + line[valueEnd:], true
}

func shareHLSAssetURL(slug string, fileID string, assetName string) string {
	return fmt.Sprintf(
		"/api/shares/view/%s/files/%s/hls/%s",
		url.PathEscape(slug),
		url.PathEscape(fileID),
		url.PathEscape(assetName),
	)
}

func rewriteSharePreviewVTT(slug string, fileID string, body string) []byte {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		assetName, ok := previewAssetNameFromVTTLine(line)
		if !ok {
			continue
		}
		replacement := fmt.Sprintf(
			"/api/shares/view/%s/files/%s/preview/%s",
			url.PathEscape(slug),
			url.PathEscape(fileID),
			url.PathEscape(assetName),
		)
		lines[index] = strings.Replace(line, assetName, replacement, 1)
	}
	return []byte(strings.Join(lines, "\n"))
}

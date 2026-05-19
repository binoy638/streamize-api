package httpserver

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

type PlaybackHandler struct {
	Store         *torrents.Store
	OriginalsDir  string
	HLSDir        string
	SubtitlesDir  string
	ThumbnailsDir string
}

func (h PlaybackHandler) ServeHLSPlaylist(w http.ResponseWriter, r *http.Request) {
	file, ok := h.loadPlayableFile(w, r)
	if !ok {
		return
	}

	playlistPath, ok := h.safeHLSPath(file.HLSPath)
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
	_, _ = w.Write(rewriteHLSPlaylist(file.ID, string(body)))
}

func (h PlaybackHandler) ServeHLSSegment(w http.ResponseWriter, r *http.Request) {
	file, ok := h.loadPlayableFile(w, r)
	if !ok {
		return
	}

	playlistPath, ok := h.safeHLSPath(file.HLSPath)
	if !ok {
		writeError(w, http.StatusNotFound, "HLS playlist not found")
		return
	}

	assetName := strings.TrimSpace(chi.URLParam(r, "segment"))
	if !isSafeHLSAssetName(assetName) {
		writeError(w, http.StatusBadRequest, "invalid HLS asset")
		return
	}

	segmentPath, ok := h.safeHLSPath(filepath.Join(filepath.Dir(playlistPath), assetName))
	if !ok {
		writeError(w, http.StatusNotFound, "HLS asset not found")
		return
	}

	contentType := hlsAssetContentType(segmentPath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, segmentPath)
}

func (h PlaybackHandler) ServeOriginalFile(w http.ResponseWriter, r *http.Request) {
	file, ok := h.loadOwnedFile(w, r)
	if !ok {
		return
	}
	if !file.DirectPlayable {
		writeError(w, http.StatusConflict, "original file is not directly playable")
		return
	}

	originalPath, ok := h.safeOriginalPath(file.OriginalPath)
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

func (h PlaybackHandler) ListSubtitles(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	subtitles, err := h.Store.ListSubtitlesForFileOwner(r.Context(), chi.URLParam(r, "id"), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list subtitles")
		return
	}
	for index := range subtitles {
		subtitles[index].URL = fmt.Sprintf("/api/subtitles/%s/track.vtt", url.PathEscape(subtitles[index].ID))
		subtitles[index].Path = ""
	}

	writeJSON(w, http.StatusOK, map[string][]torrents.Subtitle{"subtitles": subtitles})
}

func (h PlaybackHandler) ServeSubtitleTrack(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	subtitle, err := h.Store.FindSubtitleByIDForOwner(r.Context(), chi.URLParam(r, "id"), user.ID)
	if err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "subtitle not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load subtitle")
		return
	}

	subtitlePath, ok := h.safeSubtitlePath(subtitle.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "subtitle file not found")
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, subtitlePath)
}

func (h PlaybackHandler) ServePreviewVTT(w http.ResponseWriter, r *http.Request) {
	file, ok := h.loadOwnedFile(w, r)
	if !ok {
		return
	}
	vttPath, ok := h.safeThumbnailPath(file.ThumbnailVTTPath)
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
	_, _ = w.Write(rewritePreviewVTT(file.ID, string(body)))
}

func (h PlaybackHandler) ServePreviewAsset(w http.ResponseWriter, r *http.Request) {
	file, ok := h.loadOwnedFile(w, r)
	if !ok {
		return
	}
	vttPath, ok := h.safeThumbnailPath(file.ThumbnailVTTPath)
	if !ok {
		writeError(w, http.StatusNotFound, "preview thumbnails not found")
		return
	}

	assetName := strings.TrimSpace(chi.URLParam(r, "asset"))
	if !isSafePreviewAssetName(assetName) {
		writeError(w, http.StatusBadRequest, "invalid preview asset")
		return
	}
	assetPath, ok := h.safeThumbnailPath(filepath.Join(filepath.Dir(vttPath), assetName))
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

func (h PlaybackHandler) loadOwnedFile(w http.ResponseWriter, r *http.Request) (torrents.TorrentFile, bool) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return torrents.TorrentFile{}, false
	}

	file, err := h.Store.FindTorrentFileByIDForOwner(r.Context(), chi.URLParam(r, "id"), user.ID)
	if err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return torrents.TorrentFile{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load file")
		return torrents.TorrentFile{}, false
	}
	return file, true
}

func (h PlaybackHandler) loadPlayableFile(w http.ResponseWriter, r *http.Request) (torrents.TorrentFile, bool) {
	file, ok := h.loadOwnedFile(w, r)
	if !ok {
		return torrents.TorrentFile{}, false
	}

	hlsReady := strings.TrimSpace(file.HLSPath) != "" &&
		(file.Status == torrents.FileStatusDone || file.Status == torrents.FileStatusProcessing)
	if !hlsReady {
		writeError(w, http.StatusConflict, "HLS output is not ready")
		return torrents.TorrentFile{}, false
	}

	return file, true
}

func (h PlaybackHandler) safeHLSPath(candidate string) (string, bool) {
	return safePathInRoot(h.HLSDir, candidate)
}

func (h PlaybackHandler) safeOriginalPath(candidate string) (string, bool) {
	return safePathInRoot(h.OriginalsDir, candidate)
}

func (h PlaybackHandler) safeSubtitlePath(candidate string) (string, bool) {
	return safePathInRoot(h.SubtitlesDir, candidate)
}

func (h PlaybackHandler) safeThumbnailPath(candidate string) (string, bool) {
	return safePathInRoot(h.ThumbnailsDir, candidate)
}

func safePathInRoot(rootPath string, candidate string) (string, bool) {
	rootPath = strings.TrimSpace(rootPath)
	candidate = strings.TrimSpace(candidate)
	if rootPath == "" || candidate == "" {
		return "", false
	}

	root, err := filepath.Abs(rootPath)
	if err != nil || root == "" {
		return "", false
	}
	cleaned, err := filepath.Abs(candidate)
	if err != nil || cleaned == "" {
		return "", false
	}

	relative, err := filepath.Rel(root, cleaned)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}

	return cleaned, true
}

func rewriteHLSPlaylist(fileID string, body string) []byte {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		if rewritten, ok := rewriteHLSURIAttribute(fileID, line); ok {
			lines[index] = rewritten
			continue
		}

		segmentName, ok := hlsAssetNameFromPlaylistLine(line)
		if !ok {
			continue
		}

		lines[index] = fmt.Sprintf("/api/files/%s/hls/%s", url.PathEscape(fileID), url.PathEscape(segmentName))
	}

	return []byte(strings.Join(lines, "\n"))
}

func rewriteHLSURIAttribute(fileID string, line string) (string, bool) {
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

	replacement := fmt.Sprintf("/api/files/%s/hls/%s", url.PathEscape(fileID), url.PathEscape(assetName))
	return line[:valueStart] + replacement + line[valueEnd:], true
}

func hlsAssetNameFromPlaylistLine(line string) (string, bool) {
	line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if line == "" || strings.HasPrefix(line, "#") {
		return "", false
	}

	return hlsAssetNameFromURI(line)
}

func hlsAssetNameFromURI(uri string) (string, bool) {
	if parsed, err := url.Parse(strings.TrimSpace(uri)); err == nil && parsed.Path != "" {
		name := path.Base(parsed.Path)
		if isSafeHLSAssetName(name) {
			return name, true
		}
	}

	name := filepath.Base(uri)
	if !isSafeHLSAssetName(name) {
		return "", false
	}

	return name, true
}

func isSafeHLSAssetName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return false
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return false
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".ts", ".m4s", ".mp4":
		return true
	default:
		return false
	}
}

func rewritePreviewVTT(fileID string, body string) []byte {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		assetName, ok := previewAssetNameFromVTTLine(line)
		if !ok {
			continue
		}
		lines[index] = strings.Replace(line, assetName, fmt.Sprintf("/api/files/%s/preview/%s", url.PathEscape(fileID), url.PathEscape(assetName)), 1)
	}
	return []byte(strings.Join(lines, "\n"))
}

func previewAssetNameFromVTTLine(line string) (string, bool) {
	line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "WEBVTT") || strings.Contains(line, "-->") {
		return "", false
	}
	beforeFragment, _, _ := strings.Cut(line, "#")
	name := filepath.Base(beforeFragment)
	if !isSafePreviewAssetName(name) {
		return "", false
	}
	return name, true
}

func isSafePreviewAssetName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return false
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return false
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}

func hlsAssetContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ts":
		return "video/mp2t"
	case ".m4s":
		return "video/iso.segment"
	case ".mp4":
		return "video/mp4"
	default:
		contentType := mime.TypeByExtension(filepath.Ext(name))
		if contentType == "" {
			return "application/octet-stream"
		}
		return contentType
	}
}

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
	Store  *torrents.Store
	HLSDir string
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

func (h PlaybackHandler) loadPlayableFile(w http.ResponseWriter, r *http.Request) (torrents.TorrentFile, bool) {
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
	if file.Status != torrents.FileStatusDone || strings.TrimSpace(file.HLSPath) == "" {
		writeError(w, http.StatusConflict, "HLS output is not ready")
		return torrents.TorrentFile{}, false
	}

	return file, true
}

func (h PlaybackHandler) safeHLSPath(candidate string) (string, bool) {
	root, err := filepath.Abs(strings.TrimSpace(h.HLSDir))
	if err != nil || root == "" {
		return "", false
	}
	cleaned, err := filepath.Abs(strings.TrimSpace(candidate))
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

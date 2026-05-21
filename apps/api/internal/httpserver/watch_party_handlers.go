package httpserver

import (
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

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/config"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
	"github.com/binoy638/streamize-api/apps/api/internal/watchparty"
)

const watchPartyTTL = 24 * time.Hour

type WatchPartyHandler struct {
	Store        *watchparty.Store
	TorrentStore *torrents.Store
	AuthStore    *auth.Store
	Config       config.Config
	Playback     PlaybackHandler
	Hub          *watchparty.Hub
}

type createWatchPartyRequest struct {
	TorrentFileID string `json:"torrentFileId"`
	ControlMode   string `json:"controlMode"`
	DisplayName   string `json:"displayName"`
}

type joinWatchPartyRequest struct {
	DisplayName string `json:"displayName"`
}

type watchPartySession struct {
	Participant watchparty.Participant `json:"participant"`
	Token       string                 `json:"token"`
}

type watchPartyFile struct {
	ID              string  `json:"id"`
	TorrentID       string  `json:"torrentId"`
	Name            string  `json:"name"`
	Ext             string  `json:"ext"`
	SizeBytes       int64   `json:"sizeBytes"`
	Status          string  `json:"status"`
	ProgressPreview bool    `json:"progressPreview"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
	Container       string  `json:"container,omitempty"`
	VideoCodec      string  `json:"videoCodec,omitempty"`
	AudioCodec      string  `json:"audioCodec,omitempty"`
	HLSReady        bool    `json:"hlsReady"`
	DirectPlayable  bool    `json:"directPlayable"`
}

type watchPartyResponse struct {
	Party       watchparty.WatchParty   `json:"party"`
	File        watchPartyFile          `json:"file"`
	JoinURL     string                  `json:"joinUrl"`
	Session     *watchPartySession      `json:"session,omitempty"`
	Participant *watchparty.Participant `json:"participant,omitempty"`
}

func (h WatchPartyHandler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	parties, err := h.Store.ListPartiesByOwner(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list watch parties")
		return
	}

	writeJSON(w, http.StatusOK, map[string][]watchparty.WatchParty{"watchParties": parties})
}

func (h WatchPartyHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var request createWatchPartyRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	file, err := h.TorrentStore.FindTorrentFileByIDForOwner(r.Context(), request.TorrentFileID, user.ID)
	if err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load file")
		return
	}
	if !watchPartyFilePlayable(file) {
		writeError(w, http.StatusConflict, "file is not ready for watch party playback")
		return
	}

	party, participant, token, err := h.Store.CreateParty(r.Context(), watchparty.CreatePartyParams{
		OwnerUserID:   user.ID,
		TorrentFileID: file.ID,
		ControlMode:   request.ControlMode,
		DisplayName:   firstNonEmpty(request.DisplayName, user.Username),
		ExpiresAt:     time.Now().UTC().Add(watchPartyTTL),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create watch party")
		return
	}

	writeJSON(w, http.StatusCreated, watchPartyResponse{
		Party:   party,
		File:    publicWatchPartyFile(file),
		JoinURL: watchPartyJoinURL(r, party.Slug),
		Session: &watchPartySession{Participant: participant, Token: token},
	})
}

func (h WatchPartyHandler) End(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	party, err := h.Store.EndParty(r.Context(), chi.URLParam(r, "id"), user.ID)
	if err != nil {
		if errors.Is(err, watchparty.ErrNotFound) {
			writeError(w, http.StatusNotFound, "watch party not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to end watch party")
		return
	}
	if h.Hub != nil {
		h.Hub.BroadcastEnded(party)
	}

	writeJSON(w, http.StatusOK, map[string]watchparty.WatchParty{"party": party})
}

func (h WatchPartyHandler) PublicMetadata(w http.ResponseWriter, r *http.Request) {
	party, file, ok := h.loadPublicParty(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, watchPartyResponse{
		Party:   party,
		File:    publicWatchPartyFile(file),
		JoinURL: watchPartyJoinURL(r, party.Slug),
	})
}

func (h WatchPartyHandler) Join(w http.ResponseWriter, r *http.Request) {
	party, file, ok := h.loadPublicParty(w, r)
	if !ok {
		return
	}
	if !party.Joinable(time.Now()) {
		writeError(w, http.StatusGone, "watch party has ended or expired")
		return
	}

	var request joinWatchPartyRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	role := watchparty.RoleGuest
	userID := ""
	if user, ok := h.optionalUser(r); ok && user.ID == party.OwnerUserID {
		role = watchparty.RoleHost
		userID = user.ID
		if strings.TrimSpace(request.DisplayName) == "" {
			request.DisplayName = user.Username
		}
	}

	participant, token, err := h.Store.JoinParty(r.Context(), watchparty.JoinPartyParams{
		PartyID:     party.ID,
		UserID:      userID,
		DisplayName: request.DisplayName,
		Role:        role,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to join watch party")
		return
	}

	writeJSON(w, http.StatusCreated, watchPartyResponse{
		Party:   party,
		File:    publicWatchPartyFile(file),
		JoinURL: watchPartyJoinURL(r, party.Slug),
		Session: &watchPartySession{Participant: participant, Token: token},
	})
}

func (h WatchPartyHandler) WebSocket(w http.ResponseWriter, r *http.Request) {
	party, participant, ok := h.loadPartyParticipant(w, r)
	if !ok {
		return
	}
	if !party.Joinable(time.Now()) {
		writeError(w, http.StatusGone, "watch party has ended or expired")
		return
	}

	h.Hub.ServeHTTP(w, r, watchparty.ConnectionParams{
		Party:       party,
		Participant: participant,
	})
}

func (h WatchPartyHandler) ServeHLSPlaylist(w http.ResponseWriter, r *http.Request) {
	party, participant, file, token, ok := h.loadPartyFile(w, r)
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
	_, _ = w.Write(rewriteWatchPartyHLSPlaylist(party.Slug, file.ID, participant.ID, token, string(body)))
}

func (h WatchPartyHandler) ServeHLSSegment(w http.ResponseWriter, r *http.Request) {
	_, _, file, _, ok := h.loadPartyFile(w, r)
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

func (h WatchPartyHandler) ServeOriginalFile(w http.ResponseWriter, r *http.Request) {
	_, _, file, _, ok := h.loadPartyFile(w, r)
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

func (h WatchPartyHandler) ListSubtitles(w http.ResponseWriter, r *http.Request) {
	party, participant, file, token, ok := h.loadPartyFile(w, r)
	if !ok {
		return
	}

	subtitles, err := h.TorrentStore.ListSubtitlesForFile(r.Context(), file.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list subtitles")
		return
	}
	for index := range subtitles {
		subtitles[index].URL = fmt.Sprintf(
			"/api/watch-parties/join/%s/files/%s/subtitles/%s/track.vtt?participantId=%s&token=%s",
			url.PathEscape(party.Slug),
			url.PathEscape(file.ID),
			url.PathEscape(subtitles[index].ID),
			url.QueryEscape(participant.ID),
			url.QueryEscape(token),
		)
		subtitles[index].Path = ""
	}

	writeJSON(w, http.StatusOK, map[string][]torrents.Subtitle{"subtitles": subtitles})
}

func (h WatchPartyHandler) ServeSubtitleTrack(w http.ResponseWriter, r *http.Request) {
	_, _, file, _, ok := h.loadPartyFile(w, r)
	if !ok {
		return
	}

	subtitle, err := h.TorrentStore.FindSubtitleByID(r.Context(), chi.URLParam(r, "subtitleID"))
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

func (h WatchPartyHandler) ServePreviewVTT(w http.ResponseWriter, r *http.Request) {
	party, participant, file, token, ok := h.loadPartyFile(w, r)
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
	_, _ = w.Write(rewriteWatchPartyPreviewVTT(party.Slug, file.ID, participant.ID, token, string(body)))
}

func (h WatchPartyHandler) ServePreviewAsset(w http.ResponseWriter, r *http.Request) {
	_, _, file, _, ok := h.loadPartyFile(w, r)
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

func (h WatchPartyHandler) loadPublicParty(w http.ResponseWriter, r *http.Request) (watchparty.WatchParty, torrents.TorrentFile, bool) {
	party, err := h.Store.FindPartyBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, watchparty.ErrNotFound) {
			writeError(w, http.StatusNotFound, "watch party not found")
			return watchparty.WatchParty{}, torrents.TorrentFile{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load watch party")
		return watchparty.WatchParty{}, torrents.TorrentFile{}, false
	}

	file, err := h.TorrentStore.FindTorrentFileByID(r.Context(), party.TorrentFileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load party file")
		return watchparty.WatchParty{}, torrents.TorrentFile{}, false
	}

	return party, file, true
}

func (h WatchPartyHandler) loadPartyParticipant(w http.ResponseWriter, r *http.Request) (watchparty.WatchParty, watchparty.Participant, bool) {
	party, _, ok := h.loadPublicParty(w, r)
	if !ok {
		return watchparty.WatchParty{}, watchparty.Participant{}, false
	}
	participant, err := h.Store.FindParticipantByToken(r.Context(), party.ID, r.URL.Query().Get("participantId"), r.URL.Query().Get("token"))
	if err != nil {
		if errors.Is(err, watchparty.ErrForbidden) || errors.Is(err, watchparty.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "watch party participant token is invalid")
			return watchparty.WatchParty{}, watchparty.Participant{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load watch party participant")
		return watchparty.WatchParty{}, watchparty.Participant{}, false
	}
	return party, participant, true
}

func (h WatchPartyHandler) loadPartyFile(w http.ResponseWriter, r *http.Request) (watchparty.WatchParty, watchparty.Participant, torrents.TorrentFile, string, bool) {
	party, file, ok := h.loadPublicParty(w, r)
	if !ok {
		return watchparty.WatchParty{}, watchparty.Participant{}, torrents.TorrentFile{}, "", false
	}
	if !party.Joinable(time.Now()) {
		writeError(w, http.StatusGone, "watch party has ended or expired")
		return watchparty.WatchParty{}, watchparty.Participant{}, torrents.TorrentFile{}, "", false
	}
	fileID := chi.URLParam(r, "id")
	if fileID != file.ID {
		writeError(w, http.StatusNotFound, "file not found for watch party")
		return watchparty.WatchParty{}, watchparty.Participant{}, torrents.TorrentFile{}, "", false
	}
	token := r.URL.Query().Get("token")
	participant, err := h.Store.FindParticipantByToken(r.Context(), party.ID, r.URL.Query().Get("participantId"), token)
	if err != nil {
		if errors.Is(err, watchparty.ErrForbidden) || errors.Is(err, watchparty.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "watch party participant token is invalid")
			return watchparty.WatchParty{}, watchparty.Participant{}, torrents.TorrentFile{}, "", false
		}
		writeError(w, http.StatusInternalServerError, "failed to load watch party participant")
		return watchparty.WatchParty{}, watchparty.Participant{}, torrents.TorrentFile{}, "", false
	}
	return party, participant, file, token, true
}

func (h WatchPartyHandler) optionalUser(r *http.Request) (auth.User, bool) {
	cookie, err := r.Cookie(h.Config.SessionCookieName)
	if err != nil || cookie.Value == "" || h.AuthStore == nil {
		return auth.User{}, false
	}
	user, err := h.AuthStore.FindUserBySessionToken(r.Context(), cookie.Value)
	return user, err == nil
}

func publicWatchPartyFile(file torrents.TorrentFile) watchPartyFile {
	return watchPartyFile{
		ID:              file.ID,
		TorrentID:       file.TorrentID,
		Name:            file.Name,
		Ext:             file.Ext,
		SizeBytes:       file.SizeBytes,
		Status:          file.Status,
		ProgressPreview: file.ProgressPreview,
		DurationSeconds: file.DurationSeconds,
		Container:       file.Container,
		VideoCodec:      file.VideoCodec,
		AudioCodec:      file.AudioCodec,
		HLSReady:        hlsReady(file),
		DirectPlayable:  file.DirectPlayable,
	}
}

func watchPartyFilePlayable(file torrents.TorrentFile) bool {
	return file.DirectPlayable || hlsReady(file)
}

func hlsReady(file torrents.TorrentFile) bool {
	return strings.TrimSpace(file.HLSPath) != "" &&
		(file.Status == torrents.FileStatusDone || file.Status == torrents.FileStatusProcessing)
}

func watchPartyJoinURL(r *http.Request, slug string) string {
	return fmt.Sprintf("%s://%s/watch/%s", requestScheme(r), r.Host, url.PathEscape(slug))
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" || proto == "http" {
		return proto
	}
	return "http"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func rewriteWatchPartyHLSPlaylist(slug string, fileID string, participantID string, token string, body string) []byte {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		if rewritten, ok := rewriteWatchPartyHLSURIAttribute(slug, fileID, participantID, token, line); ok {
			lines[index] = rewritten
			continue
		}
		segmentName, ok := hlsAssetNameFromPlaylistLine(line)
		if !ok {
			continue
		}
		lines[index] = watchPartyHLSAssetURL(slug, fileID, segmentName, participantID, token)
	}
	return []byte(strings.Join(lines, "\n"))
}

func rewriteWatchPartyHLSURIAttribute(slug string, fileID string, participantID string, token string, line string) (string, bool) {
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
	replacement := watchPartyHLSAssetURL(slug, fileID, assetName, participantID, token)
	return line[:valueStart] + replacement + line[valueEnd:], true
}

func watchPartyHLSAssetURL(slug string, fileID string, assetName string, participantID string, token string) string {
	return fmt.Sprintf(
		"/api/watch-parties/join/%s/files/%s/hls/%s?participantId=%s&token=%s",
		url.PathEscape(slug),
		url.PathEscape(fileID),
		url.PathEscape(assetName),
		url.QueryEscape(participantID),
		url.QueryEscape(token),
	)
}

func rewriteWatchPartyPreviewVTT(slug string, fileID string, participantID string, token string, body string) []byte {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		assetName, ok := previewAssetNameFromVTTLine(line)
		if !ok {
			continue
		}
		replacement := fmt.Sprintf(
			"/api/watch-parties/join/%s/files/%s/preview/%s?participantId=%s&token=%s",
			url.PathEscape(slug),
			url.PathEscape(fileID),
			url.PathEscape(assetName),
			url.QueryEscape(participantID),
			url.QueryEscape(token),
		)
		lines[index] = strings.Replace(line, assetName, replacement, 1)
	}
	return []byte(strings.Join(lines, "\n"))
}

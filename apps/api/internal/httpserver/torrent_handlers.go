package httpserver

import (
	"context"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/qbittorrent"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

type TorrentAdder interface {
	AddMagnet(ctx context.Context, magnetURI string, savePath string, paused bool) error
}

type TorrentLister interface {
	ListTorrents(ctx context.Context) ([]qbittorrent.TorrentInfo, error)
}

type TorrentFileLister interface {
	ListTorrentFiles(ctx context.Context, hash string) ([]qbittorrent.TorrentFile, error)
}

type TorrentDeleter interface {
	DeleteTorrent(ctx context.Context, hash string, deleteFiles bool) error
}

type TorrentResumer interface {
	ResumeTorrent(ctx context.Context, hash string) error
}

type FreeDiskBytesFunc func(path string) (int64, error)

type TorrentHandler struct {
	Store         *torrents.Store
	JobStore      *jobs.Store
	Adder         TorrentAdder
	Lister        TorrentLister
	FileLister    TorrentFileLister
	Deleter       TorrentDeleter
	Resumer       TorrentResumer
	SavePath      string
	Logger        *slog.Logger
	FreeDiskBytes FreeDiskBytesFunc
}

type createTorrentRequest struct {
	MagnetURI string `json:"magnetUri"`
	Name      string `json:"name"`
}

type torrentResponse struct {
	Torrent torrents.Torrent `json:"torrent"`
}

type torrentsResponse struct {
	Torrents []torrents.Torrent `json:"torrents"`
}

type torrentFilesResponse struct {
	Files []torrents.TorrentFile `json:"files"`
}

func (h TorrentHandler) ListTorrents(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	records, err := h.Store.ListTorrents(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list torrents")
		return
	}

	records = h.syncTorrentStates(r.Context(), records)
	records = h.preflightPendingTorrents(r.Context(), user, records)
	h.ingestCompletedTorrentFiles(r.Context(), records)

	writeJSON(w, http.StatusOK, torrentsResponse{Torrents: records})
}

func (h TorrentHandler) CreateTorrent(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var request createTorrentRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	torrent, err := h.Store.CreateTorrent(r.Context(), torrents.CreateTorrentParams{
		OwnerUserID: user.ID,
		MagnetURI:   request.MagnetURI,
		Name:        request.Name,
	})
	if err != nil {
		if errors.Is(err, torrents.ErrInvalidMagnet) {
			writeError(w, http.StatusBadRequest, "invalid magnet URI")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create torrent")
		return
	}
	h.info(r.Context(), "torrent record created",
		slog.String("torrent_id", torrent.ID),
		slog.String("owner_user_id", user.ID),
		slog.String("info_hash", torrent.InfoHash),
		slog.String("status", torrent.Status),
	)

	preflightEnabled := h.canPreflight()
	if h.Adder != nil {
		h.info(r.Context(), "submitting torrent to qBittorrent",
			slog.String("torrent_id", torrent.ID),
			slog.String("info_hash", torrent.InfoHash),
			slog.Bool("preflight_enabled", preflightEnabled),
		)
		if err := h.Adder.AddMagnet(r.Context(), torrent.MagnetURI, h.SavePath, false); err != nil {
			if markErr := h.Store.MarkTorrentError(r.Context(), torrent.ID, err.Error()); markErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to record torrent submission error")
				return
			}
			h.warn(r.Context(), "qBittorrent torrent submission failed",
				slog.String("torrent_id", torrent.ID),
				slog.String("info_hash", torrent.InfoHash),
				slog.Any("error", err),
			)
			writeError(w, http.StatusBadGateway, "failed to add torrent to qBittorrent")
			return
		}
		h.info(r.Context(), "torrent submitted to qBittorrent",
			slog.String("torrent_id", torrent.ID),
			slog.String("info_hash", torrent.InfoHash),
		)
	}
	if preflightEnabled {
		if err := h.Store.UpdateTorrentTransferState(r.Context(), torrents.UpdateTorrentTransferStateParams{
			ID:     torrent.ID,
			Status: torrents.StatusQueued,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to queue torrent preflight")
			return
		}
		h.info(r.Context(), "torrent queued for preflight",
			slog.String("torrent_id", torrent.ID),
			slog.String("info_hash", torrent.InfoHash),
		)
		torrent, err = h.Store.FindTorrentByID(r.Context(), torrent.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load queued torrent")
			return
		}
	}

	writeJSON(w, http.StatusCreated, torrentResponse{Torrent: torrent})
}

func (h TorrentHandler) ListTorrentFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	record, err := h.Store.FindTorrentByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "torrent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load torrent")
		return
	}
	if record.OwnerUserID != user.ID {
		writeError(w, http.StatusNotFound, "torrent not found")
		return
	}

	if record.Status == torrents.StatusDone {
		if err := h.ingestTorrentFiles(r.Context(), record); err != nil {
			h.warn(r.Context(), "failed to refresh completed torrent files", slog.String("torrent_id", record.ID), slog.Any("error", err))
		}
	}

	files, err := h.Store.ListTorrentFiles(r.Context(), record.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list torrent files")
		return
	}

	writeJSON(w, http.StatusOK, torrentFilesResponse{Files: files})
}

func (h TorrentHandler) DeleteTorrent(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	record, err := h.Store.FindTorrentByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "torrent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load torrent")
		return
	}
	if record.OwnerUserID != user.ID {
		writeError(w, http.StatusNotFound, "torrent not found")
		return
	}

	if h.Deleter != nil {
		hash := torrentDeleteHash(record)
		if hash != "" {
			h.info(r.Context(), "deleting torrent from qBittorrent",
				slog.String("torrent_id", record.ID),
				slog.String("hash", hash),
				slog.Bool("delete_files", false),
			)
			if err := h.Deleter.DeleteTorrent(r.Context(), hash, false); err != nil {
				h.warn(r.Context(), "qBittorrent torrent delete failed",
					slog.String("torrent_id", record.ID),
					slog.String("hash", hash),
					slog.Any("error", err),
				)
				writeError(w, http.StatusBadGateway, "failed to delete torrent from qBittorrent")
				return
			}
		}
	}

	if err := h.Store.DeleteTorrent(r.Context(), id, user.ID); err != nil {
		if errors.Is(err, torrents.ErrNotFound) {
			writeError(w, http.StatusNotFound, "torrent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete torrent")
		return
	}
	h.info(r.Context(), "torrent record deleted",
		slog.String("torrent_id", record.ID),
		slog.String("owner_user_id", user.ID),
	)

	w.WriteHeader(http.StatusNoContent)
}

func torrentDeleteHash(record torrents.Torrent) string {
	if hash := strings.TrimSpace(record.QBittorrentHash); hash != "" {
		return hash
	}

	return strings.TrimSpace(record.InfoHash)
}

func (h TorrentHandler) syncTorrentStates(ctx context.Context, records []torrents.Torrent) []torrents.Torrent {
	if h.Lister == nil || len(records) == 0 {
		return records
	}

	qbtTorrents, err := h.Lister.ListTorrents(ctx)
	if err != nil {
		h.warn(ctx, "failed to sync torrents from qBittorrent", slog.Any("error", err))
		return records
	}
	h.debug(ctx, "loaded qBittorrent torrents for sync",
		slog.Int("record_count", len(records)),
		slog.Int("qbittorrent_count", len(qbtTorrents)),
	)

	byHash := make(map[string]qbittorrent.TorrentInfo, len(qbtTorrents))
	for _, qbtTorrent := range qbtTorrents {
		hash := normalizeTorrentHash(qbtTorrent.Hash)
		if hash == "" {
			continue
		}
		byHash[hash] = qbtTorrent
	}

	var changed bool
	for _, record := range records {
		qbtTorrent, ok := findQBTorrent(record, byHash)
		if !ok {
			continue
		}

		status, errorMessage := torrentStatusFromQBTorrent(qbtTorrent)
		progressPercent := qbtProgressPercent(qbtTorrent.Progress)
		if h.canPreflight() && shouldPreflightTorrent(record) {
			status = torrents.StatusQueued
			errorMessage = ""
			progressPercent = 0
		}
		if err := h.Store.UpdateTorrentTransferState(ctx, torrents.UpdateTorrentTransferStateParams{
			ID:                 record.ID,
			QBittorrentHash:    qbtTorrent.Hash,
			Name:               qbtTorrent.Name,
			SizeBytes:          qbtTorrent.Size,
			Status:             status,
			ProgressPercent:    progressPercent,
			DownloadSpeedBytes: qbtTorrent.DownloadSpeed,
			UploadSpeedBytes:   qbtTorrent.UploadSpeed,
			ETASeconds:         qbtETA(qbtTorrent.ETA),
			Peers:              qbtPeers(qbtTorrent),
			Ratio:              qbtTorrent.Ratio,
			ErrorMessage:       errorMessage,
		}); err != nil {
			h.warn(ctx, "failed to update torrent transfer state", slog.String("torrent_id", record.ID), slog.Any("error", err))
			continue
		}
		h.debug(ctx, "torrent transfer state synced",
			slog.String("torrent_id", record.ID),
			slog.String("status", status),
			slog.Float64("progress_percent", progressPercent),
			slog.Int64("size_bytes", qbtTorrent.Size),
			slog.Int64("download_speed_bytes", qbtTorrent.DownloadSpeed),
			slog.Int("peers", qbtPeers(qbtTorrent)),
		)
		changed = true
	}

	if !changed {
		return records
	}

	refreshed, err := h.Store.ListTorrents(ctx, records[0].OwnerUserID)
	if err != nil {
		h.warn(ctx, "failed to reload synced torrent records", slog.Any("error", err))
		return records
	}

	return refreshed
}

func findQBTorrent(record torrents.Torrent, byHash map[string]qbittorrent.TorrentInfo) (qbittorrent.TorrentInfo, bool) {
	for _, hash := range torrentHashKeys(record.QBittorrentHash, record.InfoHash) {
		if qbtTorrent, ok := byHash[hash]; ok {
			return qbtTorrent, true
		}
	}

	return qbittorrent.TorrentInfo{}, false
}

func torrentHashKeys(values ...string) []string {
	keys := make([]string, 0, len(values)*2)
	seen := make(map[string]struct{}, len(values)*2)
	for _, value := range values {
		hash := normalizeTorrentHash(value)
		if hash == "" {
			continue
		}

		if _, ok := seen[hash]; !ok {
			keys = append(keys, hash)
			seen[hash] = struct{}{}
		}

		decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(hash))
		if err != nil || len(decoded) != 20 {
			continue
		}
		hexHash := hex.EncodeToString(decoded)
		if _, ok := seen[hexHash]; !ok {
			keys = append(keys, hexHash)
			seen[hexHash] = struct{}{}
		}
	}

	return keys
}

func normalizeTorrentHash(hash string) string {
	return strings.ToLower(strings.TrimSpace(hash))
}

func torrentStatusFromQBTorrent(qbtTorrent qbittorrent.TorrentInfo) (string, string) {
	state := strings.ToLower(strings.TrimSpace(qbtTorrent.State))
	progress := qbtProgressPercent(qbtTorrent.Progress)
	if progress >= 99.95 {
		return torrents.StatusDone, ""
	}

	switch state {
	case "downloading", "metadl", "stalleddl", "forceddl", "checkingdl", "checkingresumedata", "allocating":
		return torrents.StatusDownloading, ""
	case "pauseddl", "pausedup", "stoppeddl", "stoppedup":
		return torrents.StatusPaused, ""
	case "queueddl", "queuedup":
		return torrents.StatusQueued, ""
	case "uploading", "stalledup", "forcedup", "checkingup", "moving":
		return torrents.StatusDone, ""
	case "error", "missingfiles", "unknown":
		return torrents.StatusError, fmt.Sprintf("qBittorrent state: %s", qbtTorrent.State)
	default:
		if progress > 0 {
			return torrents.StatusDownloading, ""
		}
		return torrents.StatusAdded, ""
	}
}

func qbtProgressPercent(progress float64) float64 {
	if progress <= 0 {
		return 0
	}
	if progress >= 1 {
		return 100
	}

	return progress * 100
}

func qbtPeers(qbtTorrent qbittorrent.TorrentInfo) int {
	seeds := qbtTorrent.NumSeeds
	if seeds < 0 {
		seeds = 0
	}
	leechs := qbtTorrent.NumLeechs
	if leechs < 0 {
		leechs = 0
	}

	return seeds + leechs
}

func qbtETA(eta int64) int64 {
	if eta < 0 || eta >= 8640000 {
		return -1
	}

	return eta
}

func (h TorrentHandler) warn(ctx context.Context, message string, attrs ...slog.Attr) {
	if h.Logger == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	h.Logger.LogAttrs(ctx, slog.LevelWarn, message, attrs...)
}

func (h TorrentHandler) info(ctx context.Context, message string, attrs ...slog.Attr) {
	if h.Logger == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	h.Logger.LogAttrs(ctx, slog.LevelInfo, message, attrs...)
}

func (h TorrentHandler) debug(ctx context.Context, message string, attrs ...slog.Attr) {
	if h.Logger == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	h.Logger.LogAttrs(ctx, slog.LevelDebug, message, attrs...)
}

package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/qbittorrent"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

const torrentPreflightDiskReserveBytes int64 = 1 << 30

var supportedVideoExtensions = map[string]struct{}{
	".avi":  {},
	".m2ts": {},
	".m4v":  {},
	".mkv":  {},
	".mov":  {},
	".mp4":  {},
	".mpeg": {},
	".mpg":  {},
	".ts":   {},
	".webm": {},
}

func (h TorrentHandler) canPreflight() bool {
	return h.Lister != nil && h.FileLister != nil && h.Deleter != nil && h.Resumer != nil
}

func (h TorrentHandler) preflightPendingTorrents(ctx context.Context, user auth.User, records []torrents.Torrent) []torrents.Torrent {
	if !h.canPreflight() || len(records) == 0 {
		return records
	}

	var changed bool
	for _, record := range records {
		if record.OwnerUserID != user.ID || !shouldPreflightTorrent(record) {
			continue
		}

		recordChanged, err := h.preflightTorrent(ctx, user, record)
		if err != nil {
			h.warn(ctx, "failed to preflight torrent", slog.String("torrent_id", record.ID), slog.Any("error", err))
			changed = changed || recordChanged
			continue
		}
		changed = changed || recordChanged
	}

	if !changed {
		return records
	}

	refreshed, err := h.Store.ListTorrents(ctx, user.ID)
	if err != nil {
		h.warn(ctx, "failed to reload preflighted torrent records", slog.Any("error", err))
		return records
	}

	return refreshed
}

func shouldPreflightTorrent(record torrents.Torrent) bool {
	if record.ErrorMessage != "" || record.ProgressPercent > 0.01 {
		return false
	}

	switch record.Status {
	case torrents.StatusAdded, torrents.StatusQueued, torrents.StatusPaused:
		return true
	default:
		return false
	}
}

func (h TorrentHandler) preflightTorrent(ctx context.Context, user auth.User, record torrents.Torrent) (bool, error) {
	h.debug(ctx, "torrent preflight started",
		slog.String("torrent_id", record.ID),
		slog.String("info_hash", record.InfoHash),
		slog.String("status", record.Status),
	)
	qbtTorrent, ok, err := h.findCurrentQBTorrent(ctx, record)
	if err != nil || !ok {
		return false, err
	}

	hash := normalizeTorrentHash(qbtTorrent.Hash)
	if hash == "" {
		return false, nil
	}

	files, err := h.FileLister.ListTorrentFiles(ctx, hash)
	if err != nil {
		return false, err
	}
	h.debug(ctx, "torrent preflight metadata loaded",
		slog.String("torrent_id", record.ID),
		slog.String("hash", hash),
		slog.Int64("size_bytes", qbtTorrent.Size),
		slog.Int("file_count", len(files)),
	)
	if len(files) == 0 || qbtTorrent.Size <= 0 {
		return h.queueTorrentForMetadata(ctx, record, qbtTorrent)
	}

	if !hasSupportedVideoFile(files) {
		return h.rejectPreflightTorrent(ctx, record, hash, "torrent does not contain a supported video file")
	}
	if err := h.validateTorrentStorage(user, qbtTorrent.Size); err != nil {
		return h.rejectPreflightTorrent(ctx, record, hash, err.Error())
	}

	if err := h.Resumer.ResumeTorrent(ctx, hash); err != nil {
		return false, err
	}
	h.info(ctx, "torrent preflight passed; resumed qBittorrent torrent",
		slog.String("torrent_id", record.ID),
		slog.String("hash", hash),
		slog.Int64("size_bytes", qbtTorrent.Size),
	)

	if err := h.Store.UpdateTorrentTransferState(ctx, torrents.UpdateTorrentTransferStateParams{
		ID:              record.ID,
		QBittorrentHash: qbtTorrent.Hash,
		Name:            qbtTorrent.Name,
		SizeBytes:       qbtTorrent.Size,
		Status:          torrents.StatusDownloading,
		ProgressPercent: qbtProgressPercent(qbtTorrent.Progress),
		ETASeconds:      qbtETA(qbtTorrent.ETA),
		Peers:           qbtPeers(qbtTorrent),
		Ratio:           qbtTorrent.Ratio,
	}); err != nil {
		return false, err
	}

	return true, nil
}

func (h TorrentHandler) findCurrentQBTorrent(ctx context.Context, record torrents.Torrent) (qbittorrent.TorrentInfo, bool, error) {
	qbtTorrents, err := h.Lister.ListTorrents(ctx)
	if err != nil {
		return qbittorrent.TorrentInfo{}, false, err
	}

	byHash := make(map[string]qbittorrent.TorrentInfo, len(qbtTorrents))
	for _, qbtTorrent := range qbtTorrents {
		hash := normalizeTorrentHash(qbtTorrent.Hash)
		if hash != "" {
			byHash[hash] = qbtTorrent
		}
	}

	qbtTorrent, ok := findQBTorrent(record, byHash)
	return qbtTorrent, ok, nil
}

func (h TorrentHandler) queueTorrentForMetadata(ctx context.Context, record torrents.Torrent, qbtTorrent qbittorrent.TorrentInfo) (bool, error) {
	hash := normalizeTorrentHash(qbtTorrent.Hash)
	if hash != "" {
		if err := h.Resumer.ResumeTorrent(ctx, hash); err != nil {
			return false, err
		}
		h.debug(ctx, "torrent preflight waiting for metadata; ensured qBittorrent torrent is running",
			slog.String("torrent_id", record.ID),
			slog.String("hash", hash),
		)
	}

	if record.Status == torrents.StatusQueued && record.QBittorrentHash != "" {
		return false, nil
	}

	if err := h.Store.UpdateTorrentTransferState(ctx, torrents.UpdateTorrentTransferStateParams{
		ID:              record.ID,
		QBittorrentHash: qbtTorrent.Hash,
		Name:            qbtTorrent.Name,
		SizeBytes:       qbtTorrent.Size,
		Status:          torrents.StatusQueued,
		ProgressPercent: 0,
		ETASeconds:      qbtETA(qbtTorrent.ETA),
		Peers:           qbtPeers(qbtTorrent),
		Ratio:           qbtTorrent.Ratio,
	}); err != nil {
		return false, err
	}

	return true, nil
}

func (h TorrentHandler) rejectPreflightTorrent(ctx context.Context, record torrents.Torrent, hash string, message string) (bool, error) {
	h.warn(ctx, "torrent preflight rejected; deleting qBittorrent torrent files",
		slog.String("torrent_id", record.ID),
		slog.String("hash", hash),
		slog.String("reason", message),
	)
	deleteErr := h.Deleter.DeleteTorrent(ctx, hash, true)
	if err := h.Store.MarkTorrentError(ctx, record.ID, message); err != nil {
		return false, err
	}
	if deleteErr != nil {
		return true, deleteErr
	}

	return true, nil
}

func (h TorrentHandler) validateTorrentStorage(user auth.User, torrentSizeBytes int64) error {
	if torrentSizeBytes <= 0 {
		return nil
	}
	if user.StorageQuotaBytes > 0 && torrentSizeBytes > user.StorageQuotaBytes {
		return fmt.Errorf("torrent exceeds user storage quota")
	}

	freeBytes, err := h.readFreeDiskBytes()
	if err != nil {
		return fmt.Errorf("unable to validate free disk storage")
	}
	requiredBytes := torrentSizeBytes + torrentPreflightDiskReserveBytes
	if freeBytes < requiredBytes {
		return fmt.Errorf("not enough free disk storage for torrent")
	}

	return nil
}

func (h TorrentHandler) readFreeDiskBytes() (int64, error) {
	if h.FreeDiskBytes != nil {
		return h.FreeDiskBytes(h.SavePath)
	}

	return statFreeDiskBytes(h.SavePath)
}

func statFreeDiskBytes(path string) (int64, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}

	available := uint64(stat.Bavail) * uint64(stat.Bsize)
	if available > uint64(math.MaxInt64) {
		return math.MaxInt64, nil
	}

	return int64(available), nil
}

func hasSupportedVideoFile(files []qbittorrent.TorrentFile) bool {
	for _, file := range files {
		extension := strings.ToLower(filepath.Ext(file.Name))
		if _, ok := supportedVideoExtensions[extension]; ok {
			return true
		}
	}

	return false
}

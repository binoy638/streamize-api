package httpserver

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

func (h TorrentHandler) ingestActiveTorrentFiles(ctx context.Context, records []torrents.Torrent) {
	if h.FileLister == nil || len(records) == 0 {
		return
	}

	for _, record := range records {
		switch record.Status {
		case torrents.StatusDownloading, torrents.StatusQueued, torrents.StatusDone:
		default:
			continue
		}
		if err := h.ingestTorrentFiles(ctx, record); err != nil {
			h.warn(ctx, "failed to ingest torrent files", slog.String("torrent_id", record.ID), slog.Any("error", err))
		}
	}
}

func (h TorrentHandler) ingestTorrentFiles(ctx context.Context, record torrents.Torrent) error {
	if h.FileLister == nil {
		return nil
	}

	hash := torrentDeleteHash(record)
	if hash == "" {
		return nil
	}

	qbtFiles, err := h.FileLister.ListTorrentFiles(ctx, hash)
	if err != nil {
		return err
	}

	isDone := record.Status == torrents.StatusDone

	var created int
	for _, qbtFile := range qbtFiles {
		if !isSupportedVideoFileName(qbtFile.Name) {
			continue
		}

		relativePath, ok := cleanTorrentFilePath(qbtFile.Name)
		if !ok {
			h.warn(ctx, "skipping torrent file with unsafe path", slog.String("torrent_id", record.ID), slog.String("file_name", qbtFile.Name))
			continue
		}

		initialStatus := torrents.FileStatusDownloading
		if isDone {
			initialStatus = torrents.FileStatusQueued
		}

		file, inserted, err := h.Store.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
			TorrentID:    record.ID,
			OwnerUserID:  record.OwnerUserID,
			Name:         relativePath,
			Ext:          strings.ToLower(filepath.Ext(relativePath)),
			OriginalPath: filepath.Join(h.SavePath, relativePath),
			SizeBytes:    qbtFile.Size,
			Status:       initialStatus,
		})
		if err != nil {
			return err
		}
		if inserted {
			created++
		}
		if err := h.ensureMetadataIdentifyJob(ctx, file); err != nil {
			return err
		}

		// Track per-file download progress.
		downloadPercent := qbtFile.Progress * 100
		if isDone {
			downloadPercent = 100
		}
		_ = h.Store.UpdateTorrentFileDownloadProgress(ctx, file.ID, downloadPercent)

		if isDone {
			// Transition files that were ingested while downloading to queued so
			// they can receive an HLS transcode job.
			if !inserted && file.Status == torrents.FileStatusDownloading {
				if err := h.Store.MarkTorrentFileQueued(ctx, file.ID); err != nil {
					return err
				}
				file.Status = torrents.FileStatusQueued
			}
			if file.Status == torrents.FileStatusQueued {
				if err := h.ensureHLSTranscodeJob(ctx, file); err != nil {
					return err
				}
			}
		}
	}

	if created > 0 {
		h.info(ctx, "torrent files ingested",
			slog.String("torrent_id", record.ID),
			slog.String("torrent_status", record.Status),
			slog.Int("created_count", created),
		)
	} else {
		h.debug(ctx, "torrent ingestion found no new files",
			slog.String("torrent_id", record.ID),
			slog.String("torrent_status", record.Status),
			slog.Int("qbittorrent_file_count", len(qbtFiles)),
		)
	}

	return nil
}

func (h TorrentHandler) ensureMetadataIdentifyJob(ctx context.Context, file torrents.TorrentFile) error {
	if h.JobStore == nil {
		return nil
	}

	job, inserted, err := h.JobStore.CreateMetadataIdentifyJobIfMissing(ctx, file.ID)
	if err != nil {
		return err
	}
	if inserted {
		h.info(ctx, "metadata identify job queued",
			slog.String("torrent_file_id", file.ID),
			slog.String("job_id", job.ID),
			slog.String("job_type", jobs.TypeMetadataIdentify),
		)
	}

	return nil
}

func (h TorrentHandler) ensureHLSTranscodeJob(ctx context.Context, file torrents.TorrentFile) error {
	if h.JobStore == nil {
		return nil
	}

	job, inserted, err := h.JobStore.CreateHLSTranscodeJobIfMissing(ctx, file.ID)
	if err != nil {
		return err
	}
	if inserted {
		h.info(ctx, "hls transcode job queued",
			slog.String("torrent_file_id", file.ID),
			slog.String("job_id", job.ID),
			slog.String("job_type", jobs.TypeHLSTranscode),
		)
	}

	return nil
}

func cleanTorrentFilePath(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}

	cleaned := filepath.Clean(name)
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", false
	}

	return cleaned, true
}

func isSupportedVideoFileName(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	_, ok := supportedVideoExtensions[extension]
	return ok
}

package httpserver

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

func (h TorrentHandler) ingestCompletedTorrentFiles(ctx context.Context, records []torrents.Torrent) {
	if h.FileLister == nil || len(records) == 0 {
		return
	}

	for _, record := range records {
		if record.Status != torrents.StatusDone {
			continue
		}
		if err := h.ingestTorrentFiles(ctx, record); err != nil {
			h.warn(ctx, "failed to ingest completed torrent files", slog.String("torrent_id", record.ID), slog.Any("error", err))
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

		file, inserted, err := h.Store.CreateTorrentFileIfMissing(ctx, torrents.CreateTorrentFileParams{
			TorrentID:    record.ID,
			Name:         relativePath,
			Ext:          strings.ToLower(filepath.Ext(relativePath)),
			OriginalPath: filepath.Join(h.SavePath, relativePath),
			SizeBytes:    qbtFile.Size,
		})
		if err != nil {
			return err
		}
		if inserted {
			created++
		}
		if file.Status == torrents.FileStatusQueued {
			if err := h.ensureHLSTranscodeJob(ctx, file); err != nil {
				return err
			}
		}
	}

	if created > 0 {
		h.info(ctx, "completed torrent files ingested",
			slog.String("torrent_id", record.ID),
			slog.Int("created_count", created),
		)
	} else {
		h.debug(ctx, "completed torrent ingestion found no new files",
			slog.String("torrent_id", record.ID),
			slog.Int("qbittorrent_file_count", len(qbtFiles)),
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

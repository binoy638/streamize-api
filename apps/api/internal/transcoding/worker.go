package transcoding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

type Worker struct {
	Jobs          *jobs.Store
	Torrents      *torrents.Store
	Prober        Prober
	Transcoder    Transcoder
	Assets        AssetProcessor
	HLSDir        string
	SubtitlesDir  string
	ThumbnailsDir string
	ID            string
	PollInterval  time.Duration
	Lease         time.Duration
	RetryDelay    time.Duration
	Logger        *slog.Logger
}

func (w Worker) Run(ctx context.Context) {
	pollInterval := w.PollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	w.info(ctx, "media worker started",
		slog.String("worker_id", w.workerID()),
		slog.Duration("poll_interval", pollInterval),
	)
	defer w.info(ctx, "media worker stopped", slog.String("worker_id", w.workerID()))

	for {
		processed, err := w.ProcessNext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.warn(ctx, "media worker processing failed", slog.Any("error", err))
		}
		if processed {
			continue
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (w Worker) ProcessNext(ctx context.Context) (bool, error) {
	if w.Jobs == nil || w.Torrents == nil {
		return false, fmt.Errorf("media worker stores are required")
	}

	job, ok, err := w.Jobs.ClaimNext(ctx, w.workerID(), w.leaseDuration())
	if err != nil || !ok {
		return ok, err
	}

	if err := w.processJob(ctx, job); err != nil {
		return true, err
	}

	return true, nil
}

func (w Worker) processJob(ctx context.Context, job jobs.Job) error {
	var payload jobs.HLSTranscodePayload
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		if failErr := w.Jobs.Fail(ctx, job.ID, "invalid hls transcode payload", 0); failErr != nil {
			return failErr
		}
		return err
	}

	file, err := w.Torrents.FindTorrentFileByID(ctx, payload.TorrentFileID)
	if err != nil {
		if failErr := w.Jobs.Fail(ctx, job.ID, err.Error(), 0); failErr != nil {
			return failErr
		}
		return err
	}

	if file.Status == torrents.FileStatusDone && strings.TrimSpace(file.HLSPath) != "" {
		return w.Jobs.Complete(ctx, job.ID)
	}
	if strings.TrimSpace(file.OriginalPath) == "" {
		message := "torrent file original path is empty"
		_ = w.Torrents.MarkTorrentFileError(ctx, file.ID, message)
		if failErr := w.Jobs.Fail(ctx, job.ID, message, 0); failErr != nil {
			return failErr
		}
		return fmt.Errorf("%s", message)
	}

	targetDir := filepath.Join(w.HLSDir, file.ID)
	playlistPath := filepath.Join(targetDir, "index.m3u8")
	segmentPattern := filepath.Join(targetDir, "segment_%05d.m4s")
	if err := os.RemoveAll(targetDir); err != nil {
		return w.failJob(ctx, job, file, fmt.Errorf("clear hls output directory: %w", err))
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return w.failJob(ctx, job, file, fmt.Errorf("create hls output directory: %w", err))
	}

	// Set hls_path and mark processing before transcoding begins so the live
	// event playlist can be served as soon as ffmpeg writes the first segment.
	if err := w.Torrents.MarkTorrentFileProcessingWithHLS(ctx, file.ID, playlistPath); err != nil {
		return w.failJob(ctx, job, file, err)
	}

	prober := w.Prober
	if prober == nil {
		prober = FFprobeProber{}
	}
	mediaInfo, err := prober.Probe(ctx, file.OriginalPath)
	if err != nil {
		return w.failJob(ctx, job, file, err)
	}
	plan, err := PlanHLS(mediaInfo)
	if err != nil {
		return w.failJob(ctx, job, file, err)
	}
	if err := w.Torrents.UpdateTorrentFileMediaMetadata(ctx, torrentFileMediaMetadata(file.ID, mediaInfo, plan)); err != nil {
		return w.failJob(ctx, job, file, err)
	}

	w.info(ctx, "hls transcode started",
		slog.String("job_id", job.ID),
		slog.String("torrent_file_id", file.ID),
		slog.String("input_path", file.OriginalPath),
		slog.String("output_playlist", playlistPath),
		slog.String("processing_mode", plan.Mode),
	)

	transcoder := w.Transcoder
	if transcoder == nil {
		transcoder = FFmpegTranscoder{}
	}
	progressReporter := w.progressReporter(ctx, file.ID)
	if err := transcoder.TranscodeHLS(ctx, file.OriginalPath, playlistPath, segmentPattern, plan, progressReporter); err != nil {
		return w.failJob(ctx, job, file, err)
	}

	// Rewrite the ffmpeg EVENT playlist to VOD now that all segments are present.
	if err := finalizeHLSPlaylist(playlistPath); err != nil {
		w.warn(ctx, "finalize hls playlist failed",
			slog.String("torrent_file_id", file.ID),
			slog.String("playlist_path", playlistPath),
			slog.Any("error", err),
		)
		// Non-fatal: the file is fully transcoded; the EVENT header is still playable.
	}

	if err := w.Torrents.MarkTorrentFileDone(ctx, file.ID, playlistPath); err != nil {
		return w.failJob(ctx, job, file, err)
	}

	w.processMediaAssets(ctx, file, mediaInfo)

	if err := w.Jobs.Complete(ctx, job.ID); err != nil {
		return err
	}

	w.info(ctx, "hls transcode completed",
		slog.String("job_id", job.ID),
		slog.String("torrent_file_id", file.ID),
		slog.String("output_playlist", playlistPath),
		slog.String("processing_mode", plan.Mode),
	)

	return nil
}

func (w Worker) processMediaAssets(ctx context.Context, file torrents.TorrentFile, mediaInfo MediaInfo) {
	processor := w.Assets
	if processor == nil {
		processor = FFmpegAssetProcessor{}
	}

	result, err := processor.ProcessMediaAssets(ctx, MediaAssetRequest{
		InputPath:     file.OriginalPath,
		TorrentFileID: file.ID,
		SubtitlesDir:  w.SubtitlesDir,
		ThumbnailsDir: w.ThumbnailsDir,
		Info:          mediaInfo,
	})
	if err != nil {
		w.warn(ctx, "media asset processing failed",
			slog.String("torrent_file_id", file.ID),
			slog.Any("error", err),
		)
		return
	}

	for _, subtitle := range result.Subtitles {
		if _, _, err := w.Torrents.CreateSubtitleIfMissing(ctx, torrents.CreateSubtitleParams{
			TorrentFileID: file.ID,
			FileName:      subtitle.FileName,
			Title:         subtitle.Title,
			Language:      subtitle.Language,
			Path:          subtitle.Path,
		}); err != nil {
			w.warn(ctx, "record subtitle failed",
				slog.String("torrent_file_id", file.ID),
				slog.String("path", subtitle.Path),
				slog.Any("error", err),
			)
		}
	}

	if strings.TrimSpace(result.ThumbnailSheetPath) != "" && strings.TrimSpace(result.ThumbnailVTTPath) != "" {
		if err := w.Torrents.MarkTorrentFilePreviewReady(ctx, file.ID, result.ThumbnailSheetPath, result.ThumbnailVTTPath); err != nil {
			w.warn(ctx, "record thumbnail preview failed",
				slog.String("torrent_file_id", file.ID),
				slog.Any("error", err),
			)
		}
	}
}

func torrentFileMediaMetadata(id string, mediaInfo MediaInfo, plan HLSPlan) torrents.UpdateTorrentFileMediaMetadataParams {
	params := torrents.UpdateTorrentFileMediaMetadataParams{
		ID:              id,
		Container:       mediaInfo.Container,
		DurationSeconds: mediaInfo.DurationSeconds,
		ProcessingMode:  plan.Mode,
	}
	if mediaInfo.Video != nil {
		params.VideoCodec = mediaInfo.Video.CodecName
	}
	if mediaInfo.Audio != nil {
		params.AudioCodec = mediaInfo.Audio.CodecName
	}
	return params
}

func (w Worker) progressReporter(ctx context.Context, torrentFileID string) ProgressReporter {
	var lastPercent float64
	var lastReportedAt time.Time

	return func(percent float64) error {
		if percent >= 100 {
			percent = 99.5
		}
		if percent < 0 {
			percent = 0
		}
		if percent <= lastPercent {
			return nil
		}

		now := time.Now()
		if percent-lastPercent < 1 && !lastReportedAt.IsZero() && now.Sub(lastReportedAt) < 5*time.Second {
			return nil
		}

		lastPercent = percent
		lastReportedAt = now
		return w.Torrents.UpdateTorrentFileTranscodingProgress(ctx, torrentFileID, percent)
	}
}

func (w Worker) failJob(ctx context.Context, job jobs.Job, file torrents.TorrentFile, err error) error {
	message := err.Error()
	if markErr := w.Torrents.MarkTorrentFileError(ctx, file.ID, message); markErr != nil {
		return markErr
	}
	if failErr := w.Jobs.Fail(ctx, job.ID, message, w.retryDelay()); failErr != nil {
		return failErr
	}

	w.warn(ctx, "hls transcode failed",
		slog.String("job_id", job.ID),
		slog.String("torrent_file_id", file.ID),
		slog.Any("error", err),
	)

	return err
}

func (w Worker) workerID() string {
	if id := strings.TrimSpace(w.ID); id != "" {
		return id
	}

	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "media-worker"
	}

	return "media-worker-" + hostname
}

func (w Worker) leaseDuration() time.Duration {
	if w.Lease > 0 {
		return w.Lease
	}

	return 30 * time.Minute
}

func (w Worker) retryDelay() time.Duration {
	if w.RetryDelay > 0 {
		return w.RetryDelay
	}

	return 30 * time.Second
}

func (w Worker) info(ctx context.Context, message string, attrs ...slog.Attr) {
	if w.Logger == nil {
		return
	}
	w.Logger.LogAttrs(ctx, slog.LevelInfo, message, attrs...)
}

func (w Worker) warn(ctx context.Context, message string, attrs ...slog.Attr) {
	if w.Logger == nil {
		return
	}
	w.Logger.LogAttrs(ctx, slog.LevelWarn, message, attrs...)
}

// finalizeHLSPlaylist rewrites #EXT-X-PLAYLIST-TYPE:EVENT to VOD in the
// m3u8 produced by ffmpeg's event mode, signalling to clients that the
// playlist is complete and all segments are available for random-access.
func finalizeHLSPlaylist(playlistPath string) error {
	content, err := os.ReadFile(playlistPath)
	if err != nil {
		return err
	}

	updated := bytes.ReplaceAll(content,
		[]byte("#EXT-X-PLAYLIST-TYPE:EVENT"),
		[]byte("#EXT-X-PLAYLIST-TYPE:VOD"),
	)

	if bytes.Equal(content, updated) {
		return nil
	}

	return os.WriteFile(playlistPath, updated, 0o644)
}

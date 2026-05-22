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

	"github.com/binoy638/streamize-api/apps/api/internal/catalog"
	"github.com/binoy638/streamize-api/apps/api/internal/jobs"
	"github.com/binoy638/streamize-api/apps/api/internal/metadata"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

type Worker struct {
	Jobs          *jobs.Store
	Torrents      *torrents.Store
	Catalog       *catalog.Store
	Metadata      metadata.Resolver
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

	// Recover jobs a previous worker left running when it crashed or the server
	// restarted mid-job, so they are retried instead of being stuck forever.
	if w.Jobs != nil {
		if recovered, err := w.Jobs.RequeueStale(ctx, w.workerID()); err != nil {
			if !errors.Is(err, context.Canceled) {
				w.warn(ctx, "recover stale jobs failed", slog.Any("error", err))
			}
		} else if recovered > 0 {
			w.info(ctx, "recovered stale jobs from a previous run", slog.Int64("count", recovered))
		}
	}

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
	// Keep the job's lease fresh for as long as it runs so a slow task is not
	// mistaken for a crashed worker and reclaimed mid-flight. The heartbeat is
	// stopped the moment the job returns.
	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	defer stopHeartbeat()
	go w.renewLeaseUntilDone(heartbeatCtx, job.ID)

	switch job.Type {
	case jobs.TypeMetadataIdentify:
		return w.processMetadataIdentifyJob(ctx, job)
	case jobs.TypeHLSTranscode:
		return w.processHLSTranscodeJob(ctx, job)
	case jobs.TypeSubtitleExtract:
		return w.processSubtitleExtractJob(ctx, job)
	case jobs.TypeSpriteGenerate:
		return w.processSpriteGenerateJob(ctx, job)
	default:
		message := "unsupported media job type"
		if failErr := w.Jobs.Fail(ctx, job.ID, message, 0); failErr != nil {
			return failErr
		}
		return fmt.Errorf("%s: %s", message, job.Type)
	}
}

func (w Worker) processMetadataIdentifyJob(ctx context.Context, job jobs.Job) error {
	payload, err := decodeMediaFilePayload(job)
	if err != nil {
		if failErr := w.Jobs.FailPermanent(ctx, job.ID, "invalid metadata identify payload"); failErr != nil {
			return failErr
		}
		return err
	}
	if w.Catalog == nil {
		message := "catalog store is not configured"
		if failErr := w.Jobs.FailPermanent(ctx, job.ID, message); failErr != nil {
			return failErr
		}
		return fmt.Errorf("%s", message)
	}

	file, err := w.Torrents.FindTorrentFileByID(ctx, payload.TorrentFileID)
	if err != nil {
		if failErr := w.Jobs.FailPermanent(ctx, job.ID, err.Error()); failErr != nil {
			return failErr
		}
		return err
	}

	match, err := w.Metadata.Resolve(ctx, file.Name)
	if err != nil {
		if errors.Is(err, metadata.ErrNoMatch) {
			if markErr := w.Catalog.MarkFileUnmatched(ctx, file.ID); markErr != nil {
				return markErr
			}
			return w.Jobs.Complete(ctx, job.ID)
		}
		if errors.Is(err, metadata.ErrProviderNotConfigured) {
			if markErr := w.Catalog.MarkFileFailed(ctx, file.ID, err.Error()); markErr != nil {
				return markErr
			}
			return w.Jobs.FailPermanent(ctx, job.ID, err.Error())
		}
		if markErr := w.Catalog.MarkFileFailed(ctx, file.ID, err.Error()); markErr != nil {
			return markErr
		}
		if failErr := w.Jobs.Fail(ctx, job.ID, err.Error(), w.retryDelay()); failErr != nil {
			return failErr
		}
		return err
	}

	item, err := w.Catalog.UpsertItem(ctx, catalog.Item{
		OwnerUserID:    file.OwnerUserID,
		MediaType:      catalogMediaType(match.MediaType),
		Provider:       match.Provider,
		ProviderID:     match.ProviderID,
		Title:          match.Title,
		OriginalTitle:  match.OriginalTitle,
		Overview:       match.Overview,
		ReleaseYear:    match.ReleaseYear,
		PosterURL:      match.PosterURL,
		BackdropURL:    match.BackdropURL,
		MetadataStatus: catalog.MetadataStatusMatched,
	})
	if err != nil {
		return err
	}

	var episodeID string
	if match.Episode != nil {
		episode, err := w.Catalog.UpsertEpisode(ctx, catalog.Episode{
			CatalogItemID:  item.ID,
			Provider:       match.Episode.Provider,
			ProviderID:     match.Episode.ProviderID,
			SeasonNumber:   match.Episode.SeasonNumber,
			EpisodeNumber:  match.Episode.EpisodeNumber,
			AbsoluteNumber: match.Episode.AbsoluteNumber,
			Title:          match.Episode.Title,
			Overview:       match.Episode.Overview,
			AirDate:        match.Episode.AirDate,
			StillURL:       match.Episode.StillURL,
		})
		if err != nil {
			return err
		}
		episodeID = episode.ID
	}

	if err := w.Catalog.LinkFile(ctx, catalog.FileLinkParams{
		TorrentFileID:    file.ID,
		CatalogItemID:    item.ID,
		CatalogEpisodeID: episodeID,
		Status:           catalog.MetadataStatusMatched,
		Confidence:       match.Confidence,
		Provider:         match.Provider,
	}); err != nil {
		return err
	}

	if err := w.Jobs.Complete(ctx, job.ID); err != nil {
		return err
	}
	w.info(ctx, "metadata identify completed",
		slog.String("job_id", job.ID),
		slog.String("torrent_file_id", file.ID),
		slog.String("catalog_item_id", item.ID),
		slog.String("provider", match.Provider),
		slog.Float64("confidence", match.Confidence),
	)
	return nil
}

func (w Worker) processHLSTranscodeJob(ctx context.Context, job jobs.Job) error {
	payload, err := decodeMediaFilePayload(job)
	if err != nil {
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
		if err := w.ensureAssetJobs(ctx, file); err != nil {
			return err
		}
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
	progressReporter := w.hlsProgressReporter(ctx, job.ID, file.ID)
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

	if err := w.ensureAssetJobs(ctx, file); err != nil {
		return w.failJob(ctx, job, file, err)
	}

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

func (w Worker) processSubtitleExtractJob(ctx context.Context, job jobs.Job) error {
	payload, err := decodeMediaFilePayload(job)
	if err != nil {
		if failErr := w.Jobs.Fail(ctx, job.ID, "invalid subtitle extraction payload", 0); failErr != nil {
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
	if strings.TrimSpace(file.OriginalPath) == "" {
		message := "torrent file original path is empty"
		if failErr := w.Jobs.Fail(ctx, job.ID, message, 0); failErr != nil {
			return failErr
		}
		return fmt.Errorf("%s", message)
	}

	prober := w.Prober
	if prober == nil {
		prober = FFprobeProber{}
	}
	mediaInfo, err := prober.Probe(ctx, file.OriginalPath)
	if err != nil {
		return w.failAssetJob(ctx, job, file, err)
	}

	w.info(ctx, "subtitle extraction started",
		slog.String("job_id", job.ID),
		slog.String("torrent_file_id", file.ID),
	)

	subtitles, err := w.assetProcessor().ProcessSubtitles(ctx, MediaAssetRequest{
		InputPath:     file.OriginalPath,
		TorrentFileID: file.ID,
		SubtitlesDir:  w.SubtitlesDir,
		Info:          mediaInfo,
	}, w.jobProgressReporter(ctx, job.ID))
	if err != nil {
		return w.failAssetJob(ctx, job, file, err)
	}

	for _, subtitle := range subtitles {
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

	if err := w.Jobs.Complete(ctx, job.ID); err != nil {
		return err
	}

	w.info(ctx, "subtitle extraction completed",
		slog.String("job_id", job.ID),
		slog.String("torrent_file_id", file.ID),
		slog.Int("subtitle_count", len(subtitles)),
	)

	return nil
}

func (w Worker) processSpriteGenerateJob(ctx context.Context, job jobs.Job) error {
	payload, err := decodeMediaFilePayload(job)
	if err != nil {
		if failErr := w.Jobs.Fail(ctx, job.ID, "invalid sprite generation payload", 0); failErr != nil {
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
	if strings.TrimSpace(file.OriginalPath) == "" {
		message := "torrent file original path is empty"
		if failErr := w.Jobs.Fail(ctx, job.ID, message, 0); failErr != nil {
			return failErr
		}
		return fmt.Errorf("%s", message)
	}

	mediaInfo := MediaInfo{DurationSeconds: file.DurationSeconds}
	if mediaInfo.DurationSeconds <= 0 {
		prober := w.Prober
		if prober == nil {
			prober = FFprobeProber{}
		}
		mediaInfo, err = prober.Probe(ctx, file.OriginalPath)
		if err != nil {
			return w.failAssetJob(ctx, job, file, err)
		}
	}

	w.info(ctx, "sprite generation started",
		slog.String("job_id", job.ID),
		slog.String("torrent_file_id", file.ID),
	)

	sheetPath, vttPath, err := w.assetProcessor().ProcessSprite(ctx, MediaAssetRequest{
		InputPath:     file.OriginalPath,
		TorrentFileID: file.ID,
		ThumbnailsDir: w.ThumbnailsDir,
		Info:          mediaInfo,
	}, w.jobProgressReporter(ctx, job.ID))
	if err != nil {
		return w.failAssetJob(ctx, job, file, err)
	}

	if strings.TrimSpace(sheetPath) != "" && strings.TrimSpace(vttPath) != "" {
		if err := w.Torrents.MarkTorrentFilePreviewReady(ctx, file.ID, sheetPath, vttPath); err != nil {
			return w.failAssetJob(ctx, job, file, err)
		}
	}

	if err := w.Jobs.Complete(ctx, job.ID); err != nil {
		return err
	}

	w.info(ctx, "sprite generation completed",
		slog.String("job_id", job.ID),
		slog.String("torrent_file_id", file.ID),
		slog.String("thumbnail_sheet_path", sheetPath),
	)

	return nil
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

func decodeMediaFilePayload(job jobs.Job) (jobs.MediaFilePayload, error) {
	var payload jobs.MediaFilePayload
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return jobs.MediaFilePayload{}, err
	}
	if strings.TrimSpace(payload.TorrentFileID) == "" {
		return jobs.MediaFilePayload{}, fmt.Errorf("torrent file id cannot be empty")
	}
	return payload, nil
}

func (w Worker) ensureAssetJobs(ctx context.Context, file torrents.TorrentFile) error {
	subtitleJob, subtitleInserted, err := w.Jobs.CreateSubtitleExtractJobIfMissing(ctx, file.ID)
	if err != nil {
		return err
	}
	if subtitleInserted {
		w.info(ctx, "subtitle extraction job queued",
			slog.String("torrent_file_id", file.ID),
			slog.String("job_id", subtitleJob.ID),
			slog.String("job_type", jobs.TypeSubtitleExtract),
		)
	}

	spriteJob, spriteInserted, err := w.Jobs.CreateSpriteGenerateJobIfMissing(ctx, file.ID)
	if err != nil {
		return err
	}
	if !spriteInserted && spriteJob.Status == jobs.StatusSucceeded && !file.ProgressPreview {
		retried, err := w.Jobs.Retry(ctx, spriteJob.ID)
		if err != nil {
			return err
		}
		spriteJob = retried
		spriteInserted = true
	}
	if spriteInserted {
		w.info(ctx, "sprite generation job queued",
			slog.String("torrent_file_id", file.ID),
			slog.String("job_id", spriteJob.ID),
			slog.String("job_type", jobs.TypeSpriteGenerate),
		)
	}

	return nil
}

func (w Worker) assetProcessor() AssetProcessor {
	if w.Assets != nil {
		return w.Assets
	}
	return FFmpegAssetProcessor{}
}

func (w Worker) hlsProgressReporter(ctx context.Context, jobID string, torrentFileID string) ProgressReporter {
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
		if err := w.Torrents.UpdateTorrentFileTranscodingProgress(ctx, torrentFileID, percent); err != nil {
			return err
		}
		return w.Jobs.UpdateProgress(ctx, jobID, percent)
	}
}

func (w Worker) jobProgressReporter(ctx context.Context, jobID string) ProgressReporter {
	var lastPercent float64
	var lastReportedAt time.Time

	return func(percent float64) error {
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		if percent < lastPercent {
			return nil
		}

		now := time.Now()
		if percent-lastPercent < 1 && !lastReportedAt.IsZero() && now.Sub(lastReportedAt) < 2*time.Second {
			return nil
		}

		lastPercent = percent
		lastReportedAt = now
		return w.Jobs.UpdateProgress(ctx, jobID, percent)
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

func (w Worker) failAssetJob(ctx context.Context, job jobs.Job, file torrents.TorrentFile, err error) error {
	message := err.Error()
	if failErr := w.Jobs.Fail(ctx, job.ID, message, w.retryDelay()); failErr != nil {
		return failErr
	}

	w.warn(ctx, "media asset job failed",
		slog.String("job_id", job.ID),
		slog.String("job_type", job.Type),
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

// renewLeaseUntilDone extends the claimed job's lease on a fixed interval so a
// long-running task (a large transcode) is not reclaimed as stale while it is
// still making progress. It exits when ctx is canceled, which the caller does
// as soon as the job finishes.
func (w Worker) renewLeaseUntilDone(ctx context.Context, jobID string) {
	lease := w.leaseDuration()
	interval := lease / 3
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Jobs.RenewLease(ctx, jobID, w.workerID(), lease); err != nil && ctx.Err() == nil {
				w.warn(ctx, "renew job lease failed",
					slog.String("job_id", jobID),
					slog.Any("error", err),
				)
			}
		}
	}
}

func (w Worker) retryDelay() time.Duration {
	if w.RetryDelay > 0 {
		return w.RetryDelay
	}

	return 30 * time.Second
}

func catalogMediaType(value string) string {
	switch strings.TrimSpace(value) {
	case metadata.MediaTypeMovie:
		return catalog.MediaTypeMovie
	case metadata.MediaTypeTV:
		return catalog.MediaTypeTV
	case metadata.MediaTypeAnime:
		return catalog.MediaTypeAnime
	default:
		return catalog.MediaTypeUnknown
	}
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

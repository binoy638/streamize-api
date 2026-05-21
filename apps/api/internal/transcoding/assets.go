package transcoding

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AssetProcessor interface {
	ProcessSubtitles(ctx context.Context, request MediaAssetRequest, onProgress ProgressReporter) ([]SubtitleAsset, error)
	ProcessSprite(ctx context.Context, request MediaAssetRequest, onProgress ProgressReporter) (string, string, error)
}

type MediaAssetRequest struct {
	InputPath     string
	TorrentFileID string
	SubtitlesDir  string
	ThumbnailsDir string
	Info          MediaInfo
}

type MediaAssetResult struct {
	Subtitles          []SubtitleAsset
	ThumbnailSheetPath string
	ThumbnailVTTPath   string
}

type SubtitleAsset struct {
	FileName string
	Title    string
	Language string
	Path     string
}

type subtitleTask struct {
	asset SubtitleAsset
	run   func() error
}

type subtitleTaskResult struct {
	asset SubtitleAsset
	err   error
}

// embeddedSubtitle pairs a planned output asset with the subtitle stream index
// it should be extracted from.
type embeddedSubtitle struct {
	asset       SubtitleAsset
	streamIndex int
}

const subtitleExtractionConcurrency = 2

// spriteFrameConcurrency bounds how many thumbnail frames are extracted in
// parallel. Each extraction is a short keyframe seek, so a handful of workers
// keeps every core busy without thrashing disk I/O.
const spriteFrameConcurrency = 4

// spriteThumbWidth and spriteThumbHeight are the pixel dimensions of a single
// thumbnail cell; the sprite sheet and its VTT cues are derived from them.
const (
	spriteThumbWidth  = 160
	spriteThumbHeight = 90
)

// spriteScaleFilter scales a frame into a single thumbnail cell, letterboxing
// to preserve aspect ratio so every cell is exactly the same size.
const spriteScaleFilter = "scale=160:90:force_original_aspect_ratio=decrease,pad=160:90:(ow-iw)/2:(oh-ih)/2"

type FFmpegAssetProcessor struct {
	Binary string
}

func (p FFmpegAssetProcessor) ProcessMediaAssets(ctx context.Context, request MediaAssetRequest) (MediaAssetResult, error) {
	var result MediaAssetResult
	subtitles, err := p.ProcessSubtitles(ctx, request, nil)
	if err != nil {
		return MediaAssetResult{}, err
	}
	result.Subtitles = subtitles

	sheetPath, vttPath, err := p.ProcessSprite(ctx, request, nil)
	if err != nil {
		return MediaAssetResult{}, err
	}
	result.ThumbnailSheetPath = sheetPath
	result.ThumbnailVTTPath = vttPath

	return result, nil
}

func (p FFmpegAssetProcessor) ProcessSubtitles(ctx context.Context, request MediaAssetRequest, onProgress ProgressReporter) ([]SubtitleAsset, error) {
	binary := strings.TrimSpace(p.Binary)
	if binary == "" {
		binary = "ffmpeg"
	}

	subtitlesDir := strings.TrimSpace(request.SubtitlesDir)
	torrentFileID := strings.TrimSpace(request.TorrentFileID)
	if subtitlesDir == "" || torrentFileID == "" {
		return nil, nil
	}

	targetDir := filepath.Join(subtitlesDir, torrentFileID)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create subtitle output directory: %w", err)
	}

	embedded := embeddedSubtitleTracks(request.Info.Subtitles, targetDir)
	externalTasks, err := p.externalSubtitleTasks(ctx, binary, request, targetDir)
	if err != nil {
		return nil, err
	}

	if len(embedded) == 0 && len(externalTasks) == 0 {
		if err := reportAssetProgress(onProgress, 100); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if err := reportAssetProgress(onProgress, 0); err != nil {
		return nil, err
	}

	assets := make([]SubtitleAsset, 0, len(embedded)+len(externalTasks))

	// Embedded tracks all live in the same source file, so a single ffmpeg
	// pass demuxes it once instead of re-reading it for every track.
	if len(embedded) > 0 {
		extracted, err := extractEmbeddedSubtitles(ctx, binary, request.InputPath, embedded)
		if err != nil {
			return nil, err
		}
		assets = append(assets, extracted...)
	}
	if err := reportAssetProgress(onProgress, 60); err != nil {
		return nil, err
	}

	// External sidecar files have independent inputs, so they still run as
	// separate concurrent tasks.
	if len(externalTasks) > 0 {
		externalAssets, err := runSubtitleTasks(ctx, externalTasks, nil)
		if err != nil {
			return nil, err
		}
		assets = append(assets, externalAssets...)
	}
	if err := reportAssetProgress(onProgress, 100); err != nil {
		return nil, err
	}

	return assets, nil
}

// embeddedSubtitleTracks plans an output asset for every text-based subtitle
// stream in the source file.
func embeddedSubtitleTracks(streams []SubtitleStream, targetDir string) []embeddedSubtitle {
	tracks := make([]embeddedSubtitle, 0, len(streams))
	for _, stream := range streams {
		if !isTextSubtitleCodec(stream.CodecName) {
			continue
		}

		language := subtitleLanguage(stream.Language)
		title := strings.TrimSpace(stream.Title)
		if title == "" {
			title = "Embedded " + strings.ToUpper(language)
		}
		fileName := fmt.Sprintf("embedded_%02d_%s.vtt", stream.Index, sanitizeAssetName(language))
		tracks = append(tracks, embeddedSubtitle{
			asset: SubtitleAsset{
				FileName: fileName,
				Title:    title,
				Language: language,
				Path:     filepath.Join(targetDir, fileName),
			},
			streamIndex: stream.Index,
		})
	}
	return tracks
}

// extractEmbeddedSubtitles demuxes the source file once and writes every
// embedded subtitle track. If the combined pass fails (for example because a
// single track is malformed), it falls back to extracting each track in its
// own pass so one bad stream cannot drop the rest.
func extractEmbeddedSubtitles(ctx context.Context, binary, inputPath string, tracks []embeddedSubtitle) ([]SubtitleAsset, error) {
	args := append(baseMediaArgs(), "-i", inputPath)
	for _, track := range tracks {
		args = append(args, "-map", fmt.Sprintf("0:s:%d", track.streamIndex), track.asset.Path)
	}

	if err := runMediaCommand(ctx, binary, args); err == nil {
		assets := make([]SubtitleAsset, len(tracks))
		for i, track := range tracks {
			assets[i] = track.asset
		}
		return assets, nil
	} else if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Fallback: extract tracks individually so a single malformed stream is
	// isolated from the rest.
	assets := make([]SubtitleAsset, 0, len(tracks))
	for _, track := range tracks {
		err := runMediaCommand(ctx, binary, append(baseMediaArgs(),
			"-i", inputPath,
			"-map", fmt.Sprintf("0:s:%d", track.streamIndex),
			track.asset.Path,
		))
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		assets = append(assets, track.asset)
	}
	return assets, nil
}

func (p FFmpegAssetProcessor) externalSubtitleTasks(ctx context.Context, binary string, request MediaAssetRequest, targetDir string) ([]subtitleTask, error) {
	sourceDir := filepath.Dir(strings.TrimSpace(request.InputPath))
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan subtitle source directory: %w", err)
	}

	tasks := make([]subtitleTask, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !isExternalSubtitleExt(ext) {
			continue
		}

		sourcePath := filepath.Join(sourceDir, name)
		baseName := strings.TrimSuffix(name, filepath.Ext(name))
		language := subtitleLanguage(languageFromSubtitleName(baseName))
		fileName := sanitizeAssetName(baseName) + ".vtt"
		outputPath := filepath.Join(targetDir, fileName)
		externalSourcePath := sourcePath
		externalOutputPath := outputPath
		asset := SubtitleAsset{
			FileName: fileName,
			Title:    strings.TrimSpace(baseName),
			Language: language,
			Path:     outputPath,
		}
		if ext == ".vtt" {
			tasks = append(tasks, subtitleTask{
				asset: asset,
				run: func() error {
					body, err := os.ReadFile(externalSourcePath)
					if err != nil {
						return err
					}
					return os.WriteFile(externalOutputPath, body, 0o644)
				},
			})
			continue
		}

		tasks = append(tasks, subtitleTask{
			asset: asset,
			run: func() error {
				return runMediaCommand(ctx, binary, append(baseMediaArgs(),
					"-i", externalSourcePath,
					externalOutputPath,
				))
			},
		})
	}

	return tasks, nil
}

func (p FFmpegAssetProcessor) ProcessSprite(ctx context.Context, request MediaAssetRequest, onProgress ProgressReporter) (string, string, error) {
	binary := strings.TrimSpace(p.Binary)
	if binary == "" {
		binary = "ffmpeg"
	}

	if request.Info.DurationSeconds <= 0 {
		return "", "", nil
	}

	thumbnailsDir := strings.TrimSpace(request.ThumbnailsDir)
	torrentFileID := strings.TrimSpace(request.TorrentFileID)
	if thumbnailsDir == "" || torrentFileID == "" {
		return "", "", nil
	}

	targetDir := filepath.Join(thumbnailsDir, torrentFileID)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create thumbnail output directory: %w", err)
	}
	if err := reportAssetProgress(onProgress, 5); err != nil {
		return "", "", err
	}

	plan := planSprite(request.Info.DurationSeconds)
	sheetPath := filepath.Join(targetDir, "sprite_00000.jpg")
	vttPath := filepath.Join(targetDir, "thumbnails.vtt")

	// Each thumbnail is captured with a fast input seek instead of decoding
	// the whole file through an fps filter, then the frames are tiled into
	// the sprite sheet. The scratch frames live in a sibling directory that
	// is cleaned up regardless of outcome.
	framesDir := filepath.Join(targetDir, "frames")
	if err := os.RemoveAll(framesDir); err != nil {
		return "", "", fmt.Errorf("reset thumbnail frame directory: %w", err)
	}
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create thumbnail frame directory: %w", err)
	}
	defer os.RemoveAll(framesDir)

	if err := extractSpriteFrames(ctx, binary, request.InputPath, framesDir, plan, onProgress); err != nil {
		return "", "", err
	}

	if err := tileSpriteFrames(ctx, binary, framesDir, sheetPath, plan); err != nil {
		return "", "", err
	}
	if err := reportAssetProgress(onProgress, 95); err != nil {
		return "", "", err
	}

	if err := os.WriteFile(vttPath, []byte(spriteVTT("sprite_00000.jpg", request.Info.DurationSeconds, plan)), 0o644); err != nil {
		return "", "", fmt.Errorf("write thumbnail vtt: %w", err)
	}
	if err := reportAssetProgress(onProgress, 100); err != nil {
		return "", "", err
	}

	return sheetPath, vttPath, nil
}

// extractSpriteFrames captures one thumbnail per cue using fast keyframe
// seeks, running a bounded number of seeks in parallel.
func extractSpriteFrames(ctx context.Context, binary, inputPath, framesDir string, plan spritePlan, onProgress ProgressReporter) error {
	if plan.CueCount <= 0 {
		return nil
	}

	limit := spriteFrameConcurrency
	if limit > plan.CueCount {
		limit = plan.CueCount
	}
	if limit < 1 {
		limit = 1
	}

	// A dedicated context lets the first failure cancel any in-flight ffmpeg
	// processes instead of leaving them to finish wasted work.
	extractCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, limit)
	results := make(chan error, plan.CueCount)
	var wg sync.WaitGroup
	for index := 0; index < plan.CueCount; index++ {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-extractCtx.Done():
				results <- extractCtx.Err()
				return
			}

			timestamp := float64(index) * plan.IntervalSeconds
			results <- runMediaCommand(extractCtx, binary, append(baseMediaArgs(),
				"-ss", formatFilterFloat(timestamp),
				"-i", inputPath,
				"-frames:v", "1",
				"-an", "-sn",
				"-vf", spriteScaleFilter,
				"-q:v", "4",
				spriteFramePath(framesDir, index),
			))
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	completed := 0
	for err := range results {
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		completed++
		percent := 10 + (float64(completed)/float64(plan.CueCount))*80
		if err := reportAssetProgress(onProgress, percent); err != nil {
			return err
		}
	}

	return nil
}

// tileSpriteFrames montages the extracted thumbnail frames into a single
// sprite sheet.
func tileSpriteFrames(ctx context.Context, binary, framesDir, sheetPath string, plan spritePlan) error {
	pattern := filepath.Join(framesDir, "frame_%05d.jpg")
	return runMediaCommand(ctx, binary, append(baseMediaArgs(),
		"-framerate", "1",
		"-start_number", "0",
		"-i", pattern,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("tile=%dx%d", plan.Columns, plan.Rows),
		"-q:v", "4",
		sheetPath,
	))
}

func spriteFramePath(framesDir string, index int) string {
	return filepath.Join(framesDir, fmt.Sprintf("frame_%05d.jpg", index))
}

func runSubtitleTasks(ctx context.Context, tasks []subtitleTask, onProgress ProgressReporter) ([]SubtitleAsset, error) {
	if len(tasks) == 0 {
		if err := reportAssetProgress(onProgress, 100); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err := reportAssetProgress(onProgress, 0); err != nil {
		return nil, err
	}

	limit := subtitleExtractionConcurrency
	if limit <= 0 || limit > len(tasks) {
		limit = len(tasks)
	}

	sem := make(chan struct{}, limit)
	results := make(chan subtitleTaskResult, len(tasks))
	var wg sync.WaitGroup
	for _, task := range tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results <- subtitleTaskResult{asset: task.asset, err: ctx.Err()}
				return
			}

			results <- subtitleTaskResult{asset: task.asset, err: task.run()}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	assets := make([]SubtitleAsset, 0, len(tasks))
	completed := 0
	for result := range results {
		completed++
		if result.err == nil {
			assets = append(assets, result.asset)
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err := reportAssetProgress(onProgress, (float64(completed)/float64(len(tasks)))*100); err != nil {
			return nil, err
		}
	}

	return assets, nil
}

func reportAssetProgress(onProgress ProgressReporter, percent float64) error {
	if onProgress == nil {
		return nil
	}
	return onProgress(percent)
}

// baseMediaArgs returns the ffmpeg global flags shared by every media
// command: overwrite output, never read stdin, and keep logging quiet so
// CombinedOutput does not buffer ffmpeg's progress chatter.
func baseMediaArgs() []string {
	return []string{"-y", "-nostdin", "-hide_banner", "-loglevel", "error"}
}

func runMediaCommand(ctx context.Context, binary string, args []string) error {
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("media command failed: %w: %s", err, trimCommandOutput(string(output)))
	}
	return nil
}

func isTextSubtitleCodec(codec string) bool {
	switch normalizeMediaValue(codec) {
	case "subrip", "ass", "ssa", "webvtt", "mov_text":
		return true
	default:
		return false
	}
}

func isExternalSubtitleExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".srt", ".vtt", ".ass", ".ssa":
		return true
	default:
		return false
	}
}

func subtitleLanguage(language string) string {
	language = sanitizeAssetName(normalizeMediaValue(language))
	if language == "" {
		return "und"
	}
	return language
}

func languageFromSubtitleName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	})
	if len(parts) == 0 {
		return ""
	}

	candidate := strings.ToLower(parts[len(parts)-1])
	if len(candidate) == 2 || len(candidate) == 3 {
		return candidate
	}
	return ""
}

var unsafeAssetNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeAssetName(name string) string {
	name = strings.Trim(strings.TrimSpace(name), ".")
	name = unsafeAssetNamePattern.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return "track"
	}
	return name
}

type spritePlan struct {
	IntervalSeconds float64
	CueCount        int
	Columns         int
	Rows            int
}

func planSprite(durationSeconds float64) spritePlan {
	const minCues = 10
	const maxCues = 200

	if durationSeconds <= 0 {
		return spritePlan{IntervalSeconds: 10, CueCount: 1, Columns: 1, Rows: 1}
	}

	// Target 1 thumbnail per minute of video.
	cueCount := int(durationSeconds / 60)
	if cueCount < minCues {
		cueCount = minCues
	}
	if cueCount > maxCues {
		cueCount = maxCues
	}

	intervalSeconds := durationSeconds / float64(cueCount)

	// Roughly-square grid that fits all cues.
	columns := int(math.Ceil(math.Sqrt(float64(cueCount))))
	rows := int(math.Ceil(float64(cueCount) / float64(columns)))

	return spritePlan{
		IntervalSeconds: intervalSeconds,
		CueCount:        cueCount,
		Columns:         columns,
		Rows:            rows,
	}
}

func formatFilterFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 3, 64), "0"), ".")
}

func spriteVTT(spriteName string, durationSeconds float64, plan spritePlan) string {
	const width = spriteThumbWidth
	const height = spriteThumbHeight

	var builder strings.Builder
	builder.WriteString("WEBVTT\n\n")
	for index := 0; index < plan.CueCount; index++ {
		start := float64(index) * plan.IntervalSeconds
		end := start + plan.IntervalSeconds
		if end > durationSeconds {
			end = durationSeconds
		}
		if end <= start {
			end = start + plan.IntervalSeconds
		}

		x := (index % plan.Columns) * width
		y := (index / plan.Columns) * height
		builder.WriteString(formatVTTTimestamp(start))
		builder.WriteString(" --> ")
		builder.WriteString(formatVTTTimestamp(end))
		builder.WriteByte('\n')
		builder.WriteString(fmt.Sprintf("%s#xywh=%d,%d,%d,%d\n\n", spriteName, x, y, width, height))
	}

	return builder.String()
}

func formatVTTTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	duration := time.Duration(seconds * float64(time.Second))
	hours := int(duration / time.Hour)
	duration -= time.Duration(hours) * time.Hour
	minutes := int(duration / time.Minute)
	duration -= time.Duration(minutes) * time.Minute
	wholeSeconds := int(duration / time.Second)
	duration -= time.Duration(wholeSeconds) * time.Second
	millis := int(duration / time.Millisecond)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, wholeSeconds, millis)
}

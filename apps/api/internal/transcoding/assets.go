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
	"time"
)

type AssetProcessor interface {
	ProcessMediaAssets(ctx context.Context, request MediaAssetRequest) (MediaAssetResult, error)
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

type FFmpegAssetProcessor struct {
	Binary string
}

func (p FFmpegAssetProcessor) ProcessMediaAssets(ctx context.Context, request MediaAssetRequest) (MediaAssetResult, error) {
	binary := strings.TrimSpace(p.Binary)
	if binary == "" {
		binary = "ffmpeg"
	}

	var result MediaAssetResult
	subtitles, err := p.processSubtitles(ctx, binary, request)
	if err != nil {
		return MediaAssetResult{}, err
	}
	result.Subtitles = subtitles

	sheetPath, vttPath, err := p.processSprite(ctx, binary, request)
	if err != nil {
		return MediaAssetResult{}, err
	}
	result.ThumbnailSheetPath = sheetPath
	result.ThumbnailVTTPath = vttPath

	return result, nil
}

func (p FFmpegAssetProcessor) processSubtitles(ctx context.Context, binary string, request MediaAssetRequest) ([]SubtitleAsset, error) {
	subtitlesDir := strings.TrimSpace(request.SubtitlesDir)
	torrentFileID := strings.TrimSpace(request.TorrentFileID)
	if subtitlesDir == "" || torrentFileID == "" {
		return nil, nil
	}

	targetDir := filepath.Join(subtitlesDir, torrentFileID)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create subtitle output directory: %w", err)
	}

	assets := make([]SubtitleAsset, 0)
	for _, stream := range request.Info.Subtitles {
		if !isTextSubtitleCodec(stream.CodecName) {
			continue
		}

		language := subtitleLanguage(stream.Language)
		title := strings.TrimSpace(stream.Title)
		if title == "" {
			title = "Embedded " + strings.ToUpper(language)
		}
		fileName := fmt.Sprintf("embedded_%02d_%s.vtt", stream.Index, sanitizeAssetName(language))
		outputPath := filepath.Join(targetDir, fileName)
		args := []string{
			"-y",
			"-nostdin",
			"-i", request.InputPath,
			"-map", fmt.Sprintf("0:s:%d", stream.Index),
			outputPath,
		}
		if err := runMediaCommand(ctx, binary, args); err != nil {
			continue
		}
		assets = append(assets, SubtitleAsset{
			FileName: fileName,
			Title:    title,
			Language: language,
			Path:     outputPath,
		})
	}

	external, err := p.processExternalSubtitles(ctx, binary, request, targetDir)
	if err != nil {
		return nil, err
	}
	assets = append(assets, external...)

	return assets, nil
}

func (p FFmpegAssetProcessor) processExternalSubtitles(ctx context.Context, binary string, request MediaAssetRequest, targetDir string) ([]SubtitleAsset, error) {
	sourceDir := filepath.Dir(strings.TrimSpace(request.InputPath))
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan subtitle source directory: %w", err)
	}

	assets := make([]SubtitleAsset, 0)
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
		if ext == ".vtt" {
			body, err := os.ReadFile(sourcePath)
			if err != nil {
				continue
			}
			if err := os.WriteFile(outputPath, body, 0o644); err != nil {
				continue
			}
		} else if err := runMediaCommand(ctx, binary, []string{"-y", "-nostdin", "-i", sourcePath, outputPath}); err != nil {
			continue
		}

		assets = append(assets, SubtitleAsset{
			FileName: fileName,
			Title:    strings.TrimSpace(baseName),
			Language: language,
			Path:     outputPath,
		})
	}

	return assets, nil
}

func (p FFmpegAssetProcessor) processSprite(ctx context.Context, binary string, request MediaAssetRequest) (string, string, error) {
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

	plan := planSprite(request.Info.DurationSeconds)
	sheetPath := filepath.Join(targetDir, "sprite_00000.jpg")
	vttPath := filepath.Join(targetDir, "thumbnails.vtt")
	filter := fmt.Sprintf(
		"fps=1/%s,scale=160:90:force_original_aspect_ratio=decrease,pad=160:90:(ow-iw)/2:(oh-ih)/2,tile=%dx%d",
		formatFilterFloat(plan.IntervalSeconds),
		plan.Columns,
		plan.Rows,
	)
	args := []string{
		"-y",
		"-nostdin",
		"-i", request.InputPath,
		"-vf", filter,
		"-frames:v", "1",
		"-q:v", "4",
		sheetPath,
	}
	if err := runMediaCommand(ctx, binary, args); err != nil {
		return "", "", nil
	}

	if err := os.WriteFile(vttPath, []byte(spriteVTT("sprite_00000.jpg", request.Info.DurationSeconds, plan)), 0o644); err != nil {
		return "", "", fmt.Errorf("write thumbnail vtt: %w", err)
	}

	return sheetPath, vttPath, nil
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
	const width = 160
	const height = 90

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

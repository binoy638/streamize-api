package transcoding

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Prober interface {
	Probe(ctx context.Context, inputPath string) (MediaInfo, error)
}

type MediaInfo struct {
	Container       string
	DurationSeconds float64
	BitRate         int64
	Video           *MediaStream
	Audio           *MediaStream
	Subtitles       []SubtitleStream
}

type MediaStream struct {
	CodecName   string
	Profile     string
	PixelFormat string
	Width       int
	Height      int
	Channels    int
	SampleRate  int
	BitRate     int64
	Level       int
}

type SubtitleStream struct {
	Index     int
	CodecName string
	Language  string
	Title     string
}

type FFprobeProber struct {
	Binary string
}

func (p FFprobeProber) Probe(ctx context.Context, inputPath string) (MediaInfo, error) {
	binary := strings.TrimSpace(p.Binary)
	if binary == "" {
		binary = "ffprobe"
	}

	cmd := exec.CommandContext(
		ctx,
		binary,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return MediaInfo{}, fmt.Errorf("ffprobe failed: %w: %s", err, trimCommandOutput(string(exitErr.Stderr)))
		}
		return MediaInfo{}, fmt.Errorf("run ffprobe: %w", err)
	}

	info, err := parseFFprobeOutput(output)
	if err != nil {
		return MediaInfo{}, err
	}
	return info, nil
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType   string `json:"codec_type"`
	CodecName   string `json:"codec_name"`
	Profile     string `json:"profile"`
	PixelFormat string `json:"pix_fmt"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Channels    int    `json:"channels"`
	SampleRate  string `json:"sample_rate"`
	BitRate     string `json:"bit_rate"`
	Level       int    `json:"level"`
	Tags        struct {
		Language string `json:"language"`
		Title    string `json:"title"`
	} `json:"tags"`
}

type ffprobeFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	BitRate    string `json:"bit_rate"`
}

func parseFFprobeOutput(output []byte) (MediaInfo, error) {
	var decoded ffprobeOutput
	if err := json.Unmarshal(output, &decoded); err != nil {
		return MediaInfo{}, fmt.Errorf("decode ffprobe output: %w", err)
	}

	info := MediaInfo{
		Container:       normalizeMediaValue(decoded.Format.FormatName),
		DurationSeconds: parsePositiveFloat(decoded.Format.Duration),
		BitRate:         parsePositiveInt64(decoded.Format.BitRate),
	}

	for _, stream := range decoded.Streams {
		switch strings.ToLower(strings.TrimSpace(stream.CodecType)) {
		case "video":
			if info.Video == nil {
				info.Video = mediaStreamFromFFprobe(stream)
			}
		case "audio":
			if info.Audio == nil {
				info.Audio = mediaStreamFromFFprobe(stream)
			}
		case "subtitle":
			info.Subtitles = append(info.Subtitles, SubtitleStream{
				Index:     len(info.Subtitles),
				CodecName: normalizeMediaValue(stream.CodecName),
				Language:  normalizeMediaValue(stream.Tags.Language),
				Title:     strings.TrimSpace(stream.Tags.Title),
			})
		}
	}

	return info, nil
}

func mediaStreamFromFFprobe(stream ffprobeStream) *MediaStream {
	return &MediaStream{
		CodecName:   normalizeMediaValue(stream.CodecName),
		Profile:     strings.TrimSpace(stream.Profile),
		PixelFormat: normalizeMediaValue(stream.PixelFormat),
		Width:       stream.Width,
		Height:      stream.Height,
		Channels:    stream.Channels,
		SampleRate:  int(parsePositiveInt64(stream.SampleRate)),
		BitRate:     parsePositiveInt64(stream.BitRate),
		Level:       stream.Level,
	}
}

func normalizeMediaValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parsePositiveFloat(value string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func parsePositiveInt64(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

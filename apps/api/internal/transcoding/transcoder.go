package transcoding

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Transcoder interface {
	TranscodeHLS(ctx context.Context, inputPath string, outputPlaylistPath string, segmentPattern string) error
}

type FFmpegTranscoder struct {
	Binary string
}

func (t FFmpegTranscoder) TranscodeHLS(ctx context.Context, inputPath string, outputPlaylistPath string, segmentPattern string) error {
	binary := strings.TrimSpace(t.Binary)
	if binary == "" {
		binary = "ffmpeg"
	}

	cmd := exec.CommandContext(
		ctx,
		binary,
		"-y",
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-c:a", "aac",
		"-f", "hls",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segmentPattern,
		outputPlaylistPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg hls transcode failed: %w: %s", err, trimCommandOutput(string(output)))
	}

	return nil
}

func trimCommandOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) <= 4096 {
		return output
	}

	return output[len(output)-4096:]
}

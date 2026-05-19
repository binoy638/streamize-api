package transcoding

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ProgressReporter func(percent float64) error

type Transcoder interface {
	TranscodeHLS(ctx context.Context, inputPath string, outputPlaylistPath string, segmentPattern string, plan HLSPlan, onProgress ProgressReporter) error
}

type FFmpegTranscoder struct {
	Binary string
}

func (t FFmpegTranscoder) TranscodeHLS(ctx context.Context, inputPath string, outputPlaylistPath string, segmentPattern string, plan HLSPlan, onProgress ProgressReporter) error {
	binary := strings.TrimSpace(t.Binary)
	if binary == "" {
		binary = "ffmpeg"
	}

	cmd := exec.CommandContext(
		ctx,
		binary,
		hlsTranscodeArgs(inputPath, outputPlaylistPath, segmentPattern, plan)...,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create ffmpeg progress pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create ffmpeg log pipe: %w", err)
	}

	state := &ffmpegProgressState{}
	var output lockedOutput

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg hls transcode: %w", err)
	}

	var readers sync.WaitGroup
	readers.Add(2)
	go func() {
		defer readers.Done()
		scanFFmpegLogs(stderr, state, &output)
	}()
	go func() {
		defer readers.Done()
		scanFFmpegProgress(stdout, state, &output, onProgress)
	}()

	waitErr := cmd.Wait()
	readers.Wait()

	if waitErr != nil {
		return fmt.Errorf("ffmpeg hls transcode failed: %w: %s", waitErr, trimCommandOutput(output.String()))
	}
	if err := state.ProgressError(); err != nil {
		return fmt.Errorf("record ffmpeg hls progress: %w", err)
	}

	return nil
}

func hlsTranscodeArgs(inputPath string, outputPlaylistPath string, segmentPattern string, plan HLSPlan) []string {
	plan = normalizeHLSPlan(plan)

	args := []string{
		"-y",
		"-nostdin",
		"-progress", "pipe:1",
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
	}

	switch plan.Mode {
	case HLSModeRemux:
		args = append(args,
			"-c:v", "copy",
			"-c:a", "copy",
		)
	case HLSModeAudioTranscode:
		args = append(args,
			"-c:v", "copy",
			"-c:a", "aac",
			"-ac", "2",
			"-ar", "48000",
			"-b:a", "160k",
		)
	default:
		args = append(args,
			"-c:v", "libx264",
			"-crf", "23",
			"-preset", "veryfast",
			"-pix_fmt", "yuv420p",
			"-sc_threshold", "0",
			"-force_key_frames", "expr:gte(t,n_forced*6)",
			"-c:a", "aac",
			"-ac", "2",
			"-ar", "48000",
			"-b:a", "160k",
		)
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_segment_filename", segmentPattern,
		outputPlaylistPath,
	)

	return args
}

type ffmpegProgressState struct {
	mu          sync.Mutex
	duration    time.Duration
	progressErr error
}

func (s *ffmpegProgressState) SetDuration(duration time.Duration) {
	if duration <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.duration <= 0 {
		s.duration = duration
	}
}

func (s *ffmpegProgressState) Duration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.duration
}

func (s *ffmpegProgressState) SetProgressError(err error) {
	if err == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.progressErr == nil {
		s.progressErr = err
	}
}

func (s *ffmpegProgressState) ProgressError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progressErr
}

type lockedOutput struct {
	mu      sync.Mutex
	builder strings.Builder
}

func (o *lockedOutput) WriteLine(line string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.builder.WriteString(line)
	o.builder.WriteByte('\n')
}

func (o *lockedOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.builder.String()
}

func scanFFmpegLogs(reader io.Reader, state *ffmpegProgressState, output *lockedOutput) {
	scanner := newCommandScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		output.WriteLine(line)
		if duration, ok := parseFFmpegDuration(line); ok {
			state.SetDuration(duration)
		}
	}
}

func scanFFmpegProgress(reader io.Reader, state *ffmpegProgressState, output *lockedOutput, onProgress ProgressReporter) {
	scanner := newCommandScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		output.WriteLine(line)
		if onProgress == nil || state.ProgressError() != nil {
			continue
		}

		outTime, ok := parseFFmpegOutTime(line)
		if !ok {
			continue
		}
		duration := state.Duration()
		if duration <= 0 || outTime <= 0 {
			continue
		}

		percent := (float64(outTime) / float64(duration)) * 100
		if percent > 100 {
			percent = 100
		}
		if err := onProgress(percent); err != nil {
			state.SetProgressError(err)
		}
	}
}

func newCommandScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return scanner
}

var durationPattern = regexp.MustCompile(`Duration:\s*([0-9]+:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?)`)

func parseFFmpegDuration(line string) (time.Duration, bool) {
	matches := durationPattern.FindStringSubmatch(line)
	if len(matches) != 2 {
		return 0, false
	}

	return parseClockDuration(matches[1])
}

func parseFFmpegOutTime(line string) (time.Duration, bool) {
	key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
	if !ok || key != "out_time" {
		return 0, false
	}

	return parseClockDuration(value)
}

func parseClockDuration(value string) (time.Duration, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, false
	}

	hours, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	minutes, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, false
	}

	duration := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds*float64(time.Second))
	return duration, duration >= 0
}

func trimCommandOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) <= 4096 {
		return output
	}

	return output[len(output)-4096:]
}

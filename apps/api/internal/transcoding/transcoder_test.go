package transcoding

import (
	"slices"
	"testing"
	"time"
)

func TestParseFFmpegDuration(t *testing.T) {
	duration, ok := parseFFmpegDuration("  Duration: 00:02:03.50, start: 0.000000, bitrate: 1200 kb/s")
	if !ok {
		t.Fatal("expected duration to parse")
	}

	expected := 2*time.Minute + 3500*time.Millisecond
	if duration != expected {
		t.Fatalf("expected duration %s, got %s", expected, duration)
	}
}

func TestParseFFmpegOutTime(t *testing.T) {
	duration, ok := parseFFmpegOutTime("out_time=00:01:01.250000")
	if !ok {
		t.Fatal("expected out_time to parse")
	}

	expected := time.Minute + 1250*time.Millisecond
	if duration != expected {
		t.Fatalf("expected out_time %s, got %s", expected, duration)
	}
}

func TestHLSTranscodeArgsUseRelativeFMP4InitFilename(t *testing.T) {
	args := hlsTranscodeArgs(
		"/media/originals/movie.mkv",
		"/media/hls/tfi_123/index.m3u8",
		"/media/hls/tfi_123/segment_%05d.m4s",
		HLSPlan{Mode: HLSModeFullTranscode},
	)

	index := slices.Index(args, "-hls_fmp4_init_filename")
	if index < 0 || index+1 >= len(args) {
		t.Fatalf("expected -hls_fmp4_init_filename in args: %#v", args)
	}
	if args[index+1] != "init.mp4" {
		t.Fatalf("expected relative init filename, got %q", args[index+1])
	}
}

func TestHLSTranscodeArgsUseStreamCopyForRemux(t *testing.T) {
	args := hlsTranscodeArgs(
		"/media/originals/movie.mp4",
		"/media/hls/tfi_123/index.m3u8",
		"/media/hls/tfi_123/segment_%05d.m4s",
		HLSPlan{Mode: HLSModeRemux},
	)

	if !hasArgValue(args, "-c:v", "copy") || !hasArgValue(args, "-c:a", "copy") {
		t.Fatalf("expected remux args to copy audio and video: %#v", args)
	}
	if slices.Contains(args, "-force_key_frames") {
		t.Fatalf("expected remux args not to force keyframes: %#v", args)
	}
}

func TestHLSTranscodeArgsCopyVideoAndTranscodeAudio(t *testing.T) {
	args := hlsTranscodeArgs(
		"/media/originals/movie.mkv",
		"/media/hls/tfi_123/index.m3u8",
		"/media/hls/tfi_123/segment_%05d.m4s",
		HLSPlan{Mode: HLSModeAudioTranscode},
	)

	if !hasArgValue(args, "-c:v", "copy") || !hasArgValue(args, "-c:a", "aac") {
		t.Fatalf("expected audio-transcode args to copy video and encode audio: %#v", args)
	}
	if slices.Contains(args, "-force_key_frames") {
		t.Fatalf("expected audio-transcode args not to force keyframes: %#v", args)
	}
}

func TestHLSTranscodeArgsUseFullTranscodeDefaults(t *testing.T) {
	args := hlsTranscodeArgs(
		"/media/originals/movie.mkv",
		"/media/hls/tfi_123/index.m3u8",
		"/media/hls/tfi_123/segment_%05d.m4s",
		HLSPlan{Mode: HLSModeFullTranscode},
	)

	if !hasArgValue(args, "-c:v", "libx264") || !hasArgValue(args, "-c:a", "aac") || !hasArgValue(args, "-crf", "23") {
		t.Fatalf("expected full transcode args to encode video and audio: %#v", args)
	}
	if !slices.Contains(args, "-force_key_frames") {
		t.Fatalf("expected full transcode args to force HLS keyframes: %#v", args)
	}
}

func hasArgValue(args []string, key string, value string) bool {
	for index, arg := range args {
		if arg == key && index+1 < len(args) && args[index+1] == value {
			return true
		}
	}
	return false
}

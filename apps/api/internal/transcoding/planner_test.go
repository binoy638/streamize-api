package transcoding

import "testing"

func TestPlanHLSRemuxesSafeH264AAC(t *testing.T) {
	plan, err := PlanHLS(MediaInfo{
		Video: &MediaStream{CodecName: "h264", PixelFormat: "yuv420p"},
		Audio: &MediaStream{CodecName: "aac"},
	})
	if err != nil {
		t.Fatalf("PlanHLS returned error: %v", err)
	}

	if plan.Mode != HLSModeRemux || !plan.VideoCopy || !plan.AudioCopy {
		t.Fatalf("expected remux plan, got %+v", plan)
	}
}

func TestPlanHLSOnlyTranscodesAudioForSafeVideo(t *testing.T) {
	plan, err := PlanHLS(MediaInfo{
		Video: &MediaStream{CodecName: "h264", PixelFormat: "yuv420p"},
		Audio: &MediaStream{CodecName: "ac3"},
	})
	if err != nil {
		t.Fatalf("PlanHLS returned error: %v", err)
	}

	if plan.Mode != HLSModeAudioTranscode || !plan.VideoCopy || plan.AudioCopy {
		t.Fatalf("expected audio-transcode plan, got %+v", plan)
	}
}

func TestPlanHLSFullyTranscodesUnsupportedVideo(t *testing.T) {
	plan, err := PlanHLS(MediaInfo{
		Video: &MediaStream{CodecName: "hevc", PixelFormat: "yuv420p"},
		Audio: &MediaStream{CodecName: "aac"},
	})
	if err != nil {
		t.Fatalf("PlanHLS returned error: %v", err)
	}

	if plan.Mode != HLSModeFullTranscode || plan.VideoCopy || plan.AudioCopy {
		t.Fatalf("expected full transcode plan, got %+v", plan)
	}
}

func TestPlanHLSFullyTranscodesHighBitDepthH264(t *testing.T) {
	plan, err := PlanHLS(MediaInfo{
		Video: &MediaStream{CodecName: "h264", PixelFormat: "yuv420p10le"},
		Audio: &MediaStream{CodecName: "aac"},
	})
	if err != nil {
		t.Fatalf("PlanHLS returned error: %v", err)
	}

	if plan.Mode != HLSModeFullTranscode {
		t.Fatalf("expected full transcode plan, got %+v", plan)
	}
}

func TestPlanHLSRejectsMissingVideo(t *testing.T) {
	if _, err := PlanHLS(MediaInfo{Audio: &MediaStream{CodecName: "aac"}}); err == nil {
		t.Fatal("expected missing video stream to fail")
	}
}

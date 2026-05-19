package transcoding

import (
	"fmt"
	"strings"
	"testing"
)

func TestSpriteVTTUsesSpriteCoordinates(t *testing.T) {
	// 100s video: cueCount = max(10, floor(100/60)) = 10, interval = 10s, columns = 4
	plan := planSprite(100)
	body := spriteVTT("sprite_00000.jpg", 100, plan)
	if !strings.HasPrefix(body, "WEBVTT") {
		t.Fatalf("expected WEBVTT header, got:\n%s", body)
	}
	// Second cue: start=interval, end=2*interval
	secondStart := formatVTTTimestamp(plan.IntervalSeconds)
	secondEnd := formatVTTTimestamp(plan.IntervalSeconds * 2)
	if !strings.Contains(body, secondStart+" --> "+secondEnd) {
		t.Fatalf("expected second cue timing %s --> %s, got:\n%s", secondStart, secondEnd, body)
	}
	// Second cue is index=1, x = (1 % columns) * 160, y = 0
	expectedX := (1 % plan.Columns) * 160
	if !strings.Contains(body, fmt.Sprintf("sprite_00000.jpg#xywh=%d,0,160,90", expectedX)) {
		t.Fatalf("expected second cell x=%d,y=0, got:\n%s", expectedX, body)
	}
}

func TestPlanSpriteDynamicCount(t *testing.T) {
	// 1-hour video: cueCount = 60, interval = 60s
	plan := planSprite(3600)
	if plan.CueCount != 60 {
		t.Fatalf("expected 60 cues for 1h video, got %d", plan.CueCount)
	}
	if plan.IntervalSeconds != 60 {
		t.Fatalf("expected 60s interval for 1h video, got %f", plan.IntervalSeconds)
	}
	// columns = ceil(sqrt(60)) = 8, rows = ceil(60/8) = 8
	if plan.Columns != 8 || plan.Rows != 8 {
		t.Fatalf("expected 8x8 grid for 60 cues, got %dx%d", plan.Columns, plan.Rows)
	}

	body := spriteVTT("sprite_00000.jpg", 3600, plan)
	// Last cue: index=59, start=59*60=3540, end=3600
	if !strings.Contains(body, "00:59:00.000 --> 01:00:00.000") {
		t.Fatalf("expected final cue to cover end of 1h video, got:\n%s", body)
	}
}

func TestPlanSpriteMinimumCues(t *testing.T) {
	// Short video (2 min) should still get the minimum 10 cues.
	plan := planSprite(120)
	if plan.CueCount != 10 {
		t.Fatalf("expected 10 cues for short video, got %d", plan.CueCount)
	}
	if plan.IntervalSeconds != 12 {
		t.Fatalf("expected 12s interval, got %f", plan.IntervalSeconds)
	}
}

func TestPlanSpriteMaximumCues(t *testing.T) {
	// Very long video should be capped at 200 cues.
	plan := planSprite(36000) // 10 hours
	if plan.CueCount != 200 {
		t.Fatalf("expected 200 cues for 10h video, got %d", plan.CueCount)
	}
}

func TestLanguageFromSubtitleName(t *testing.T) {
	if got := languageFromSubtitleName("movie.en"); got != "en" {
		t.Fatalf("expected en, got %q", got)
	}
	if got := languageFromSubtitleName("movie.forced.eng"); got != "eng" {
		t.Fatalf("expected eng, got %q", got)
	}
}

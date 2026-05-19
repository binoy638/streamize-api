package transcoding

import (
	"strings"
	"testing"
)

func TestSpriteVTTUsesSpriteCoordinates(t *testing.T) {
	body := spriteVTT("sprite_00000.jpg", 35, 10)
	if !strings.HasPrefix(body, "WEBVTT") {
		t.Fatalf("expected WEBVTT header, got:\n%s", body)
	}
	if !strings.Contains(body, "00:00:10.000 --> 00:00:20.000") {
		t.Fatalf("expected second cue timing, got:\n%s", body)
	}
	if !strings.Contains(body, "sprite_00000.jpg#xywh=160,0,160,90") {
		t.Fatalf("expected second cell coordinates, got:\n%s", body)
	}
}

func TestPlanSpriteSpreadsLongVideosAcrossSheet(t *testing.T) {
	plan := planSprite(3600)
	if plan.CueCount != 25 {
		t.Fatalf("expected 25 cues, got %d", plan.CueCount)
	}
	if plan.IntervalSeconds < 143 || plan.IntervalSeconds > 145 {
		t.Fatalf("expected interval around 144s, got %f", plan.IntervalSeconds)
	}

	body := spriteVTT("sprite_00000.jpg", 3600, plan.IntervalSeconds)
	if !strings.Contains(body, "00:57:36.000 --> 01:00:00.000") {
		t.Fatalf("expected final cue to cover end of long video, got:\n%s", body)
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

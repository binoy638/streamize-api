package metadata

import "testing"

func TestParseTVRelease(t *testing.T) {
	candidate := Parse("Family.Guy.S24E15.1080p.WEB.h264-EDITH.mkv")
	if candidate.MediaType != MediaTypeTV {
		t.Fatalf("expected tv type, got %q", candidate.MediaType)
	}
	if candidate.Query != "Family Guy" {
		t.Fatalf("expected title Family Guy, got %q", candidate.Query)
	}
	if candidate.SeasonNumber != 24 || candidate.EpisodeNumber != 15 {
		t.Fatalf("unexpected episode parse: S%dE%d", candidate.SeasonNumber, candidate.EpisodeNumber)
	}
}

func TestParseMovieRelease(t *testing.T) {
	candidate := Parse("Signal.Ridge.2026.1080p.WEB-DL.mkv")
	if candidate.MediaType != MediaTypeMovie {
		t.Fatalf("expected movie type, got %q", candidate.MediaType)
	}
	if candidate.Query != "Signal Ridge" || candidate.Year != 2026 {
		t.Fatalf("unexpected movie parse: %+v", candidate)
	}
}

func TestParseAnimeAbsoluteEpisode(t *testing.T) {
	candidate := Parse("Neon Genesis - 07 [1080p].mkv")
	if candidate.MediaType != MediaTypeAnime {
		t.Fatalf("expected anime type, got %q", candidate.MediaType)
	}
	if candidate.Query != "Neon Genesis" || candidate.AbsoluteEpisode != 7 {
		t.Fatalf("unexpected anime parse: %+v", candidate)
	}
}

func TestTitleScoreIsConservative(t *testing.T) {
	if score := titleScore("Family Guy", "Family Guy", 1999, 0); score < 0.9 {
		t.Fatalf("expected strong exact score, got %f", score)
	}
	if score := titleScore("Family Guy", "Different Show", 2026, 0); score > 0.5 {
		t.Fatalf("expected weak mismatch score, got %f", score)
	}
}

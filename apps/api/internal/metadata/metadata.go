package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	MediaTypeMovie = "movie"
	MediaTypeTV    = "tv"
	MediaTypeAnime = "anime"

	ProviderTMDB    = "tmdb"
	ProviderAniList = "anilist"
)

var (
	ErrNoMatch               = errors.New("metadata: no confident match")
	ErrProviderNotConfigured = errors.New("metadata: provider not configured")
)

type Candidate struct {
	Raw             string
	Query           string
	MediaType       string
	Year            int
	SeasonNumber    int
	EpisodeNumber   int
	AbsoluteEpisode int
}

type Match struct {
	MediaType     string
	Provider      string
	ProviderID    string
	Title         string
	OriginalTitle string
	Overview      string
	ReleaseYear   int
	PosterURL     string
	BackdropURL   string
	Confidence    float64
	Episode       *EpisodeMatch
}

type EpisodeMatch struct {
	Provider       string
	ProviderID     string
	SeasonNumber   int
	EpisodeNumber  int
	AbsoluteNumber int
	Title          string
	Overview       string
	AirDate        string
	StillURL       string
}

type Resolver struct {
	TMDBAPIKey string
	Language   string
	HTTPClient *http.Client
}

func (r Resolver) Resolve(ctx context.Context, name string) (Match, error) {
	candidate := Parse(name)
	if candidate.Query == "" {
		return Match{}, ErrNoMatch
	}

	var lastErr error
	if strings.TrimSpace(r.TMDBAPIKey) != "" {
		match, err := r.resolveTMDB(ctx, candidate)
		if err == nil {
			return match, nil
		}
		lastErr = err
	}

	if candidate.MediaType == MediaTypeAnime {
		match, err := r.resolveAniList(ctx, candidate)
		if err == nil {
			return match, nil
		}
		lastErr = err
	}

	if lastErr != nil && !errors.Is(lastErr, ErrNoMatch) {
		return Match{}, lastErr
	}
	if strings.TrimSpace(r.TMDBAPIKey) == "" && candidate.MediaType != MediaTypeAnime {
		return Match{}, ErrProviderNotConfigured
	}
	return Match{}, ErrNoMatch
}

func Parse(name string) Candidate {
	raw := strings.TrimSpace(name)
	base := filepath.Base(strings.ReplaceAll(raw, "\\", "/"))
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}

	candidate := Candidate{Raw: raw}
	lower := strings.ToLower(base)
	if match := seasonEpisodePattern.FindStringSubmatch(lower); len(match) == 5 {
		candidate.MediaType = MediaTypeTV
		if match[1] != "" {
			candidate.SeasonNumber = atoi(match[1])
			candidate.EpisodeNumber = atoi(match[2])
		} else {
			candidate.SeasonNumber = atoi(match[3])
			candidate.EpisodeNumber = atoi(match[4])
		}
		base = base[:strings.Index(strings.ToLower(base), match[0])]
	} else if match := absoluteEpisodePattern.FindStringSubmatch(base); len(match) == 2 {
		candidate.MediaType = MediaTypeAnime
		candidate.AbsoluteEpisode = atoi(match[1])
		base = base[:strings.Index(base, match[0])]
	}

	if match := yearPattern.FindStringSubmatch(base); len(match) == 2 {
		candidate.Year = atoi(match[1])
		base = base[:strings.Index(base, match[1])]
	}
	if candidate.MediaType == "" {
		if candidate.Year > 0 {
			candidate.MediaType = MediaTypeMovie
		} else {
			candidate.MediaType = MediaTypeTV
		}
	}

	candidate.Query = cleanTitle(base)
	return candidate
}

func (r Resolver) resolveTMDB(ctx context.Context, candidate Candidate) (Match, error) {
	switch candidate.MediaType {
	case MediaTypeMovie:
		return r.searchTMDBMovie(ctx, candidate)
	case MediaTypeTV, MediaTypeAnime:
		return r.searchTMDBTV(ctx, candidate)
	default:
		return Match{}, ErrNoMatch
	}
}

func (r Resolver) searchTMDBMovie(ctx context.Context, candidate Candidate) (Match, error) {
	var result tmdbSearchResponse[tmdbMovieResult]
	values := url.Values{
		"api_key":  {strings.TrimSpace(r.TMDBAPIKey)},
		"query":    {candidate.Query},
		"language": {languageOrDefault(r.Language)},
	}
	if candidate.Year > 0 {
		values.Set("year", strconv.Itoa(candidate.Year))
	}
	if err := r.getJSON(ctx, "https://api.themoviedb.org/3/search/movie?"+values.Encode(), &result); err != nil {
		return Match{}, err
	}
	best, confidence, ok := bestMovie(candidate, result.Results)
	if !ok || confidence < 0.86 {
		return Match{}, ErrNoMatch
	}
	return Match{
		MediaType:     MediaTypeMovie,
		Provider:      ProviderTMDB,
		ProviderID:    strconv.Itoa(best.ID),
		Title:         firstNonEmpty(best.Title, best.OriginalTitle, candidate.Query),
		OriginalTitle: best.OriginalTitle,
		Overview:      best.Overview,
		ReleaseYear:   yearFromDate(best.ReleaseDate),
		PosterURL:     tmdbImageURL(best.PosterPath, "w500"),
		BackdropURL:   tmdbImageURL(best.BackdropPath, "original"),
		Confidence:    confidence,
	}, nil
}

func (r Resolver) searchTMDBTV(ctx context.Context, candidate Candidate) (Match, error) {
	var result tmdbSearchResponse[tmdbTVResult]
	values := url.Values{
		"api_key":  {strings.TrimSpace(r.TMDBAPIKey)},
		"query":    {candidate.Query},
		"language": {languageOrDefault(r.Language)},
	}
	if candidate.Year > 0 {
		values.Set("first_air_date_year", strconv.Itoa(candidate.Year))
	}
	if err := r.getJSON(ctx, "https://api.themoviedb.org/3/search/tv?"+values.Encode(), &result); err != nil {
		return Match{}, err
	}
	best, confidence, ok := bestTV(candidate, result.Results)
	if !ok || confidence < 0.86 {
		return Match{}, ErrNoMatch
	}
	match := Match{
		MediaType:     candidate.MediaType,
		Provider:      ProviderTMDB,
		ProviderID:    strconv.Itoa(best.ID),
		Title:         firstNonEmpty(best.Name, best.OriginalName, candidate.Query),
		OriginalTitle: best.OriginalName,
		Overview:      best.Overview,
		ReleaseYear:   yearFromDate(best.FirstAirDate),
		PosterURL:     tmdbImageURL(best.PosterPath, "w500"),
		BackdropURL:   tmdbImageURL(best.BackdropPath, "original"),
		Confidence:    confidence,
	}
	if candidate.SeasonNumber > 0 && candidate.EpisodeNumber > 0 {
		episode, err := r.getTMDBEpisode(ctx, best.ID, candidate.SeasonNumber, candidate.EpisodeNumber)
		if err == nil {
			match.Episode = &episode
		}
	}
	return match, nil
}

func (r Resolver) getTMDBEpisode(ctx context.Context, seriesID int, season int, episode int) (EpisodeMatch, error) {
	var result tmdbEpisodeResult
	values := url.Values{
		"api_key":  {strings.TrimSpace(r.TMDBAPIKey)},
		"language": {languageOrDefault(r.Language)},
	}
	path := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/season/%d/episode/%d?%s", seriesID, season, episode, values.Encode())
	if err := r.getJSON(ctx, path, &result); err != nil {
		return EpisodeMatch{}, err
	}
	return EpisodeMatch{
		Provider:      ProviderTMDB,
		ProviderID:    strconv.Itoa(result.ID),
		SeasonNumber:  season,
		EpisodeNumber: episode,
		Title:         firstNonEmpty(result.Name, fmt.Sprintf("S%02dE%02d", season, episode)),
		Overview:      result.Overview,
		AirDate:       result.AirDate,
		StillURL:      tmdbImageURL(result.StillPath, "w500"),
	}, nil
}

func (r Resolver) resolveAniList(ctx context.Context, candidate Candidate) (Match, error) {
	query := `query ($search: String) {
		Media(search: $search, type: ANIME) {
			id
			title { romaji english native }
			description(asHtml: false)
			startDate { year }
			coverImage { large }
			bannerImage
		}
	}`
	var response struct {
		Data struct {
			Media struct {
				ID    int `json:"id"`
				Title struct {
					Romaji  string `json:"romaji"`
					English string `json:"english"`
					Native  string `json:"native"`
				} `json:"title"`
				Description string `json:"description"`
				StartDate   struct {
					Year int `json:"year"`
				} `json:"startDate"`
				CoverImage struct {
					Large string `json:"large"`
				} `json:"coverImage"`
				BannerImage string `json:"bannerImage"`
			} `json:"Media"`
		} `json:"data"`
	}
	body, _ := json.Marshal(map[string]any{
		"query":     query,
		"variables": map[string]string{"search": candidate.Query},
	})
	if err := r.postJSON(ctx, "https://graphql.anilist.co", body, &response); err != nil {
		return Match{}, err
	}
	media := response.Data.Media
	if media.ID == 0 {
		return Match{}, ErrNoMatch
	}
	title := firstNonEmpty(media.Title.English, media.Title.Romaji, media.Title.Native, candidate.Query)
	confidence := titleScore(candidate.Query, title, media.StartDate.Year, candidate.Year)
	if confidence < 0.82 {
		return Match{}, ErrNoMatch
	}
	match := Match{
		MediaType:     MediaTypeAnime,
		Provider:      ProviderAniList,
		ProviderID:    strconv.Itoa(media.ID),
		Title:         title,
		OriginalTitle: firstNonEmpty(media.Title.Romaji, media.Title.Native),
		Overview:      stripHTML(media.Description),
		ReleaseYear:   media.StartDate.Year,
		PosterURL:     media.CoverImage.Large,
		BackdropURL:   media.BannerImage,
		Confidence:    confidence,
	}
	if candidate.AbsoluteEpisode > 0 {
		match.Episode = &EpisodeMatch{
			Provider:       ProviderAniList,
			ProviderID:     fmt.Sprintf("%d:%d", media.ID, candidate.AbsoluteEpisode),
			AbsoluteNumber: candidate.AbsoluteEpisode,
			Title:          fmt.Sprintf("Episode %d", candidate.AbsoluteEpisode),
		}
	}
	return match, nil
}

func (r Resolver) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	return r.doJSON(request, target)
}

func (r Resolver) postJSON(ctx context.Context, endpoint string, body []byte, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	return r.doJSON(request, target)
}

func (r Resolver) doJSON(request *http.Request, target any) error {
	client := r.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrProviderNotConfigured
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("metadata provider returned %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

type tmdbSearchResponse[T any] struct {
	Results []T `json:"results"`
}

type tmdbMovieResult struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	Overview      string `json:"overview"`
	ReleaseDate   string `json:"release_date"`
	PosterPath    string `json:"poster_path"`
	BackdropPath  string `json:"backdrop_path"`
}

type tmdbTVResult struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	Overview     string `json:"overview"`
	FirstAirDate string `json:"first_air_date"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

type tmdbEpisodeResult struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Overview  string `json:"overview"`
	AirDate   string `json:"air_date"`
	StillPath string `json:"still_path"`
}

var (
	seasonEpisodePattern   = regexp.MustCompile(`(?i)s(\d{1,2})e(\d{1,3})|(\d{1,2})x(\d{1,3})`)
	absoluteEpisodePattern = regexp.MustCompile(`\s-\s(\d{1,4})(?:\s|$)`)
	yearPattern            = regexp.MustCompile(`\b((?:19|20)\d{2})\b`)
	noisePattern           = regexp.MustCompile(`(?i)\b(480p|720p|1080p|2160p|4k|web[- ]?dl|webrip|bluray|brrip|hdrip|x264|x265|h\.?264|h\.?265|hevc|aac|ac3|eac3|dts|proper|repack|extended|remux)\b`)
	htmlTagPattern         = regexp.MustCompile(`<[^>]+>`)
)

func bestMovie(candidate Candidate, results []tmdbMovieResult) (tmdbMovieResult, float64, bool) {
	sort.SliceStable(results, func(i, j int) bool {
		_, scoreI := scoreMovie(candidate, results[i])
		_, scoreJ := scoreMovie(candidate, results[j])
		return scoreI > scoreJ
	})
	if len(results) == 0 {
		return tmdbMovieResult{}, 0, false
	}
	_, score := scoreMovie(candidate, results[0])
	return results[0], score, true
}

func bestTV(candidate Candidate, results []tmdbTVResult) (tmdbTVResult, float64, bool) {
	sort.SliceStable(results, func(i, j int) bool {
		_, scoreI := scoreTV(candidate, results[i])
		_, scoreJ := scoreTV(candidate, results[j])
		return scoreI > scoreJ
	})
	if len(results) == 0 {
		return tmdbTVResult{}, 0, false
	}
	_, score := scoreTV(candidate, results[0])
	return results[0], score, true
}

func scoreMovie(candidate Candidate, result tmdbMovieResult) (tmdbMovieResult, float64) {
	return result, titleScore(candidate.Query, firstNonEmpty(result.Title, result.OriginalTitle), yearFromDate(result.ReleaseDate), candidate.Year)
}

func scoreTV(candidate Candidate, result tmdbTVResult) (tmdbTVResult, float64) {
	return result, titleScore(candidate.Query, firstNonEmpty(result.Name, result.OriginalName), yearFromDate(result.FirstAirDate), candidate.Year)
}

func titleScore(query string, title string, resultYear int, candidateYear int) float64 {
	q := normalizeComparable(query)
	t := normalizeComparable(title)
	if q == "" || t == "" {
		return 0
	}
	score := tokenSimilarity(q, t)
	if q == t {
		score = 1
	} else if strings.Contains(t, q) || strings.Contains(q, t) {
		score = math.Max(score, 0.88)
	}
	if candidateYear > 0 {
		if resultYear == candidateYear {
			score += 0.08
		} else if resultYear > 0 {
			score -= 0.18
		}
	}
	return math.Max(0, math.Min(1, score))
}

func tokenSimilarity(a string, b string) float64 {
	aTokens := strings.Fields(a)
	bTokens := strings.Fields(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}
	bSet := make(map[string]struct{}, len(bTokens))
	for _, token := range bTokens {
		bSet[token] = struct{}{}
	}
	matches := 0
	for _, token := range aTokens {
		if _, ok := bSet[token]; ok {
			matches++
		}
	}
	return float64(matches) / float64(maxInt(len(aTokens), len(bTokens)))
}

func cleanTitle(value string) string {
	value = strings.ReplaceAll(value, ".", " ")
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = noisePattern.ReplaceAllString(value, " ")
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}

func normalizeComparable(value string) string {
	value = strings.ToLower(cleanTitle(value))
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == ' ' {
			builder.WriteRune(char)
		} else {
			builder.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func tmdbImageURL(path string, size string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return "https://image.tmdb.org/t/p/" + strings.Trim(size, "/") + "/" + strings.TrimLeft(path, "/")
}

func languageOrDefault(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "en-US"
	}
	return value
}

func yearFromDate(value string) int {
	if len(value) < 4 {
		return 0
	}
	return atoi(value[:4])
}

func atoi(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func stripHTML(value string) string {
	return strings.Join(strings.Fields(htmlTagPattern.ReplaceAllString(value, " ")), " ")
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

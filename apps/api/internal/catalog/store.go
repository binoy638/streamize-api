package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
	"github.com/binoy638/streamize-api/apps/api/internal/torrents"
)

const (
	MediaTypeMovie   = "movie"
	MediaTypeTV      = "tv"
	MediaTypeAnime   = "anime"
	MediaTypeUnknown = "unknown"

	MetadataStatusPending   = "pending"
	MetadataStatusMatched   = "matched"
	MetadataStatusManual    = "manual"
	MetadataStatusUnmatched = "unmatched"
	MetadataStatusFailed    = "failed"
)

type Store struct {
	db *sql.DB
}

type Item struct {
	ID             string `json:"id"`
	OwnerUserID    string `json:"ownerUserId"`
	MediaType      string `json:"mediaType"`
	Provider       string `json:"provider,omitempty"`
	ProviderID     string `json:"providerId,omitempty"`
	Title          string `json:"title"`
	OriginalTitle  string `json:"originalTitle,omitempty"`
	Overview       string `json:"overview,omitempty"`
	ReleaseYear    int    `json:"releaseYear,omitempty"`
	PosterURL      string `json:"posterUrl,omitempty"`
	BackdropURL    string `json:"backdropUrl,omitempty"`
	MetadataStatus string `json:"metadataStatus"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type Episode struct {
	ID             string `json:"id"`
	CatalogItemID  string `json:"catalogItemId"`
	Provider       string `json:"provider,omitempty"`
	ProviderID     string `json:"providerId,omitempty"`
	SeasonNumber   int    `json:"seasonNumber,omitempty"`
	EpisodeNumber  int    `json:"episodeNumber,omitempty"`
	AbsoluteNumber int    `json:"absoluteNumber,omitempty"`
	Title          string `json:"title"`
	Overview       string `json:"overview,omitempty"`
	AirDate        string `json:"airDate,omitempty"`
	StillURL       string `json:"stillUrl,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type FileLinkParams struct {
	TorrentFileID    string
	CatalogItemID    string
	CatalogEpisodeID string
	Status           string
	Confidence       float64
	Provider         string
	Error            string
}

type LibraryItem struct {
	ID             string        `json:"id"`
	MediaType      string        `json:"mediaType"`
	Title          string        `json:"title"`
	OriginalTitle  string        `json:"originalTitle,omitempty"`
	Overview       string        `json:"overview,omitempty"`
	ReleaseYear    int           `json:"releaseYear,omitempty"`
	PosterURL      string        `json:"posterUrl,omitempty"`
	BackdropURL    string        `json:"backdropUrl,omitempty"`
	MetadataStatus string        `json:"metadataStatus"`
	Provider       string        `json:"provider,omitempty"`
	ProviderID     string        `json:"providerId,omitempty"`
	Files          []LibraryFile `json:"files"`
}

type LibraryFile struct {
	torrents.TorrentFile
	Episode *Episode `json:"episode,omitempty"`
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) UpsertItem(ctx context.Context, item Item) (Item, error) {
	ownerUserID := strings.TrimSpace(item.OwnerUserID)
	title := strings.TrimSpace(item.Title)
	if ownerUserID == "" || title == "" {
		return Item{}, fmt.Errorf("owner user id and title are required")
	}

	item.OwnerUserID = ownerUserID
	item.Title = title
	item.MediaType = normalizeMediaType(item.MediaType)
	item.Provider = strings.TrimSpace(item.Provider)
	item.ProviderID = strings.TrimSpace(item.ProviderID)
	item.MetadataStatus = normalizeMetadataStatus(item.MetadataStatus, MetadataStatusMatched)

	if item.Provider != "" && item.ProviderID != "" {
		existing, err := s.findItemByProvider(ctx, item.OwnerUserID, item.Provider, item.ProviderID)
		if err == nil {
			if err := s.updateItem(ctx, existing.ID, item); err != nil {
				return Item{}, err
			}
			return s.FindItemByID(ctx, existing.ID)
		}
		if err != sql.ErrNoRows {
			return Item{}, err
		}
	}

	id, err := auth.NewID("cat")
	if err != nil {
		return Item{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO catalog_items (id, owner_user_id, media_type, provider, provider_id, title, original_title, overview, release_year, poster_url, backdrop_url, metadata_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, item.OwnerUserID, item.MediaType, item.Provider, item.ProviderID, item.Title, nullableString(item.OriginalTitle), nullableString(item.Overview), item.ReleaseYear, nullableString(item.PosterURL), nullableString(item.BackdropURL), item.MetadataStatus); err != nil {
		return Item{}, fmt.Errorf("insert catalog item: %w", err)
	}

	return s.FindItemByID(ctx, id)
}

func (s *Store) FindItemByID(ctx context.Context, id string) (Item, error) {
	return scanItem(s.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, media_type, provider, provider_id, title, original_title, overview, release_year, poster_url, backdrop_url, metadata_status, created_at, updated_at
		FROM catalog_items
		WHERE id = ?
	`, strings.TrimSpace(id)))
}

func (s *Store) UpsertEpisode(ctx context.Context, episode Episode) (Episode, error) {
	catalogItemID := strings.TrimSpace(episode.CatalogItemID)
	title := strings.TrimSpace(episode.Title)
	if catalogItemID == "" {
		return Episode{}, fmt.Errorf("catalog item id is required")
	}
	if title == "" {
		title = episodeLabel(episode.SeasonNumber, episode.EpisodeNumber, episode.AbsoluteNumber)
	}
	episode.CatalogItemID = catalogItemID
	episode.Title = title
	episode.Provider = strings.TrimSpace(episode.Provider)
	episode.ProviderID = strings.TrimSpace(episode.ProviderID)

	existing, err := s.findEpisode(ctx, episode)
	if err == nil {
		if err := s.updateEpisode(ctx, existing.ID, episode); err != nil {
			return Episode{}, err
		}
		return s.FindEpisodeByID(ctx, existing.ID)
	}
	if err != sql.ErrNoRows {
		return Episode{}, err
	}

	id, err := auth.NewID("cep")
	if err != nil {
		return Episode{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO catalog_episodes (id, catalog_item_id, provider, provider_id, season_number, episode_number, absolute_number, title, overview, air_date, still_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, episode.CatalogItemID, episode.Provider, episode.ProviderID, maxInt(episode.SeasonNumber, 0), maxInt(episode.EpisodeNumber, 0), maxInt(episode.AbsoluteNumber, 0), episode.Title, nullableString(episode.Overview), nullableString(episode.AirDate), nullableString(episode.StillURL)); err != nil {
		return Episode{}, fmt.Errorf("insert catalog episode: %w", err)
	}

	return s.FindEpisodeByID(ctx, id)
}

func (s *Store) FindEpisodeByID(ctx context.Context, id string) (Episode, error) {
	return scanEpisode(s.db.QueryRowContext(ctx, `
		SELECT id, catalog_item_id, provider, provider_id, season_number, episode_number, absolute_number, title, overview, air_date, still_url, created_at, updated_at
		FROM catalog_episodes
		WHERE id = ?
	`, strings.TrimSpace(id)))
}

func (s *Store) LinkFile(ctx context.Context, params FileLinkParams) error {
	id := strings.TrimSpace(params.TorrentFileID)
	if id == "" {
		return fmt.Errorf("torrent file id is required")
	}
	status := normalizeMetadataStatus(params.Status, MetadataStatusMatched)
	_, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET catalog_item_id = ?,
			catalog_episode_id = ?,
			metadata_status = ?,
			metadata_confidence = ?,
			metadata_provider = ?,
			metadata_error = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nullableString(params.CatalogItemID), nullableString(params.CatalogEpisodeID), status, clampConfidence(params.Confidence), nullableString(params.Provider), nullableString(params.Error), id)
	if err != nil {
		return fmt.Errorf("link torrent file metadata: %w", err)
	}
	return nil
}

func (s *Store) MarkFileUnmatched(ctx context.Context, torrentFileID string) error {
	return s.LinkFile(ctx, FileLinkParams{
		TorrentFileID: torrentFileID,
		Status:        MetadataStatusUnmatched,
		Confidence:    0,
	})
}

func (s *Store) MarkFileFailed(ctx context.Context, torrentFileID string, message string) error {
	return s.LinkFile(ctx, FileLinkParams{
		TorrentFileID: torrentFileID,
		Status:        MetadataStatusFailed,
		Confidence:    0,
		Error:         strings.TrimSpace(message),
	})
}

func (s *Store) ListLibrary(ctx context.Context, ownerUserID string) ([]LibraryItem, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, fmt.Errorf("owner user id cannot be empty")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			tf.id, COALESCE(tf.torrent_id, ''), tf.owner_user_id, tf.slug, tf.name, tf.ext, tf.original_path, tf.hls_path, tf.size_bytes, tf.status, tf.progress_preview, tf.transcoding_percent, tf.download_percent, tf.container, tf.video_codec, tf.audio_codec, tf.duration_seconds, tf.processing_mode, tf.thumbnail_sheet_path, tf.thumbnail_vtt_path, tf.error_message, tf.created_at, tf.updated_at,
			COALESCE(tf.metadata_status, 'pending'), tf.metadata_confidence, COALESCE(tf.metadata_provider, ''), COALESCE(tf.metadata_error, ''),
			ci.id, ci.owner_user_id, ci.media_type, ci.provider, ci.provider_id, ci.title, ci.original_title, ci.overview, ci.release_year, ci.poster_url, ci.backdrop_url, ci.metadata_status, ci.created_at, ci.updated_at,
			ce.id, ce.catalog_item_id, ce.provider, ce.provider_id, ce.season_number, ce.episode_number, ce.absolute_number, ce.title, ce.overview, ce.air_date, ce.still_url, ce.created_at, ce.updated_at
		FROM torrent_files tf
		LEFT JOIN catalog_items ci ON ci.id = tf.catalog_item_id
		LEFT JOIN catalog_episodes ce ON ce.id = tf.catalog_episode_id
		WHERE tf.owner_user_id = ?
		ORDER BY COALESCE(ci.title, tf.name) ASC, ce.season_number ASC, ce.episode_number ASC, tf.name ASC
	`, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list library: %w", err)
	}
	defer rows.Close()

	items := make([]LibraryItem, 0)
	byID := make(map[string]int)
	for rows.Next() {
		row, err := scanLibraryRow(rows)
		if err != nil {
			return nil, err
		}
		key := row.item.ID
		if key == "" {
			key = "file:" + row.file.ID
			row.item = rawLibraryItem(row.file, row.fileMetadataStatus)
		}
		index, ok := byID[key]
		if !ok {
			byID[key] = len(items)
			items = append(items, row.item)
			index = len(items) - 1
		}
		items[index].Files = append(items[index].Files, LibraryFile{
			TorrentFile: row.file,
			Episode:     row.episode,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate library: %w", err)
	}

	return items, nil
}

func (s *Store) updateItem(ctx context.Context, id string, item Item) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE catalog_items
		SET media_type = ?,
			title = ?,
			original_title = ?,
			overview = ?,
			release_year = ?,
			poster_url = ?,
			backdrop_url = ?,
			metadata_status = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, item.MediaType, item.Title, nullableString(item.OriginalTitle), nullableString(item.Overview), item.ReleaseYear, nullableString(item.PosterURL), nullableString(item.BackdropURL), item.MetadataStatus, id)
	if err != nil {
		return fmt.Errorf("update catalog item: %w", err)
	}
	return nil
}

func (s *Store) findItemByProvider(ctx context.Context, ownerUserID string, provider string, providerID string) (Item, error) {
	return scanItem(s.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, media_type, provider, provider_id, title, original_title, overview, release_year, poster_url, backdrop_url, metadata_status, created_at, updated_at
		FROM catalog_items
		WHERE owner_user_id = ? AND provider = ? AND provider_id = ?
	`, ownerUserID, provider, providerID))
}

func (s *Store) updateEpisode(ctx context.Context, id string, episode Episode) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE catalog_episodes
		SET provider = ?,
			provider_id = ?,
			season_number = ?,
			episode_number = ?,
			absolute_number = ?,
			title = ?,
			overview = ?,
			air_date = ?,
			still_url = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, episode.Provider, episode.ProviderID, maxInt(episode.SeasonNumber, 0), maxInt(episode.EpisodeNumber, 0), maxInt(episode.AbsoluteNumber, 0), episode.Title, nullableString(episode.Overview), nullableString(episode.AirDate), nullableString(episode.StillURL), id)
	if err != nil {
		return fmt.Errorf("update catalog episode: %w", err)
	}
	return nil
}

func (s *Store) findEpisode(ctx context.Context, episode Episode) (Episode, error) {
	if episode.Provider != "" && episode.ProviderID != "" {
		found, err := scanEpisode(s.db.QueryRowContext(ctx, `
			SELECT id, catalog_item_id, provider, provider_id, season_number, episode_number, absolute_number, title, overview, air_date, still_url, created_at, updated_at
			FROM catalog_episodes
			WHERE catalog_item_id = ? AND provider = ? AND provider_id = ?
		`, episode.CatalogItemID, episode.Provider, episode.ProviderID))
		if err == nil || err != sql.ErrNoRows {
			return found, err
		}
	}
	if episode.SeasonNumber > 0 && episode.EpisodeNumber > 0 {
		return scanEpisode(s.db.QueryRowContext(ctx, `
			SELECT id, catalog_item_id, provider, provider_id, season_number, episode_number, absolute_number, title, overview, air_date, still_url, created_at, updated_at
			FROM catalog_episodes
			WHERE catalog_item_id = ? AND season_number = ? AND episode_number = ?
		`, episode.CatalogItemID, episode.SeasonNumber, episode.EpisodeNumber))
	}
	if episode.AbsoluteNumber > 0 {
		return scanEpisode(s.db.QueryRowContext(ctx, `
			SELECT id, catalog_item_id, provider, provider_id, season_number, episode_number, absolute_number, title, overview, air_date, still_url, created_at, updated_at
			FROM catalog_episodes
			WHERE catalog_item_id = ? AND absolute_number = ?
		`, episode.CatalogItemID, episode.AbsoluteNumber))
	}
	return Episode{}, sql.ErrNoRows
}

type libraryRow struct {
	file               torrents.TorrentFile
	fileMetadataStatus string
	item               LibraryItem
	episode            *Episode
}

func scanLibraryRow(row rowScanner) (libraryRow, error) {
	var result libraryRow
	var item Item
	var episode Episode
	var itemID, itemOwnerUserID, itemMediaType, itemProvider, itemProviderID, itemTitle, itemOriginalTitle, itemOverview, itemPosterURL, itemBackdropURL, itemMetadataStatus, itemCreatedAt, itemUpdatedAt sql.NullString
	var itemReleaseYear sql.NullInt64
	var episodeID, episodeCatalogItemID, episodeProvider, episodeProviderID, episodeTitle, episodeOverview, episodeAirDate, episodeStillURL, episodeCreatedAt, episodeUpdatedAt sql.NullString
	var episodeSeason, episodeNumber, episodeAbsolute sql.NullInt64
	var metadataProvider, metadataError string
	var metadataConfidence float64

	if err := scanTorrentFilePrefix(row, &result.file,
		&result.fileMetadataStatus, &metadataConfidence, &metadataProvider, &metadataError,
		&itemID, &itemOwnerUserID, &itemMediaType, &itemProvider, &itemProviderID, &itemTitle, &itemOriginalTitle, &itemOverview, &itemReleaseYear, &itemPosterURL, &itemBackdropURL, &itemMetadataStatus, &itemCreatedAt, &itemUpdatedAt,
		&episodeID, &episodeCatalogItemID, &episodeProvider, &episodeProviderID, &episodeSeason, &episodeNumber, &episodeAbsolute, &episodeTitle, &episodeOverview, &episodeAirDate, &episodeStillURL, &episodeCreatedAt, &episodeUpdatedAt,
	); err != nil {
		return libraryRow{}, err
	}
	_ = metadataConfidence
	_ = metadataProvider
	_ = metadataError

	if itemID.Valid {
		item = Item{
			ID:             itemID.String,
			OwnerUserID:    itemOwnerUserID.String,
			MediaType:      itemMediaType.String,
			Provider:       itemProvider.String,
			ProviderID:     itemProviderID.String,
			Title:          itemTitle.String,
			OriginalTitle:  itemOriginalTitle.String,
			Overview:       itemOverview.String,
			ReleaseYear:    int(itemReleaseYear.Int64),
			PosterURL:      itemPosterURL.String,
			BackdropURL:    itemBackdropURL.String,
			MetadataStatus: itemMetadataStatus.String,
			CreatedAt:      itemCreatedAt.String,
			UpdatedAt:      itemUpdatedAt.String,
		}
		result.item = LibraryItem{
			ID:             item.ID,
			MediaType:      item.MediaType,
			Title:          item.Title,
			OriginalTitle:  item.OriginalTitle,
			Overview:       item.Overview,
			ReleaseYear:    item.ReleaseYear,
			PosterURL:      item.PosterURL,
			BackdropURL:    item.BackdropURL,
			MetadataStatus: item.MetadataStatus,
			Provider:       item.Provider,
			ProviderID:     item.ProviderID,
		}
	}
	if episodeID.Valid {
		episode = Episode{
			ID:             episodeID.String,
			CatalogItemID:  episodeCatalogItemID.String,
			Provider:       episodeProvider.String,
			ProviderID:     episodeProviderID.String,
			SeasonNumber:   int(episodeSeason.Int64),
			EpisodeNumber:  int(episodeNumber.Int64),
			AbsoluteNumber: int(episodeAbsolute.Int64),
			Title:          episodeTitle.String,
			Overview:       episodeOverview.String,
			AirDate:        episodeAirDate.String,
			StillURL:       episodeStillURL.String,
			CreatedAt:      episodeCreatedAt.String,
			UpdatedAt:      episodeUpdatedAt.String,
		}
		result.episode = &episode
	}

	return result, nil
}

func rawLibraryItem(file torrents.TorrentFile, metadataStatus string) LibraryItem {
	status := normalizeMetadataStatus(metadataStatus, MetadataStatusPending)
	return LibraryItem{
		ID:             "file:" + file.ID,
		MediaType:      MediaTypeUnknown,
		Title:          file.Name,
		MetadataStatus: status,
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (Item, error) {
	var item Item
	var originalTitle, overview, posterURL, backdropURL sql.NullString
	if err := row.Scan(&item.ID, &item.OwnerUserID, &item.MediaType, &item.Provider, &item.ProviderID, &item.Title, &originalTitle, &overview, &item.ReleaseYear, &posterURL, &backdropURL, &item.MetadataStatus, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return Item{}, err
	}
	item.OriginalTitle = originalTitle.String
	item.Overview = overview.String
	item.PosterURL = posterURL.String
	item.BackdropURL = backdropURL.String
	return item, nil
}

func scanEpisode(row rowScanner) (Episode, error) {
	var episode Episode
	var overview, airDate, stillURL sql.NullString
	if err := row.Scan(&episode.ID, &episode.CatalogItemID, &episode.Provider, &episode.ProviderID, &episode.SeasonNumber, &episode.EpisodeNumber, &episode.AbsoluteNumber, &episode.Title, &overview, &airDate, &stillURL, &episode.CreatedAt, &episode.UpdatedAt); err != nil {
		return Episode{}, err
	}
	episode.Overview = overview.String
	episode.AirDate = airDate.String
	episode.StillURL = stillURL.String
	return episode, nil
}

func scanTorrentFilePrefix(row rowScanner, file *torrents.TorrentFile, extras ...any) error {
	var torrentID sql.NullString
	var originalPath sql.NullString
	var hlsPath sql.NullString
	var container sql.NullString
	var videoCodec sql.NullString
	var audioCodec sql.NullString
	var processingMode sql.NullString
	var thumbnailSheetPath sql.NullString
	var thumbnailVTTPath sql.NullString
	var progressPreview int
	var errorMessage sql.NullString
	dest := []any{
		&file.ID,
		&torrentID,
		&file.OwnerUserID,
		&file.Slug,
		&file.Name,
		&file.Ext,
		&originalPath,
		&hlsPath,
		&file.SizeBytes,
		&file.Status,
		&progressPreview,
		&file.TranscodingPercent,
		&file.DownloadPercent,
		&container,
		&videoCodec,
		&audioCodec,
		&file.DurationSeconds,
		&processingMode,
		&thumbnailSheetPath,
		&thumbnailVTTPath,
		&errorMessage,
		&file.CreatedAt,
		&file.UpdatedAt,
	}
	dest = append(dest, extras...)
	if err := row.Scan(dest...); err != nil {
		return err
	}
	file.TorrentID = torrentID.String
	file.OriginalPath = originalPath.String
	file.HLSPath = hlsPath.String
	file.ProgressPreview = progressPreview == 1
	file.Container = container.String
	file.VideoCodec = videoCodec.String
	file.AudioCodec = audioCodec.String
	file.ProcessingMode = processingMode.String
	file.ThumbnailSheetPath = thumbnailSheetPath.String
	file.ThumbnailVTTPath = thumbnailVTTPath.String
	file.DirectPlayable = torrents.IsDirectPlayable(*file)
	file.ErrorMessage = errorMessage.String
	return nil
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func normalizeMediaType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case MediaTypeMovie, MediaTypeTV, MediaTypeAnime:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return MediaTypeUnknown
	}
}

func normalizeMetadataStatus(value string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case MetadataStatusPending, MetadataStatusMatched, MetadataStatusManual, MetadataStatusUnmatched, MetadataStatusFailed:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return fallback
	}
}

func clampConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func episodeLabel(season int, episode int, absolute int) string {
	if season > 0 && episode > 0 {
		return fmt.Sprintf("S%02dE%02d", season, episode)
	}
	if absolute > 0 {
		return fmt.Sprintf("Episode %d", absolute)
	}
	return "Episode"
}

func maxInt(value int, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}

package torrents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
)

const (
	StatusAdded       = "added"
	StatusDownloading = "downloading"
	StatusPaused      = "paused"
	StatusQueued      = "queued"
	StatusProcessing  = "processing"
	StatusDone        = "done"
	StatusError       = "error"

	FileStatusDownloading = "downloading"
	FileStatusQueued      = "queued"
	FileStatusProcessing  = "processing"
	FileStatusDone        = "done"
	FileStatusError       = "error"

	RetentionKeep = "keep"
)

var (
	ErrInvalidMagnet = errors.New("torrents: invalid magnet URI")
	ErrNotFound      = errors.New("torrents: not found")
)

type Store struct {
	db *sql.DB
}

type Torrent struct {
	ID                 string  `json:"id"`
	OwnerUserID        string  `json:"ownerUserId"`
	Slug               string  `json:"slug"`
	MagnetURI          string  `json:"magnetUri"`
	InfoHash           string  `json:"infoHash,omitempty"`
	QBittorrentHash    string  `json:"qbittorrentHash,omitempty"`
	Name               string  `json:"name,omitempty"`
	SizeBytes          int64   `json:"sizeBytes"`
	Status             string  `json:"status"`
	ProgressPercent    float64 `json:"progressPercent"`
	DownloadSpeedBytes int64   `json:"downloadSpeedBytes"`
	UploadSpeedBytes   int64   `json:"uploadSpeedBytes"`
	ETASeconds         int64   `json:"etaSeconds"`
	Peers              int     `json:"peers"`
	Ratio              float64 `json:"ratio"`
	RetentionPolicy    string  `json:"retentionPolicy"`
	ErrorMessage       string  `json:"errorMessage,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

type TorrentFile struct {
	ID                 string  `json:"id"`
	TorrentID          string  `json:"torrentId"`
	OwnerUserID        string  `json:"ownerUserId"`
	Slug               string  `json:"slug"`
	Name               string  `json:"name"`
	Ext                string  `json:"ext"`
	OriginalPath       string  `json:"originalPath,omitempty"`
	HLSPath            string  `json:"hlsPath,omitempty"`
	SizeBytes          int64   `json:"sizeBytes"`
	Status             string  `json:"status"`
	ProgressPreview    bool    `json:"progressPreview"`
	TranscodingPercent float64 `json:"transcodingPercent"`
	DownloadPercent    float64 `json:"downloadPercent"`
	Container          string  `json:"container,omitempty"`
	VideoCodec         string  `json:"videoCodec,omitempty"`
	AudioCodec         string  `json:"audioCodec,omitempty"`
	DurationSeconds    float64 `json:"durationSeconds,omitempty"`
	ProcessingMode     string  `json:"processingMode,omitempty"`
	ThumbnailSheetPath string  `json:"thumbnailSheetPath,omitempty"`
	ThumbnailVTTPath   string  `json:"thumbnailVttPath,omitempty"`
	DirectPlayable     bool    `json:"directPlayable"`
	ErrorMessage       string  `json:"errorMessage,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

type Subtitle struct {
	ID            string `json:"id"`
	TorrentFileID string `json:"torrentFileId"`
	FileName      string `json:"fileName"`
	Title         string `json:"title"`
	Language      string `json:"language"`
	Path          string `json:"path,omitempty"`
	URL           string `json:"url,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

type CreateTorrentParams struct {
	OwnerUserID string
	MagnetURI   string
	Name        string
}

type UpdateTorrentTransferStateParams struct {
	ID                 string
	QBittorrentHash    string
	Name               string
	SizeBytes          int64
	Status             string
	ProgressPercent    float64
	DownloadSpeedBytes int64
	UploadSpeedBytes   int64
	ETASeconds         int64
	Peers              int
	Ratio              float64
	ErrorMessage       string
}

type CreateTorrentFileParams struct {
	TorrentID    string
	OwnerUserID  string
	Name         string
	Ext          string
	OriginalPath string
	SizeBytes    int64
	Status       string
}

type UpdateTorrentFileMediaMetadataParams struct {
	ID              string
	Container       string
	VideoCodec      string
	AudioCodec      string
	DurationSeconds float64
	ProcessingMode  string
}

type CreateSubtitleParams struct {
	TorrentFileID string
	FileName      string
	Title         string
	Language      string
	Path          string
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateTorrent(ctx context.Context, params CreateTorrentParams) (Torrent, error) {
	ownerUserID := strings.TrimSpace(params.OwnerUserID)
	if ownerUserID == "" {
		return Torrent{}, fmt.Errorf("owner user id cannot be empty")
	}

	magnetURI := strings.TrimSpace(params.MagnetURI)
	infoHash, displayName, err := ParseMagnet(magnetURI)
	if err != nil {
		return Torrent{}, err
	}

	name := strings.TrimSpace(params.Name)
	if name == "" {
		name = displayName
	}

	id, err := auth.NewID("tor")
	if err != nil {
		return Torrent{}, err
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO torrents (id, owner_user_id, slug, magnet_uri, info_hash, name, status, retention_policy)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, ownerUserID, id, magnetURI, nullableString(infoHash), nullableString(name), StatusAdded, RetentionKeep); err != nil {
		return Torrent{}, fmt.Errorf("create torrent: %w", err)
	}

	return s.FindTorrentByID(ctx, id)
}

func (s *Store) FindTorrentByID(ctx context.Context, id string) (Torrent, error) {
	return scanTorrent(s.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, slug, magnet_uri, info_hash, qbt_hash, name, size_bytes, status, progress_percent, download_speed_bytes, upload_speed_bytes, eta_seconds, peers, ratio, retention_policy, error_message, created_at, updated_at
		FROM torrents
		WHERE id = ?
	`, id))
}

func (s *Store) ListTorrents(ctx context.Context, ownerUserID string) ([]Torrent, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, fmt.Errorf("owner user id cannot be empty")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, owner_user_id, slug, magnet_uri, info_hash, qbt_hash, name, size_bytes, status, progress_percent, download_speed_bytes, upload_speed_bytes, eta_seconds, peers, ratio, retention_policy, error_message, created_at, updated_at
		FROM torrents
		WHERE owner_user_id = ?
		ORDER BY created_at DESC, id DESC
	`, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list torrents: %w", err)
	}
	defer rows.Close()

	torrents := make([]Torrent, 0)
	for rows.Next() {
		torrent, err := scanTorrentRow(rows)
		if err != nil {
			return nil, err
		}
		torrents = append(torrents, torrent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate torrents: %w", err)
	}

	return torrents, nil
}

func (s *Store) UpdateTorrentTransferState(ctx context.Context, params UpdateTorrentTransferStateParams) error {
	id := strings.TrimSpace(params.ID)
	if id == "" {
		return ErrNotFound
	}

	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = StatusAdded
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE torrents
		SET qbt_hash = COALESCE(?, qbt_hash),
			name = COALESCE(?, name),
			size_bytes = ?,
			status = ?,
			progress_percent = ?,
			download_speed_bytes = ?,
			upload_speed_bytes = ?,
			eta_seconds = ?,
			peers = ?,
			ratio = ?,
			error_message = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		nullableString(strings.TrimSpace(params.QBittorrentHash)),
		nullableString(strings.TrimSpace(params.Name)),
		maxInt64(params.SizeBytes, 0),
		status,
		clampFloat(params.ProgressPercent, 0, 100),
		maxInt64(params.DownloadSpeedBytes, 0),
		maxInt64(params.UploadSpeedBytes, 0),
		maxInt64(params.ETASeconds, -1),
		maxInt(params.Peers, 0),
		maxFloat(params.Ratio, 0),
		nullableString(strings.TrimSpace(params.ErrorMessage)),
		id,
	)
	if err != nil {
		return fmt.Errorf("update torrent transfer state: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update torrent transfer state rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *Store) DeleteTorrent(ctx context.Context, id string, ownerUserID string) error {
	id = strings.TrimSpace(id)
	ownerUserID = strings.TrimSpace(ownerUserID)
	if id == "" || ownerUserID == "" {
		return ErrNotFound
	}

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM torrents
		WHERE id = ? AND owner_user_id = ?
	`, id, ownerUserID)
	if err != nil {
		return fmt.Errorf("delete torrent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete torrent rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *Store) CreateTorrentFileIfMissing(ctx context.Context, params CreateTorrentFileParams) (TorrentFile, bool, error) {
	torrentID := strings.TrimSpace(params.TorrentID)
	name := strings.TrimSpace(params.Name)
	originalPath := strings.TrimSpace(params.OriginalPath)
	if torrentID == "" || name == "" || originalPath == "" {
		return TorrentFile{}, false, fmt.Errorf("torrent id, name, and original path are required")
	}

	ownerUserID := strings.TrimSpace(params.OwnerUserID)
	if ownerUserID == "" {
		torrent, err := s.FindTorrentByID(ctx, torrentID)
		if err != nil {
			return TorrentFile{}, false, fmt.Errorf("resolve torrent owner: %w", err)
		}
		ownerUserID = torrent.OwnerUserID
	}

	existing, err := s.findTorrentFileByOriginalPath(ctx, torrentID, originalPath)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return TorrentFile{}, false, err
	}

	id, err := auth.NewID("tfi")
	if err != nil {
		return TorrentFile{}, false, err
	}

	initialStatus := strings.TrimSpace(params.Status)
	if initialStatus == "" {
		initialStatus = FileStatusQueued
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO torrent_files (id, torrent_id, owner_user_id, slug, name, ext, original_path, size_bytes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, torrentID, ownerUserID, id, name, strings.ToLower(strings.TrimSpace(params.Ext)), originalPath, maxInt64(params.SizeBytes, 0), initialStatus); err != nil {
		return TorrentFile{}, false, fmt.Errorf("create torrent file: %w", err)
	}

	created, err := s.FindTorrentFileByID(ctx, id)
	if err != nil {
		return TorrentFile{}, false, err
	}

	return created, true, nil
}

func (s *Store) FindTorrentFileByID(ctx context.Context, id string) (TorrentFile, error) {
	return scanTorrentFile(s.db.QueryRowContext(ctx, `
		SELECT id, torrent_id, owner_user_id, slug, name, ext, original_path, hls_path, size_bytes, status, progress_preview, transcoding_percent, download_percent, container, video_codec, audio_codec, duration_seconds, processing_mode, thumbnail_sheet_path, thumbnail_vtt_path, error_message, created_at, updated_at
		FROM torrent_files
		WHERE id = ?
	`, strings.TrimSpace(id)))
}

func (s *Store) FindTorrentFileByIDForOwner(ctx context.Context, id string, ownerUserID string) (TorrentFile, error) {
	return scanTorrentFile(s.db.QueryRowContext(ctx, `
		SELECT id, torrent_id, owner_user_id, slug, name, ext, original_path, hls_path, size_bytes, status, progress_preview, transcoding_percent, download_percent, container, video_codec, audio_codec, duration_seconds, processing_mode, thumbnail_sheet_path, thumbnail_vtt_path, error_message, created_at, updated_at
		FROM torrent_files
		WHERE id = ? AND owner_user_id = ?
	`, strings.TrimSpace(id), strings.TrimSpace(ownerUserID)))
}

func (s *Store) ListTorrentFiles(ctx context.Context, torrentID string) ([]TorrentFile, error) {
	torrentID = strings.TrimSpace(torrentID)
	if torrentID == "" {
		return nil, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, torrent_id, owner_user_id, slug, name, ext, original_path, hls_path, size_bytes, status, progress_preview, transcoding_percent, download_percent, container, video_codec, audio_codec, duration_seconds, processing_mode, thumbnail_sheet_path, thumbnail_vtt_path, error_message, created_at, updated_at
		FROM torrent_files
		WHERE torrent_id = ?
		ORDER BY name ASC, id ASC
	`, torrentID)
	if err != nil {
		return nil, fmt.Errorf("list torrent files: %w", err)
	}
	defer rows.Close()

	files := make([]TorrentFile, 0)
	for rows.Next() {
		file, err := scanTorrentFileRow(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate torrent files: %w", err)
	}

	return files, nil
}

func (s *Store) ListTorrentFilesByOwner(ctx context.Context, ownerUserID string) ([]TorrentFile, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, fmt.Errorf("owner user id cannot be empty")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, torrent_id, owner_user_id, slug, name, ext, original_path, hls_path, size_bytes, status, progress_preview, transcoding_percent, download_percent, container, video_codec, audio_codec, duration_seconds, processing_mode, thumbnail_sheet_path, thumbnail_vtt_path, error_message, created_at, updated_at
		FROM torrent_files
		WHERE owner_user_id = ?
		ORDER BY created_at DESC, id DESC
	`, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list torrent files by owner: %w", err)
	}
	defer rows.Close()

	files := make([]TorrentFile, 0)
	for rows.Next() {
		file, err := scanTorrentFileRow(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate torrent files by owner: %w", err)
	}

	return files, nil
}

func (s *Store) DeleteTorrentFilesForTorrent(ctx context.Context, torrentID string) error {
	torrentID = strings.TrimSpace(torrentID)
	if torrentID == "" {
		return ErrNotFound
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM torrent_files WHERE torrent_id = ?`, torrentID)
	if err != nil {
		return fmt.Errorf("delete torrent files: %w", err)
	}

	return nil
}

func (s *Store) MarkTorrentFileProcessing(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET status = ?,
			transcoding_percent = 0,
			error_message = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, FileStatusProcessing, id)
	if err != nil {
		return fmt.Errorf("mark torrent file processing: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) MarkTorrentFileProcessingWithHLS(ctx context.Context, id string, hlsPath string) error {
	id = strings.TrimSpace(id)
	hlsPath = strings.TrimSpace(hlsPath)
	if id == "" {
		return ErrNotFound
	}
	if hlsPath == "" {
		return fmt.Errorf("hls path cannot be empty")
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET status = ?,
			hls_path = ?,
			transcoding_percent = 0,
			error_message = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, FileStatusProcessing, hlsPath, id)
	if err != nil {
		return fmt.Errorf("mark torrent file processing with hls: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) MarkTorrentFileQueued(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	// Transition from downloading → queued when torrent completes download.
	// Silently succeeds if the file is already in a later state.
	_, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = ?
	`, FileStatusQueued, id, FileStatusDownloading)
	if err != nil {
		return fmt.Errorf("mark torrent file queued: %w", err)
	}

	return nil
}

func (s *Store) UpdateTorrentFileDownloadProgress(ctx context.Context, id string, percent float64) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET download_percent = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, clampFloat(percent, 0, 100), id)
	if err != nil {
		return fmt.Errorf("update torrent file download progress: %w", err)
	}

	return nil
}

func (s *Store) UpdateTorrentFileTranscodingProgress(ctx context.Context, id string, percent float64) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET status = ?,
			transcoding_percent = ?,
			error_message = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, FileStatusProcessing, clampFloat(percent, 0, 100), id)
	if err != nil {
		return fmt.Errorf("update torrent file transcoding progress: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) UpdateTorrentFileMediaMetadata(ctx context.Context, params UpdateTorrentFileMediaMetadataParams) error {
	id := strings.TrimSpace(params.ID)
	if id == "" {
		return ErrNotFound
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET container = ?,
			video_codec = ?,
			audio_codec = ?,
			duration_seconds = ?,
			processing_mode = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		strings.ToLower(strings.TrimSpace(params.Container)),
		strings.ToLower(strings.TrimSpace(params.VideoCodec)),
		strings.ToLower(strings.TrimSpace(params.AudioCodec)),
		maxFloat(params.DurationSeconds, 0),
		strings.ToLower(strings.TrimSpace(params.ProcessingMode)),
		id,
	)
	if err != nil {
		return fmt.Errorf("update torrent file media metadata: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) MarkTorrentFilePreviewReady(ctx context.Context, id string, sheetPath string, vttPath string) error {
	id = strings.TrimSpace(id)
	sheetPath = strings.TrimSpace(sheetPath)
	vttPath = strings.TrimSpace(vttPath)
	if id == "" {
		return ErrNotFound
	}
	if sheetPath == "" || vttPath == "" {
		return fmt.Errorf("thumbnail sheet and vtt paths are required")
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET progress_preview = 1,
			thumbnail_sheet_path = ?,
			thumbnail_vtt_path = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, sheetPath, vttPath, id)
	if err != nil {
		return fmt.Errorf("mark torrent file preview ready: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) ResetTorrentFileForTranscode(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET status = ?,
			hls_path = NULL,
			progress_preview = 0,
			thumbnail_sheet_path = NULL,
			thumbnail_vtt_path = NULL,
			transcoding_percent = 0,
			error_message = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, FileStatusQueued, id)
	if err != nil {
		return fmt.Errorf("reset torrent file for transcode: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) MarkTorrentFileDone(ctx context.Context, id string, hlsPath string) error {
	id = strings.TrimSpace(id)
	hlsPath = strings.TrimSpace(hlsPath)
	if id == "" {
		return ErrNotFound
	}
	if hlsPath == "" {
		return fmt.Errorf("hls path cannot be empty")
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET status = ?,
			hls_path = ?,
			transcoding_percent = 100,
			error_message = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, FileStatusDone, hlsPath, id)
	if err != nil {
		return fmt.Errorf("mark torrent file done: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) MarkTorrentFileError(ctx context.Context, id string, message string) error {
	id = strings.TrimSpace(id)
	message = strings.TrimSpace(message)
	if id == "" {
		return ErrNotFound
	}
	if message == "" {
		message = "file processing failed"
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE torrent_files
		SET status = ?,
			error_message = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, FileStatusError, message, id)
	if err != nil {
		return fmt.Errorf("mark torrent file error: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) findTorrentFileByOriginalPath(ctx context.Context, torrentID string, originalPath string) (TorrentFile, error) {
	return scanTorrentFile(s.db.QueryRowContext(ctx, `
		SELECT id, torrent_id, owner_user_id, slug, name, ext, original_path, hls_path, size_bytes, status, progress_preview, transcoding_percent, download_percent, container, video_codec, audio_codec, duration_seconds, processing_mode, thumbnail_sheet_path, thumbnail_vtt_path, error_message, created_at, updated_at
		FROM torrent_files
		WHERE torrent_id = ? AND original_path = ?
	`, torrentID, originalPath))
}

func (s *Store) CreateSubtitleIfMissing(ctx context.Context, params CreateSubtitleParams) (Subtitle, bool, error) {
	torrentFileID := strings.TrimSpace(params.TorrentFileID)
	path := strings.TrimSpace(params.Path)
	fileName := strings.TrimSpace(params.FileName)
	if torrentFileID == "" || path == "" || fileName == "" {
		return Subtitle{}, false, fmt.Errorf("torrent file id, file name, and path are required")
	}

	id, err := auth.NewID("sub")
	if err != nil {
		return Subtitle{}, false, err
	}

	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = fileName
	}
	language := strings.TrimSpace(params.Language)
	if language == "" {
		language = "und"
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO subtitles (id, torrent_file_id, file_name, title, language, path)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, torrentFileID, fileName, title, language, path)
	if err != nil {
		return Subtitle{}, false, fmt.Errorf("create subtitle: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Subtitle{}, false, fmt.Errorf("read create subtitle rows affected: %w", err)
	}
	if rowsAffected == 0 {
		subtitle, err := s.findSubtitleByPath(ctx, torrentFileID, path)
		return subtitle, false, err
	}

	subtitle, err := s.FindSubtitleByID(ctx, id)
	return subtitle, true, err
}

func (s *Store) ListSubtitlesForFileOwner(ctx context.Context, torrentFileID string, ownerUserID string) ([]Subtitle, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.torrent_file_id, s.file_name, s.title, s.language, s.path, s.created_at
		FROM subtitles s
		INNER JOIN torrent_files tf ON tf.id = s.torrent_file_id
		WHERE s.torrent_file_id = ? AND tf.owner_user_id = ?
		ORDER BY s.language ASC, s.title ASC, s.id ASC
	`, strings.TrimSpace(torrentFileID), strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, fmt.Errorf("list subtitles for file owner: %w", err)
	}
	defer rows.Close()

	subtitles := make([]Subtitle, 0)
	for rows.Next() {
		subtitle, err := scanSubtitleRow(rows)
		if err != nil {
			return nil, err
		}
		subtitles = append(subtitles, subtitle)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subtitles for file owner: %w", err)
	}

	return subtitles, nil
}

func (s *Store) FindSubtitleByID(ctx context.Context, id string) (Subtitle, error) {
	return scanSubtitle(s.db.QueryRowContext(ctx, `
		SELECT id, torrent_file_id, file_name, title, language, path, created_at
		FROM subtitles
		WHERE id = ?
	`, strings.TrimSpace(id)))
}

func (s *Store) FindSubtitleByIDForOwner(ctx context.Context, id string, ownerUserID string) (Subtitle, error) {
	return scanSubtitle(s.db.QueryRowContext(ctx, `
		SELECT s.id, s.torrent_file_id, s.file_name, s.title, s.language, s.path, s.created_at
		FROM subtitles s
		INNER JOIN torrent_files tf ON tf.id = s.torrent_file_id
		WHERE s.id = ? AND tf.owner_user_id = ?
	`, strings.TrimSpace(id), strings.TrimSpace(ownerUserID)))
}

func (s *Store) findSubtitleByPath(ctx context.Context, torrentFileID string, path string) (Subtitle, error) {
	return scanSubtitle(s.db.QueryRowContext(ctx, `
		SELECT id, torrent_file_id, file_name, title, language, path, created_at
		FROM subtitles
		WHERE torrent_file_id = ? AND path = ?
	`, strings.TrimSpace(torrentFileID), strings.TrimSpace(path)))
}

func (s *Store) MarkTorrentError(ctx context.Context, id string, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "torrent submission failed"
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE torrents
		SET status = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, StatusError, message, id); err != nil {
		return fmt.Errorf("mark torrent error: %w", err)
	}

	return nil
}

func checkRowsAffected(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func ParseMagnet(magnetURI string) (infoHash string, displayName string, err error) {
	magnetURI = strings.TrimSpace(magnetURI)
	if magnetURI == "" {
		return "", "", ErrInvalidMagnet
	}

	parsed, err := url.Parse(magnetURI)
	if err != nil || !strings.EqualFold(parsed.Scheme, "magnet") {
		return "", "", ErrInvalidMagnet
	}

	query := parsed.Query()
	for _, exactTopic := range query["xt"] {
		lower := strings.ToLower(strings.TrimSpace(exactTopic))
		const prefix = "urn:btih:"
		if strings.HasPrefix(lower, prefix) {
			infoHash = strings.TrimSpace(lower[len(prefix):])
			break
		}
	}
	if infoHash == "" {
		return "", "", ErrInvalidMagnet
	}

	return infoHash, strings.TrimSpace(query.Get("dn")), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTorrent(row rowScanner) (Torrent, error) {
	torrent, err := scanTorrentValues(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return Torrent{}, ErrNotFound
		}
		return Torrent{}, fmt.Errorf("scan torrent: %w", err)
	}

	return torrent, nil
}

func scanTorrentRow(row rowScanner) (Torrent, error) {
	torrent, err := scanTorrentValues(row)
	if err != nil {
		return Torrent{}, fmt.Errorf("scan torrent: %w", err)
	}

	return torrent, nil
}

func scanTorrentFile(row rowScanner) (TorrentFile, error) {
	file, err := scanTorrentFileValues(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return TorrentFile{}, ErrNotFound
		}
		return TorrentFile{}, fmt.Errorf("scan torrent file: %w", err)
	}

	return file, nil
}

func scanTorrentFileRow(row rowScanner) (TorrentFile, error) {
	file, err := scanTorrentFileValues(row)
	if err != nil {
		return TorrentFile{}, fmt.Errorf("scan torrent file: %w", err)
	}

	return file, nil
}

func scanTorrentFileValues(row rowScanner) (TorrentFile, error) {
	var file TorrentFile
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

	if err := row.Scan(
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
	); err != nil {
		return TorrentFile{}, err
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
	file.DirectPlayable = IsDirectPlayable(file)
	file.ErrorMessage = errorMessage.String

	return file, nil
}

func scanSubtitle(row rowScanner) (Subtitle, error) {
	subtitle, err := scanSubtitleValues(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return Subtitle{}, ErrNotFound
		}
		return Subtitle{}, fmt.Errorf("scan subtitle: %w", err)
	}

	return subtitle, nil
}

func scanSubtitleRow(row rowScanner) (Subtitle, error) {
	subtitle, err := scanSubtitleValues(row)
	if err != nil {
		return Subtitle{}, fmt.Errorf("scan subtitle: %w", err)
	}

	return subtitle, nil
}

func scanSubtitleValues(row rowScanner) (Subtitle, error) {
	var subtitle Subtitle
	if err := row.Scan(
		&subtitle.ID,
		&subtitle.TorrentFileID,
		&subtitle.FileName,
		&subtitle.Title,
		&subtitle.Language,
		&subtitle.Path,
		&subtitle.CreatedAt,
	); err != nil {
		return Subtitle{}, err
	}

	return subtitle, nil
}

func IsDirectPlayable(file TorrentFile) bool {
	ext := strings.ToLower(strings.TrimSpace(file.Ext))
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(file.Name))
	}
	if ext != ".mp4" && ext != ".m4v" {
		return false
	}

	videoCodec := strings.ToLower(strings.TrimSpace(file.VideoCodec))
	audioCodec := strings.ToLower(strings.TrimSpace(file.AudioCodec))
	if videoCodec == "" || (videoCodec != "h264" && videoCodec != "avc" && videoCodec != "avc1") {
		return false
	}
	return audioCodec == "" || audioCodec == "aac"
}

func scanTorrentValues(row rowScanner) (Torrent, error) {
	var torrent Torrent
	var infoHash sql.NullString
	var qbtHash sql.NullString
	var name sql.NullString
	var errorMessage sql.NullString

	if err := row.Scan(
		&torrent.ID,
		&torrent.OwnerUserID,
		&torrent.Slug,
		&torrent.MagnetURI,
		&infoHash,
		&qbtHash,
		&name,
		&torrent.SizeBytes,
		&torrent.Status,
		&torrent.ProgressPercent,
		&torrent.DownloadSpeedBytes,
		&torrent.UploadSpeedBytes,
		&torrent.ETASeconds,
		&torrent.Peers,
		&torrent.Ratio,
		&torrent.RetentionPolicy,
		&errorMessage,
		&torrent.CreatedAt,
		&torrent.UpdatedAt,
	); err != nil {
		return Torrent{}, err
	}

	torrent.InfoHash = infoHash.String
	torrent.QBittorrentHash = qbtHash.String
	torrent.Name = name.String
	torrent.ErrorMessage = errorMessage.String

	return torrent, nil
}

func clampFloat(value float64, minValue float64, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxFloat(value float64, minValue float64) float64 {
	if value < minValue {
		return minValue
	}
	return value
}

func maxInt(value int, minValue int) int {
	if value < minValue {
		return minValue
	}
	return value
}

func maxInt64(value int64, minValue int64) int64 {
	if value < minValue {
		return minValue
	}
	return value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

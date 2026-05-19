package torrents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
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

	FileStatusQueued     = "queued"
	FileStatusProcessing = "processing"
	FileStatusDone       = "done"
	FileStatusError      = "error"

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
	Slug               string  `json:"slug"`
	Name               string  `json:"name"`
	Ext                string  `json:"ext"`
	OriginalPath       string  `json:"originalPath,omitempty"`
	HLSPath            string  `json:"hlsPath,omitempty"`
	SizeBytes          int64   `json:"sizeBytes"`
	Status             string  `json:"status"`
	ProgressPreview    bool    `json:"progressPreview"`
	TranscodingPercent float64 `json:"transcodingPercent"`
	ErrorMessage       string  `json:"errorMessage,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
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
	Name         string
	Ext          string
	OriginalPath string
	SizeBytes    int64
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

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO torrent_files (id, torrent_id, slug, name, ext, original_path, size_bytes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, torrentID, id, name, strings.ToLower(strings.TrimSpace(params.Ext)), originalPath, maxInt64(params.SizeBytes, 0), FileStatusQueued); err != nil {
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
		SELECT id, torrent_id, slug, name, ext, original_path, hls_path, size_bytes, status, progress_preview, transcoding_percent, error_message, created_at, updated_at
		FROM torrent_files
		WHERE id = ?
	`, strings.TrimSpace(id)))
}

func (s *Store) ListTorrentFiles(ctx context.Context, torrentID string) ([]TorrentFile, error) {
	torrentID = strings.TrimSpace(torrentID)
	if torrentID == "" {
		return nil, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, torrent_id, slug, name, ext, original_path, hls_path, size_bytes, status, progress_preview, transcoding_percent, error_message, created_at, updated_at
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
			progress_preview = 1,
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
		SELECT id, torrent_id, slug, name, ext, original_path, hls_path, size_bytes, status, progress_preview, transcoding_percent, error_message, created_at, updated_at
		FROM torrent_files
		WHERE torrent_id = ? AND original_path = ?
	`, torrentID, originalPath))
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
	var originalPath sql.NullString
	var hlsPath sql.NullString
	var progressPreview int
	var errorMessage sql.NullString

	if err := row.Scan(
		&file.ID,
		&file.TorrentID,
		&file.Slug,
		&file.Name,
		&file.Ext,
		&originalPath,
		&hlsPath,
		&file.SizeBytes,
		&file.Status,
		&progressPreview,
		&file.TranscodingPercent,
		&errorMessage,
		&file.CreatedAt,
		&file.UpdatedAt,
	); err != nil {
		return TorrentFile{}, err
	}

	file.OriginalPath = originalPath.String
	file.HLSPath = hlsPath.String
	file.ProgressPreview = progressPreview == 1
	file.ErrorMessage = errorMessage.String

	return file, nil
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

package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
)

const (
	TypeHLSTranscode = "hls_transcode"

	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCanceled  = "canceled"
)

var (
	ErrInvalidTransition = errors.New("jobs: invalid transition")
	ErrNotFound          = errors.New("jobs: not found")
)

type Store struct {
	db *sql.DB
}

type Job struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	PayloadJSON string `json:"payloadJson"`
	DedupeKey   string `json:"dedupeKey,omitempty"`
	Attempts    int    `json:"attempts"`
	MaxAttempts int    `json:"maxAttempts"`
	LeaseUntil  string `json:"leaseUntil,omitempty"`
	LockedBy    string `json:"lockedBy,omitempty"`
	LastError   string `json:"lastError,omitempty"`
	AvailableAt string `json:"availableAt"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type JobRecord struct {
	Job
	TorrentID       string  `json:"torrentId,omitempty"`
	TorrentFileID   string  `json:"torrentFileId,omitempty"`
	Target          string  `json:"target,omitempty"`
	ProgressPercent float64 `json:"progressPercent"`
}

type HLSTranscodePayload struct {
	TorrentFileID string `json:"torrentFileId"`
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func HLSTranscodeDedupeKey(torrentFileID string) string {
	return TypeHLSTranscode + ":" + strings.TrimSpace(torrentFileID)
}

func (s *Store) CreateHLSTranscodeJobIfMissing(ctx context.Context, torrentFileID string) (Job, bool, error) {
	torrentFileID = strings.TrimSpace(torrentFileID)
	if torrentFileID == "" {
		return Job{}, false, fmt.Errorf("torrent file id cannot be empty")
	}

	dedupeKey := HLSTranscodeDedupeKey(torrentFileID)
	payload, err := json.Marshal(HLSTranscodePayload{TorrentFileID: torrentFileID})
	if err != nil {
		return Job{}, false, fmt.Errorf("marshal hls transcode job payload: %w", err)
	}

	id, err := auth.NewID("job")
	if err != nil {
		return Job{}, false, err
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO jobs (id, type, status, payload_json, dedupe_key)
		VALUES (?, ?, ?, ?, ?)
	`, id, TypeHLSTranscode, StatusQueued, string(payload), dedupeKey)
	if err != nil {
		return Job{}, false, fmt.Errorf("create hls transcode job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Job{}, false, fmt.Errorf("read create hls transcode job rows affected: %w", err)
	}

	if rowsAffected == 0 {
		job, err := s.FindJobByDedupeKey(ctx, dedupeKey)
		return job, false, err
	}

	job, err := s.FindJobByID(ctx, id)
	return job, true, err
}

func (s *Store) FindJobByID(ctx context.Context, id string) (Job, error) {
	return scanJob(s.db.QueryRowContext(ctx, `
		SELECT id, type, status, payload_json, dedupe_key, attempts, max_attempts, lease_until, locked_by, last_error, available_at, created_at, updated_at
		FROM jobs
		WHERE id = ?
	`, strings.TrimSpace(id)))
}

func (s *Store) FindJobByDedupeKey(ctx context.Context, dedupeKey string) (Job, error) {
	return scanJob(s.db.QueryRowContext(ctx, `
		SELECT id, type, status, payload_json, dedupe_key, attempts, max_attempts, lease_until, locked_by, last_error, available_at, created_at, updated_at
		FROM jobs
		WHERE dedupe_key = ?
	`, strings.TrimSpace(dedupeKey)))
}

func (s *Store) ListJobsForOwner(ctx context.Context, ownerUserID string) ([]JobRecord, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, fmt.Errorf("owner user id cannot be empty")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT j.id, j.type, j.status, j.payload_json, j.dedupe_key, j.attempts, j.max_attempts, j.lease_until, j.locked_by, j.last_error, j.available_at, j.created_at, j.updated_at,
			tf.torrent_id, tf.id, tf.name, tf.transcoding_percent
		FROM jobs j
		INNER JOIN torrent_files tf ON j.dedupe_key = ? || tf.id
		INNER JOIN torrents t ON t.id = tf.torrent_id
		WHERE t.owner_user_id = ?
		ORDER BY
			CASE j.status
				WHEN 'running' THEN 0
				WHEN 'queued' THEN 1
				WHEN 'failed' THEN 2
				WHEN 'canceled' THEN 3
				ELSE 4
			END,
			j.updated_at DESC,
			j.created_at DESC,
			j.id DESC
	`, TypeHLSTranscode+":", ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list jobs for owner: %w", err)
	}
	defer rows.Close()

	records := make([]JobRecord, 0)
	for rows.Next() {
		record, err := scanJobRecordRow(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs for owner: %w", err)
	}

	return records, nil
}

func (s *Store) FindJobByIDForOwner(ctx context.Context, id string, ownerUserID string) (JobRecord, error) {
	return scanJobRecord(s.db.QueryRowContext(ctx, `
		SELECT j.id, j.type, j.status, j.payload_json, j.dedupe_key, j.attempts, j.max_attempts, j.lease_until, j.locked_by, j.last_error, j.available_at, j.created_at, j.updated_at,
			tf.torrent_id, tf.id, tf.name, tf.transcoding_percent
		FROM jobs j
		INNER JOIN torrent_files tf ON j.dedupe_key = ? || tf.id
		INNER JOIN torrents t ON t.id = tf.torrent_id
		WHERE j.id = ? AND t.owner_user_id = ?
	`, TypeHLSTranscode+":", strings.TrimSpace(id), strings.TrimSpace(ownerUserID)))
}

func (s *Store) ClaimNext(ctx context.Context, workerID string, leaseDuration time.Duration) (Job, bool, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return Job{}, false, fmt.Errorf("worker id cannot be empty")
	}
	if leaseDuration <= 0 {
		leaseDuration = 10 * time.Minute
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, false, fmt.Errorf("begin claim job: %w", err)
	}
	defer tx.Rollback()

	var id string
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM jobs
		WHERE type = ?
			AND status = ?
			AND attempts < max_attempts
			AND available_at <= CURRENT_TIMESTAMP
			AND (lease_until IS NULL OR lease_until <= CURRENT_TIMESTAMP)
		ORDER BY created_at ASC, id ASC
		LIMIT 1
	`, TypeHLSTranscode, StatusQueued).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return Job{}, false, nil
		}
		return Job{}, false, fmt.Errorf("select claim job: %w", err)
	}

	leaseSeconds := int64(leaseDuration.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 600
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?,
			attempts = attempts + 1,
			lease_until = DATETIME('now', ?),
			locked_by = ?,
			last_error = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, StatusRunning, fmt.Sprintf("+%d seconds", leaseSeconds), workerID, id); err != nil {
		return Job{}, false, fmt.Errorf("update claimed job: %w", err)
	}

	job, err := scanJob(tx.QueryRowContext(ctx, `
		SELECT id, type, status, payload_json, dedupe_key, attempts, max_attempts, lease_until, locked_by, last_error, available_at, created_at, updated_at
		FROM jobs
		WHERE id = ?
	`, id))
	if err != nil {
		return Job{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return Job{}, false, fmt.Errorf("commit claim job: %w", err)
	}

	return job, true, nil
}

func (s *Store) Complete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?,
			lease_until = NULL,
			locked_by = NULL,
			last_error = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, StatusSucceeded, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) Fail(ctx context.Context, id string, message string, retryDelay time.Duration) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "job failed"
	}
	if retryDelay < 0 {
		retryDelay = 0
	}

	delaySeconds := int64(retryDelay.Seconds())
	result, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = CASE WHEN attempts >= max_attempts THEN ? ELSE ? END,
			lease_until = NULL,
			locked_by = NULL,
			last_error = ?,
			available_at = CASE WHEN attempts >= max_attempts THEN available_at ELSE DATETIME('now', ?) END,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, StatusFailed, StatusQueued, message, fmt.Sprintf("+%d seconds", delaySeconds), strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("fail job: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) Retry(ctx context.Context, id string) (Job, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Job{}, ErrNotFound
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?,
			attempts = 0,
			lease_until = NULL,
			locked_by = NULL,
			last_error = NULL,
			available_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
			AND status IN (?, ?, ?)
	`, StatusQueued, id, StatusFailed, StatusCanceled, StatusSucceeded)
	if err != nil {
		return Job{}, fmt.Errorf("retry job: %w", err)
	}
	if err := s.checkTransitionRowsAffected(ctx, result, id); err != nil {
		return Job{}, err
	}

	return s.FindJobByID(ctx, id)
}

func (s *Store) Cancel(ctx context.Context, id string) (Job, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Job{}, ErrNotFound
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?,
			lease_until = NULL,
			locked_by = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
			AND status IN (?, ?)
	`, StatusCanceled, id, StatusQueued, StatusFailed)
	if err != nil {
		return Job{}, fmt.Errorf("cancel job: %w", err)
	}
	if err := s.checkTransitionRowsAffected(ctx, result, id); err != nil {
		return Job{}, err
	}

	return s.FindJobByID(ctx, id)
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

func (s *Store) checkTransitionRowsAffected(ctx context.Context, result sql.Result, id string) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	if _, err := s.FindJobByID(ctx, id); err != nil {
		return err
	}

	return ErrInvalidTransition
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	var dedupeKey sql.NullString
	var leaseUntil sql.NullString
	var lockedBy sql.NullString
	var lastError sql.NullString

	if err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Status,
		&job.PayloadJSON,
		&dedupeKey,
		&job.Attempts,
		&job.MaxAttempts,
		&leaseUntil,
		&lockedBy,
		&lastError,
		&job.AvailableAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return Job{}, ErrNotFound
		}
		return Job{}, fmt.Errorf("scan job: %w", err)
	}

	job.DedupeKey = dedupeKey.String
	job.LeaseUntil = leaseUntil.String
	job.LockedBy = lockedBy.String
	job.LastError = lastError.String

	return job, nil
}

func scanJobRecord(row rowScanner) (JobRecord, error) {
	record, err := scanJobRecordValues(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return JobRecord{}, ErrNotFound
		}
		return JobRecord{}, fmt.Errorf("scan job record: %w", err)
	}

	return record, nil
}

func scanJobRecordRow(row rowScanner) (JobRecord, error) {
	record, err := scanJobRecordValues(row)
	if err != nil {
		return JobRecord{}, fmt.Errorf("scan job record: %w", err)
	}

	return record, nil
}

func scanJobRecordValues(row rowScanner) (JobRecord, error) {
	var record JobRecord
	var dedupeKey sql.NullString
	var leaseUntil sql.NullString
	var lockedBy sql.NullString
	var lastError sql.NullString

	if err := row.Scan(
		&record.ID,
		&record.Type,
		&record.Status,
		&record.PayloadJSON,
		&dedupeKey,
		&record.Attempts,
		&record.MaxAttempts,
		&leaseUntil,
		&lockedBy,
		&lastError,
		&record.AvailableAt,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.TorrentID,
		&record.TorrentFileID,
		&record.Target,
		&record.ProgressPercent,
	); err != nil {
		return JobRecord{}, err
	}

	record.DedupeKey = dedupeKey.String
	record.LeaseUntil = leaseUntil.String
	record.LockedBy = lockedBy.String
	record.LastError = lastError.String

	return record, nil
}

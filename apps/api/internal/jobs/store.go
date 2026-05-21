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
	TypeHLSTranscode    = "hls_transcode"
	TypeSubtitleExtract = "subtitle_extract"
	TypeSpriteGenerate  = "sprite_generate"

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
	ID              string  `json:"id"`
	Type            string  `json:"type"`
	Status          string  `json:"status"`
	PayloadJSON     string  `json:"payloadJson"`
	DedupeKey       string  `json:"dedupeKey,omitempty"`
	Attempts        int     `json:"attempts"`
	MaxAttempts     int     `json:"maxAttempts"`
	ProgressPercent float64 `json:"progressPercent"`
	LeaseUntil      string  `json:"leaseUntil,omitempty"`
	LockedBy        string  `json:"lockedBy,omitempty"`
	LastError       string  `json:"lastError,omitempty"`
	AvailableAt     string  `json:"availableAt"`
	StartedAt       string  `json:"startedAt,omitempty"`
	FinishedAt      string  `json:"finishedAt,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type JobRecord struct {
	Job
	TorrentID     string `json:"torrentId,omitempty"`
	TorrentFileID string `json:"torrentFileId,omitempty"`
	Target        string `json:"target,omitempty"`
}

type MediaFilePayload struct {
	TorrentFileID string `json:"torrentFileId"`
}

type HLSTranscodePayload = MediaFilePayload

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func MediaJobDedupeKey(jobType string, torrentFileID string) string {
	return strings.TrimSpace(jobType) + ":" + strings.TrimSpace(torrentFileID)
}

func HLSTranscodeDedupeKey(torrentFileID string) string {
	return MediaJobDedupeKey(TypeHLSTranscode, torrentFileID)
}

func SubtitleExtractDedupeKey(torrentFileID string) string {
	return MediaJobDedupeKey(TypeSubtitleExtract, torrentFileID)
}

func SpriteGenerateDedupeKey(torrentFileID string) string {
	return MediaJobDedupeKey(TypeSpriteGenerate, torrentFileID)
}

func (s *Store) CreateHLSTranscodeJobIfMissing(ctx context.Context, torrentFileID string) (Job, bool, error) {
	return s.createMediaFileJobIfMissing(ctx, TypeHLSTranscode, torrentFileID)
}

func (s *Store) CreateSubtitleExtractJobIfMissing(ctx context.Context, torrentFileID string) (Job, bool, error) {
	return s.createMediaFileJobIfMissing(ctx, TypeSubtitleExtract, torrentFileID)
}

func (s *Store) CreateSpriteGenerateJobIfMissing(ctx context.Context, torrentFileID string) (Job, bool, error) {
	return s.createMediaFileJobIfMissing(ctx, TypeSpriteGenerate, torrentFileID)
}

func (s *Store) createMediaFileJobIfMissing(ctx context.Context, jobType string, torrentFileID string) (Job, bool, error) {
	jobType = strings.TrimSpace(jobType)
	torrentFileID = strings.TrimSpace(torrentFileID)
	if !isMediaFileJobType(jobType) {
		return Job{}, false, fmt.Errorf("unsupported media job type %q", jobType)
	}
	if torrentFileID == "" {
		return Job{}, false, fmt.Errorf("torrent file id cannot be empty")
	}

	dedupeKey := MediaJobDedupeKey(jobType, torrentFileID)
	payload, err := json.Marshal(MediaFilePayload{TorrentFileID: torrentFileID})
	if err != nil {
		return Job{}, false, fmt.Errorf("marshal media file job payload: %w", err)
	}

	id, err := auth.NewID("job")
	if err != nil {
		return Job{}, false, err
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO jobs (id, type, status, payload_json, dedupe_key)
		VALUES (?, ?, ?, ?, ?)
	`, id, jobType, StatusQueued, string(payload), dedupeKey)
	if err != nil {
		return Job{}, false, fmt.Errorf("create media file job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Job{}, false, fmt.Errorf("read create media file job rows affected: %w", err)
	}

	if rowsAffected == 0 {
		job, err := s.FindJobByDedupeKey(ctx, dedupeKey)
		return job, false, err
	}

	job, err := s.FindJobByID(ctx, id)
	return job, true, err
}

func isMediaFileJobType(jobType string) bool {
	switch strings.TrimSpace(jobType) {
	case TypeHLSTranscode, TypeSubtitleExtract, TypeSpriteGenerate:
		return true
	default:
		return false
	}
}

func (s *Store) FindJobByID(ctx context.Context, id string) (Job, error) {
	return scanJob(s.db.QueryRowContext(ctx, `
		SELECT id, type, status, payload_json, dedupe_key, attempts, max_attempts, progress_percent, lease_until, locked_by, last_error, available_at, started_at, finished_at, created_at, updated_at
		FROM jobs
		WHERE id = ?
	`, strings.TrimSpace(id)))
}

func (s *Store) FindJobByDedupeKey(ctx context.Context, dedupeKey string) (Job, error) {
	return scanJob(s.db.QueryRowContext(ctx, `
		SELECT id, type, status, payload_json, dedupe_key, attempts, max_attempts, progress_percent, lease_until, locked_by, last_error, available_at, started_at, finished_at, created_at, updated_at
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
		SELECT j.id, j.type, j.status, j.payload_json, j.dedupe_key, j.attempts, j.max_attempts, j.progress_percent, j.lease_until, j.locked_by, j.last_error, j.available_at, j.started_at, j.finished_at, j.created_at, j.updated_at,
			tf.torrent_id, tf.id, tf.name
		FROM jobs j
		INNER JOIN torrent_files tf ON j.dedupe_key IN (? || tf.id, ? || tf.id, ? || tf.id)
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
	`, TypeHLSTranscode+":", TypeSubtitleExtract+":", TypeSpriteGenerate+":", ownerUserID)
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
		SELECT j.id, j.type, j.status, j.payload_json, j.dedupe_key, j.attempts, j.max_attempts, j.progress_percent, j.lease_until, j.locked_by, j.last_error, j.available_at, j.started_at, j.finished_at, j.created_at, j.updated_at,
			tf.torrent_id, tf.id, tf.name
		FROM jobs j
		INNER JOIN torrent_files tf ON j.dedupe_key IN (? || tf.id, ? || tf.id, ? || tf.id)
		INNER JOIN torrents t ON t.id = tf.torrent_id
		WHERE j.id = ? AND t.owner_user_id = ?
	`, TypeHLSTranscode+":", TypeSubtitleExtract+":", TypeSpriteGenerate+":", strings.TrimSpace(id), strings.TrimSpace(ownerUserID)))
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
		WHERE type IN (?, ?, ?)
			AND status = ?
			AND attempts < max_attempts
			AND available_at <= CURRENT_TIMESTAMP
			AND (lease_until IS NULL OR lease_until <= CURRENT_TIMESTAMP)
		ORDER BY
			CASE type
				WHEN 'hls_transcode' THEN 0
				WHEN 'subtitle_extract' THEN 1
				WHEN 'sprite_generate' THEN 2
				ELSE 3
			END,
			created_at ASC,
			id ASC
		LIMIT 1
	`, TypeHLSTranscode, TypeSubtitleExtract, TypeSpriteGenerate, StatusQueued).Scan(&id); err != nil {
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
			started_at = COALESCE(started_at, CURRENT_TIMESTAMP),
			finished_at = NULL,
			lease_until = DATETIME('now', ?),
			locked_by = ?,
			last_error = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, StatusRunning, fmt.Sprintf("+%d seconds", leaseSeconds), workerID, id); err != nil {
		return Job{}, false, fmt.Errorf("update claimed job: %w", err)
	}

	job, err := scanJob(tx.QueryRowContext(ctx, `
		SELECT id, type, status, payload_json, dedupe_key, attempts, max_attempts, progress_percent, lease_until, locked_by, last_error, available_at, started_at, finished_at, created_at, updated_at
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
			progress_percent = 100,
			lease_until = NULL,
			locked_by = NULL,
			last_error = NULL,
			finished_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, StatusSucceeded, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *Store) UpdateProgress(ctx context.Context, id string, percent float64) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET progress_percent = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
			AND status = ?
	`, clampPercent(percent), id, StatusRunning)
	if err != nil {
		return fmt.Errorf("update job progress: %w", err)
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
			progress_percent = CASE WHEN attempts >= max_attempts THEN progress_percent ELSE 0 END,
			lease_until = NULL,
			locked_by = NULL,
			last_error = ?,
			available_at = CASE WHEN attempts >= max_attempts THEN available_at ELSE DATETIME('now', ?) END,
			started_at = CASE WHEN attempts >= max_attempts THEN started_at ELSE NULL END,
			finished_at = CASE WHEN attempts >= max_attempts THEN CURRENT_TIMESTAMP ELSE NULL END,
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
			progress_percent = 0,
			lease_until = NULL,
			locked_by = NULL,
			last_error = NULL,
			available_at = CURRENT_TIMESTAMP,
			started_at = NULL,
			finished_at = NULL,
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
			finished_at = CURRENT_TIMESTAMP,
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

// RequeueStale recovers jobs left in the running state by a worker that exited
// without releasing them — a crash or a server restart mid-job. A job is
// considered stale when it is locked by the given worker (a same-host restart
// reuses the worker ID) or when its lease has expired. Stale jobs with retry
// budget left go back to queued and become immediately claimable; jobs that
// have exhausted their attempts are marked failed so they stay visible and
// remain retryable through the API.
func (s *Store) RequeueStale(ctx context.Context, workerID string) (int64, error) {
	workerID = strings.TrimSpace(workerID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = CASE WHEN attempts >= max_attempts THEN ? ELSE ? END,
			progress_percent = CASE WHEN attempts >= max_attempts THEN progress_percent ELSE 0 END,
			lease_until = NULL,
			locked_by = NULL,
			last_error = ?,
			available_at = CASE WHEN attempts >= max_attempts THEN available_at ELSE CURRENT_TIMESTAMP END,
			started_at = CASE WHEN attempts >= max_attempts THEN started_at ELSE NULL END,
			finished_at = CASE WHEN attempts >= max_attempts THEN CURRENT_TIMESTAMP ELSE NULL END,
			updated_at = CURRENT_TIMESTAMP
		WHERE status = ?
			AND (
				(? <> '' AND locked_by = ?)
				OR lease_until IS NULL
				OR lease_until <= CURRENT_TIMESTAMP
			)
	`, StatusFailed, StatusQueued, "worker exited before the job finished", StatusRunning, workerID, workerID)
	if err != nil {
		return 0, fmt.Errorf("requeue stale jobs: %w", err)
	}

	return result.RowsAffected()
}

// RenewLease extends the lease on a running job so a long-running task (a large
// transcode) is not reclaimed as stale while its worker is alive and working.
// It only touches a job still owned by the given worker; a no-op match (the job
// already finished or was reclaimed) is not an error.
func (s *Store) RenewLease(ctx context.Context, id string, workerID string, leaseDuration time.Duration) error {
	id = strings.TrimSpace(id)
	workerID = strings.TrimSpace(workerID)
	if id == "" || workerID == "" {
		return ErrNotFound
	}
	if leaseDuration <= 0 {
		leaseDuration = 10 * time.Minute
	}

	leaseSeconds := int64(leaseDuration.Seconds())
	if leaseSeconds <= 0 {
		leaseSeconds = 600
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET lease_until = DATETIME('now', ?),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
			AND status = ?
			AND locked_by = ?
	`, fmt.Sprintf("+%d seconds", leaseSeconds), id, StatusRunning, workerID); err != nil {
		return fmt.Errorf("renew job lease: %w", err)
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

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
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
	var startedAt sql.NullString
	var finishedAt sql.NullString

	if err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Status,
		&job.PayloadJSON,
		&dedupeKey,
		&job.Attempts,
		&job.MaxAttempts,
		&job.ProgressPercent,
		&leaseUntil,
		&lockedBy,
		&lastError,
		&job.AvailableAt,
		&startedAt,
		&finishedAt,
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
	job.StartedAt = startedAt.String
	job.FinishedAt = finishedAt.String

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
	var startedAt sql.NullString
	var finishedAt sql.NullString

	if err := row.Scan(
		&record.ID,
		&record.Type,
		&record.Status,
		&record.PayloadJSON,
		&dedupeKey,
		&record.Attempts,
		&record.MaxAttempts,
		&record.ProgressPercent,
		&leaseUntil,
		&lockedBy,
		&lastError,
		&record.AvailableAt,
		&startedAt,
		&finishedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.TorrentID,
		&record.TorrentFileID,
		&record.Target,
	); err != nil {
		return JobRecord{}, err
	}

	record.DedupeKey = dedupeKey.String
	record.LeaseUntil = leaseUntil.String
	record.LockedBy = lockedBy.String
	record.LastError = lastError.String
	record.StartedAt = startedAt.String
	record.FinishedAt = finishedAt.String

	return record, nil
}

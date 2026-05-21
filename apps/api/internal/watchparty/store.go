package watchparty

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/binoy638/streamize-api/apps/api/internal/auth"
)

const (
	ControlModeHostOnly = "host_only"
	ControlModeEveryone = "everyone"

	StatusActive = "active"
	StatusEnded  = "ended"

	RoleHost  = "host"
	RoleGuest = "guest"
)

var (
	ErrForbidden = errors.New("watchparty: forbidden")
	ErrNotFound  = errors.New("watchparty: not found")
)

type Store struct {
	db *sql.DB
}

type WatchParty struct {
	ID                     string  `json:"id"`
	OwnerUserID            string  `json:"ownerUserId"`
	Slug                   string  `json:"slug"`
	TorrentFileID          string  `json:"torrentFileId"`
	ControlMode            string  `json:"controlMode"`
	Status                 string  `json:"status"`
	CurrentPositionSeconds float64 `json:"currentPositionSeconds"`
	DurationSeconds        float64 `json:"durationSeconds"`
	IsPlaying              bool    `json:"isPlaying"`
	LastEventAt            string  `json:"lastEventAt"`
	ExpiresAt              string  `json:"expiresAt"`
	CreatedAt              string  `json:"createdAt"`
	UpdatedAt              string  `json:"updatedAt"`
}

type Participant struct {
	ID           string `json:"id"`
	WatchPartyID string `json:"watchPartyId"`
	UserID       string `json:"userId,omitempty"`
	DisplayName  string `json:"displayName"`
	Role         string `json:"role"`
	Connected    bool   `json:"connected"`
	CreatedAt    string `json:"createdAt"`
	LastSeenAt   string `json:"lastSeenAt"`
}

type CreatePartyParams struct {
	OwnerUserID   string
	TorrentFileID string
	ControlMode   string
	DisplayName   string
	ExpiresAt     time.Time
}

type JoinPartyParams struct {
	PartyID     string
	UserID      string
	DisplayName string
	Role        string
}

type UpdatePlaybackParams struct {
	ID              string
	PositionSeconds float64
	DurationSeconds float64
	IsPlaying       bool
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateParty(ctx context.Context, params CreatePartyParams) (WatchParty, Participant, string, error) {
	ownerUserID := strings.TrimSpace(params.OwnerUserID)
	torrentFileID := strings.TrimSpace(params.TorrentFileID)
	if ownerUserID == "" || torrentFileID == "" {
		return WatchParty{}, Participant{}, "", fmt.Errorf("owner user id and torrent file id are required")
	}

	controlMode := normalizeControlMode(params.ControlMode)
	displayName := normalizeDisplayName(params.DisplayName, "Host")
	expiresAt := params.ExpiresAt.UTC()
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(24 * time.Hour)
	}

	partyID, err := auth.NewID("wpy")
	if err != nil {
		return WatchParty{}, Participant{}, "", err
	}
	slug, err := auth.NewID("wp")
	if err != nil {
		return WatchParty{}, Participant{}, "", err
	}
	participantID, err := auth.NewID("wpp")
	if err != nil {
		return WatchParty{}, Participant{}, "", err
	}
	token, err := auth.NewSessionToken()
	if err != nil {
		return WatchParty{}, Participant{}, "", err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WatchParty{}, Participant{}, "", fmt.Errorf("begin create watch party: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO watch_parties (id, owner_user_id, slug, torrent_file_id, control_mode, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, partyID, ownerUserID, slug, torrentFileID, controlMode, expiresAt.Format(time.RFC3339)); err != nil {
		return WatchParty{}, Participant{}, "", fmt.Errorf("create watch party: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO watch_party_participants (id, watch_party_id, user_id, display_name, role, token_hash)
		VALUES (?, ?, ?, ?, ?, ?)
	`, participantID, partyID, ownerUserID, displayName, RoleHost, auth.HashToken(token)); err != nil {
		return WatchParty{}, Participant{}, "", fmt.Errorf("create host participant: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return WatchParty{}, Participant{}, "", fmt.Errorf("commit create watch party: %w", err)
	}

	party, err := s.FindPartyByID(ctx, partyID)
	if err != nil {
		return WatchParty{}, Participant{}, "", err
	}
	participant, err := s.FindParticipantByID(ctx, participantID)
	if err != nil {
		return WatchParty{}, Participant{}, "", err
	}

	return party, participant, token, nil
}

func (s *Store) ListPartiesByOwner(ctx context.Context, ownerUserID string) ([]WatchParty, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, owner_user_id, slug, torrent_file_id, control_mode, status, current_position_seconds, duration_seconds, is_playing, last_event_at, expires_at, created_at, updated_at
		FROM watch_parties
		WHERE owner_user_id = ?
		ORDER BY created_at DESC, id DESC
	`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, fmt.Errorf("list watch parties: %w", err)
	}
	defer rows.Close()

	parties := make([]WatchParty, 0)
	for rows.Next() {
		party, err := scanPartyRow(rows)
		if err != nil {
			return nil, err
		}
		parties = append(parties, party)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate watch parties: %w", err)
	}

	return parties, nil
}

func (s *Store) FindPartyByID(ctx context.Context, id string) (WatchParty, error) {
	return scanParty(s.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, slug, torrent_file_id, control_mode, status, current_position_seconds, duration_seconds, is_playing, last_event_at, expires_at, created_at, updated_at
		FROM watch_parties
		WHERE id = ?
	`, strings.TrimSpace(id)))
}

func (s *Store) FindPartyBySlug(ctx context.Context, slug string) (WatchParty, error) {
	return scanParty(s.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, slug, torrent_file_id, control_mode, status, current_position_seconds, duration_seconds, is_playing, last_event_at, expires_at, created_at, updated_at
		FROM watch_parties
		WHERE slug = ?
	`, strings.TrimSpace(slug)))
}

func (s *Store) EndParty(ctx context.Context, id string, ownerUserID string) (WatchParty, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE watch_parties
		SET status = ?, is_playing = 0, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND owner_user_id = ?
	`, StatusEnded, strings.TrimSpace(id), strings.TrimSpace(ownerUserID))
	if err != nil {
		return WatchParty{}, fmt.Errorf("end watch party: %w", err)
	}
	if err := checkRowsAffected(result); err != nil {
		return WatchParty{}, err
	}

	return s.FindPartyByID(ctx, id)
}

func (s *Store) JoinParty(ctx context.Context, params JoinPartyParams) (Participant, string, error) {
	partyID := strings.TrimSpace(params.PartyID)
	if partyID == "" {
		return Participant{}, "", ErrNotFound
	}

	role := normalizeRole(params.Role)
	displayName := normalizeDisplayName(params.DisplayName, role)
	participantID, err := auth.NewID("wpp")
	if err != nil {
		return Participant{}, "", err
	}
	token, err := auth.NewSessionToken()
	if err != nil {
		return Participant{}, "", err
	}

	var userID any
	if trimmed := strings.TrimSpace(params.UserID); trimmed != "" {
		userID = trimmed
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO watch_party_participants (id, watch_party_id, user_id, display_name, role, token_hash)
		VALUES (?, ?, ?, ?, ?, ?)
	`, participantID, partyID, userID, displayName, role, auth.HashToken(token)); err != nil {
		return Participant{}, "", fmt.Errorf("join watch party: %w", err)
	}

	participant, err := s.FindParticipantByID(ctx, participantID)
	return participant, token, err
}

func (s *Store) FindParticipantByID(ctx context.Context, id string) (Participant, error) {
	return scanParticipant(s.db.QueryRowContext(ctx, `
		SELECT id, watch_party_id, user_id, display_name, role, connected, created_at, last_seen_at
		FROM watch_party_participants
		WHERE id = ?
	`, strings.TrimSpace(id)))
}

func (s *Store) FindParticipantByToken(ctx context.Context, partyID string, participantID string, token string) (Participant, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Participant{}, ErrForbidden
	}

	participant, tokenHash, err := scanParticipantWithToken(s.db.QueryRowContext(ctx, `
		SELECT id, watch_party_id, user_id, display_name, role, connected, created_at, last_seen_at, token_hash
		FROM watch_party_participants
		WHERE id = ? AND watch_party_id = ?
	`, strings.TrimSpace(participantID), strings.TrimSpace(partyID)))
	if err != nil {
		return Participant{}, err
	}
	if tokenHash != auth.HashToken(token) {
		return Participant{}, ErrForbidden
	}

	return participant, nil
}

func (s *Store) ListConnectedParticipants(ctx context.Context, partyID string) ([]Participant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, watch_party_id, user_id, display_name, role, connected, created_at, last_seen_at
		FROM watch_party_participants
		WHERE watch_party_id = ? AND connected = 1
		ORDER BY role DESC, display_name ASC, created_at ASC
	`, strings.TrimSpace(partyID))
	if err != nil {
		return nil, fmt.Errorf("list connected participants: %w", err)
	}
	defer rows.Close()

	participants := make([]Participant, 0)
	for rows.Next() {
		participant, err := scanParticipantRow(rows)
		if err != nil {
			return nil, err
		}
		participants = append(participants, participant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connected participants: %w", err)
	}

	return participants, nil
}

func (s *Store) SetParticipantConnected(ctx context.Context, participantID string, connected bool) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE watch_party_participants
		SET connected = ?, last_seen_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, boolInt(connected), strings.TrimSpace(participantID)); err != nil {
		return fmt.Errorf("set participant connected: %w", err)
	}
	return nil
}

func (s *Store) UpdatePlayback(ctx context.Context, params UpdatePlaybackParams) (WatchParty, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE watch_parties
		SET current_position_seconds = ?,
			duration_seconds = ?,
			is_playing = ?,
			last_event_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = ?
	`, clampSeconds(params.PositionSeconds), clampSeconds(params.DurationSeconds), boolInt(params.IsPlaying), strings.TrimSpace(params.ID), StatusActive)
	if err != nil {
		return WatchParty{}, fmt.Errorf("update watch party playback: %w", err)
	}
	if err := checkRowsAffected(result); err != nil {
		return WatchParty{}, err
	}

	return s.FindPartyByID(ctx, params.ID)
}

func (party WatchParty) Joinable(now time.Time) bool {
	if party.Status != StatusActive {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, party.ExpiresAt)
	if err != nil {
		return false
	}
	return now.UTC().Before(expiresAt)
}

func normalizeControlMode(value string) string {
	switch strings.TrimSpace(value) {
	case ControlModeEveryone:
		return ControlModeEveryone
	default:
		return ControlModeHostOnly
	}
}

func normalizeRole(value string) string {
	if strings.TrimSpace(value) == RoleHost {
		return RoleHost
	}
	return RoleGuest
}

func normalizeDisplayName(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if value == "" {
		value = "Guest"
	}
	if len(value) > 48 {
		value = value[:48]
	}
	return value
}

func clampSeconds(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanParty(row rowScanner) (WatchParty, error) {
	party, err := scanPartyValues(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return WatchParty{}, ErrNotFound
		}
		return WatchParty{}, fmt.Errorf("scan watch party: %w", err)
	}
	return party, nil
}

func scanPartyRow(row rowScanner) (WatchParty, error) {
	party, err := scanPartyValues(row)
	if err != nil {
		return WatchParty{}, fmt.Errorf("scan watch party: %w", err)
	}
	return party, nil
}

func scanPartyValues(row rowScanner) (WatchParty, error) {
	var party WatchParty
	var isPlaying int
	if err := row.Scan(
		&party.ID,
		&party.OwnerUserID,
		&party.Slug,
		&party.TorrentFileID,
		&party.ControlMode,
		&party.Status,
		&party.CurrentPositionSeconds,
		&party.DurationSeconds,
		&isPlaying,
		&party.LastEventAt,
		&party.ExpiresAt,
		&party.CreatedAt,
		&party.UpdatedAt,
	); err != nil {
		return WatchParty{}, err
	}
	party.IsPlaying = isPlaying == 1
	return party, nil
}

func scanParticipant(row rowScanner) (Participant, error) {
	participant, err := scanParticipantValues(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return Participant{}, ErrNotFound
		}
		return Participant{}, fmt.Errorf("scan watch party participant: %w", err)
	}
	return participant, nil
}

func scanParticipantRow(row rowScanner) (Participant, error) {
	participant, err := scanParticipantValues(row)
	if err != nil {
		return Participant{}, fmt.Errorf("scan watch party participant: %w", err)
	}
	return participant, nil
}

func scanParticipantWithToken(row rowScanner) (Participant, string, error) {
	var tokenHash string
	participant, err := scanParticipantValuesWithExtra(row, &tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return Participant{}, "", ErrNotFound
		}
		return Participant{}, "", fmt.Errorf("scan watch party participant: %w", err)
	}
	return participant, tokenHash, nil
}

func scanParticipantValues(row rowScanner) (Participant, error) {
	return scanParticipantValuesWithExtra(row)
}

func scanParticipantValuesWithExtra(row rowScanner, extra ...any) (Participant, error) {
	var participant Participant
	var userID sql.NullString
	var connected int
	dest := []any{
		&participant.ID,
		&participant.WatchPartyID,
		&userID,
		&participant.DisplayName,
		&participant.Role,
		&connected,
		&participant.CreatedAt,
		&participant.LastSeenAt,
	}
	dest = append(dest, extra...)
	if err := row.Scan(dest...); err != nil {
		return Participant{}, err
	}
	participant.UserID = userID.String
	participant.Connected = connected == 1
	return participant, nil
}

package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/binoy638/streamize-api/apps/api/internal/config"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

var (
	ErrConflict           = errors.New("auth: conflict")
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrNotFound           = errors.New("auth: not found")
	ErrUnauthorized       = errors.New("auth: unauthorized")
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	Role              string `json:"role"`
	StorageQuotaBytes int64  `json:"storageQuotaBytes"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type CreateUserParams struct {
	Username          string
	Password          string
	Role              string
	StorageQuotaBytes int64
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) BootstrapAdmin(ctx context.Context, cfg config.Config) error {
	username := normalizeUsername(cfg.AdminUsername)
	if username == "" {
		return fmt.Errorf("admin username cannot be empty")
	}

	user, err := s.FindUserByUsername(ctx, username)
	if err == nil {
		if user.Role == RoleAdmin && user.StorageQuotaBytes == cfg.AdminStorageQuota {
			return nil
		}

		if _, err := s.db.ExecContext(ctx, `
			UPDATE users
			SET role = ?, storage_quota_bytes = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, RoleAdmin, cfg.AdminStorageQuota, user.ID); err != nil {
			return fmt.Errorf("update bootstrap admin: %w", err)
		}

		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}

	_, err = s.CreateUser(ctx, CreateUserParams{
		Username:          username,
		Password:          cfg.AdminPassword,
		Role:              RoleAdmin,
		StorageQuotaBytes: cfg.AdminStorageQuota,
	})
	if err != nil {
		return fmt.Errorf("create bootstrap admin: %w", err)
	}

	return nil
}

func (s *Store) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	username := normalizeUsername(params.Username)
	if username == "" {
		return User{}, fmt.Errorf("username cannot be empty")
	}
	if len(username) < 3 {
		return User{}, fmt.Errorf("username must be at least 3 characters")
	}
	if params.StorageQuotaBytes < 0 {
		return User{}, fmt.Errorf("storage quota cannot be negative")
	}

	role := normalizeRole(params.Role)
	passwordHash, err := HashPassword(params.Password)
	if err != nil {
		return User{}, err
	}

	id, err := NewID("usr")
	if err != nil {
		return User{}, err
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, storage_quota_bytes)
		VALUES (?, ?, ?, ?, ?)
	`, id, username, passwordHash, role, params.StorageQuotaBytes); err != nil {
		if isUniqueConstraint(err) {
			return User{}, ErrConflict
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}

	return s.FindUserByID(ctx, id)
}

func (s *Store) Authenticate(ctx context.Context, username string, password string) (User, error) {
	username = normalizeUsername(username)
	if username == "" || password == "" {
		return User{}, ErrInvalidCredentials
	}

	user, passwordHash, err := s.findUserWithPasswordByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return User{}, ErrInvalidCredentials
		}
		return User{}, err
	}

	if !CheckPassword(password, passwordHash) {
		return User{}, ErrInvalidCredentials
	}

	return user, nil
}

func (s *Store) FindUserByID(ctx context.Context, id string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `
		SELECT id, username, role, storage_quota_bytes, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id))
}

func (s *Store) FindUserByUsername(ctx context.Context, username string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `
		SELECT id, username, role, storage_quota_bytes, created_at, updated_at
		FROM users
		WHERE username = ?
	`, normalizeUsername(username)))
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, role, storage_quota_bytes, created_at, updated_at
		FROM users
		ORDER BY created_at ASC, username ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.StorageQuotaBytes, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

func (s *Store) CreateSession(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user id cannot be empty")
	}
	if ttl <= 0 {
		return "", fmt.Errorf("session ttl must be greater than zero")
	}

	id, err := NewID("ses")
	if err != nil {
		return "", err
	}

	token, err := NewSessionToken()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().UTC().Add(ttl).Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, id, userID, HashToken(token), expiresAt); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return token, nil
}

func (s *Store) FindUserBySessionToken(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}

	var user User
	var expiresAtRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT users.id, users.username, users.role, users.storage_quota_bytes, users.created_at, users.updated_at, sessions.expires_at
		FROM sessions
		INNER JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = ?
	`, HashToken(token)).Scan(
		&user.ID,
		&user.Username,
		&user.Role,
		&user.StorageQuotaBytes,
		&user.CreatedAt,
		&user.UpdatedAt,
		&expiresAtRaw,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, ErrUnauthorized
		}
		return User{}, fmt.Errorf("find session: %w", err)
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtRaw)
	if err != nil {
		return User{}, fmt.Errorf("parse session expiry: %w", err)
	}
	if !expiresAt.After(time.Now().UTC()) {
		_ = s.DeleteSessionByToken(ctx, token)
		return User{}, ErrUnauthorized
	}

	return user, nil
}

func (s *Store) DeleteSessionByToken(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	if _, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", HashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (s *Store) findUserWithPasswordByUsername(ctx context.Context, username string) (User, string, error) {
	var user User
	var passwordHash string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, role, storage_quota_bytes, created_at, updated_at, password_hash
		FROM users
		WHERE username = ?
	`, normalizeUsername(username)).Scan(
		&user.ID,
		&user.Username,
		&user.Role,
		&user.StorageQuotaBytes,
		&user.CreatedAt,
		&user.UpdatedAt,
		&passwordHash,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, "", ErrNotFound
		}
		return User{}, "", fmt.Errorf("find user by username: %w", err)
	}

	return user, passwordHash, nil
}

func scanUser(row *sql.Row) (User, error) {
	var user User
	if err := row.Scan(&user.ID, &user.Username, &user.Role, &user.StorageQuotaBytes, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("scan user: %w", err)
	}

	return user, nil
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleAdmin:
		return RoleAdmin
	default:
		return RoleUser
	}
}

func isUniqueConstraint(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}

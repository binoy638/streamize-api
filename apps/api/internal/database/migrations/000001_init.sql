CREATE TABLE users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
	storage_quota_bytes INTEGER NOT NULL DEFAULT 0 CHECK (storage_quota_bytes >= 0),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE torrents (
	id TEXT PRIMARY KEY,
	owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	slug TEXT NOT NULL UNIQUE,
	magnet_uri TEXT NOT NULL,
	info_hash TEXT,
	qbt_hash TEXT,
	name TEXT,
	size_bytes INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
	status TEXT NOT NULL DEFAULT 'added' CHECK (status IN ('added', 'downloading', 'paused', 'queued', 'processing', 'done', 'error')),
	retention_policy TEXT NOT NULL DEFAULT 'keep' CHECK (retention_policy IN ('keep', 'delete_after_hls')),
	error_message TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_torrents_owner_user_id ON torrents(owner_user_id);
CREATE INDEX idx_torrents_info_hash ON torrents(info_hash);
CREATE INDEX idx_torrents_qbt_hash ON torrents(qbt_hash);
CREATE INDEX idx_torrents_status ON torrents(status);

CREATE TABLE torrent_files (
	id TEXT PRIMARY KEY,
	torrent_id TEXT NOT NULL REFERENCES torrents(id) ON DELETE CASCADE,
	slug TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	ext TEXT NOT NULL DEFAULT '',
	original_path TEXT,
	hls_path TEXT,
	size_bytes INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
	status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('downloading', 'queued', 'processing', 'done', 'error')),
	progress_preview INTEGER NOT NULL DEFAULT 0 CHECK (progress_preview IN (0, 1)),
	transcoding_percent REAL NOT NULL DEFAULT 0 CHECK (transcoding_percent >= 0 AND transcoding_percent <= 100),
	error_message TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_torrent_files_torrent_id ON torrent_files(torrent_id);
CREATE INDEX idx_torrent_files_status ON torrent_files(status);

CREATE TABLE subtitles (
	id TEXT PRIMARY KEY,
	torrent_file_id TEXT NOT NULL REFERENCES torrent_files(id) ON DELETE CASCADE,
	file_name TEXT NOT NULL,
	title TEXT NOT NULL,
	language TEXT NOT NULL,
	path TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_subtitles_torrent_file_id ON subtitles(torrent_file_id);

CREATE TABLE video_progress (
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	torrent_file_id TEXT NOT NULL REFERENCES torrent_files(id) ON DELETE CASCADE,
	position_seconds REAL NOT NULL DEFAULT 0 CHECK (position_seconds >= 0),
	duration_seconds REAL NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (user_id, torrent_file_id)
);

CREATE TABLE shares (
	id TEXT PRIMARY KEY,
	owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	slug TEXT NOT NULL UNIQUE,
	torrent_id TEXT REFERENCES torrents(id) ON DELETE CASCADE,
	torrent_file_id TEXT REFERENCES torrent_files(id) ON DELETE CASCADE,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CHECK (
		(torrent_id IS NOT NULL AND torrent_file_id IS NULL)
		OR (torrent_id IS NULL AND torrent_file_id IS NOT NULL)
	)
);

CREATE INDEX idx_shares_owner_user_id ON shares(owner_user_id);
CREATE INDEX idx_shares_slug_expires_at ON shares(slug, expires_at);

CREATE TABLE jobs (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'canceled')),
	payload_json TEXT NOT NULL DEFAULT '{}',
	attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
	max_attempts INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts > 0),
	lease_until TEXT,
	locked_by TEXT,
	last_error TEXT,
	available_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_jobs_claim ON jobs(status, available_at, lease_until);
CREATE INDEX idx_jobs_type ON jobs(type);

CREATE TABLE app_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE watch_parties (
	id TEXT PRIMARY KEY,
	owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	slug TEXT NOT NULL UNIQUE,
	torrent_file_id TEXT NOT NULL REFERENCES torrent_files(id) ON DELETE CASCADE,
	control_mode TEXT NOT NULL DEFAULT 'host_only' CHECK (control_mode IN ('host_only', 'everyone')),
	status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'ended')),
	current_position_seconds REAL NOT NULL DEFAULT 0 CHECK (current_position_seconds >= 0),
	duration_seconds REAL NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
	is_playing INTEGER NOT NULL DEFAULT 0 CHECK (is_playing IN (0, 1)),
	last_event_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_watch_parties_owner_user_id ON watch_parties(owner_user_id);
CREATE INDEX idx_watch_parties_slug ON watch_parties(slug);
CREATE INDEX idx_watch_parties_torrent_file_id ON watch_parties(torrent_file_id);
CREATE INDEX idx_watch_parties_status_expires_at ON watch_parties(status, expires_at);

CREATE TABLE watch_party_participants (
	id TEXT PRIMARY KEY,
	watch_party_id TEXT NOT NULL REFERENCES watch_parties(id) ON DELETE CASCADE,
	user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
	display_name TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'guest' CHECK (role IN ('host', 'guest')),
	token_hash TEXT NOT NULL UNIQUE,
	connected INTEGER NOT NULL DEFAULT 0 CHECK (connected IN (0, 1)),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_watch_party_participants_party_id ON watch_party_participants(watch_party_id);
CREATE INDEX idx_watch_party_participants_user_id ON watch_party_participants(user_id);
CREATE INDEX idx_watch_party_participants_connected ON watch_party_participants(watch_party_id, connected);

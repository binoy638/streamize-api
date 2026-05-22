CREATE TABLE catalog_items (
	id TEXT PRIMARY KEY,
	owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	media_type TEXT NOT NULL CHECK (media_type IN ('movie', 'tv', 'anime', 'unknown')),
	provider TEXT NOT NULL DEFAULT '',
	provider_id TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	original_title TEXT,
	overview TEXT,
	release_year INTEGER NOT NULL DEFAULT 0 CHECK (release_year >= 0),
	poster_url TEXT,
	backdrop_url TEXT,
	metadata_status TEXT NOT NULL DEFAULT 'matched' CHECK (metadata_status IN ('matched', 'manual', 'unmatched', 'failed')),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_catalog_items_provider ON catalog_items(owner_user_id, provider, provider_id) WHERE provider <> '' AND provider_id <> '';
CREATE INDEX idx_catalog_items_owner_user_id ON catalog_items(owner_user_id);
CREATE INDEX idx_catalog_items_media_type ON catalog_items(media_type);

CREATE TABLE catalog_episodes (
	id TEXT PRIMARY KEY,
	catalog_item_id TEXT NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
	provider TEXT NOT NULL DEFAULT '',
	provider_id TEXT NOT NULL DEFAULT '',
	season_number INTEGER NOT NULL DEFAULT 0 CHECK (season_number >= 0),
	episode_number INTEGER NOT NULL DEFAULT 0 CHECK (episode_number >= 0),
	absolute_number INTEGER NOT NULL DEFAULT 0 CHECK (absolute_number >= 0),
	title TEXT NOT NULL,
	overview TEXT,
	air_date TEXT,
	still_url TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_catalog_episodes_season_episode ON catalog_episodes(catalog_item_id, season_number, episode_number) WHERE season_number > 0 AND episode_number > 0;
CREATE UNIQUE INDEX idx_catalog_episodes_absolute ON catalog_episodes(catalog_item_id, absolute_number) WHERE absolute_number > 0;

ALTER TABLE torrent_files ADD COLUMN catalog_item_id TEXT REFERENCES catalog_items(id) ON DELETE SET NULL;
ALTER TABLE torrent_files ADD COLUMN catalog_episode_id TEXT REFERENCES catalog_episodes(id) ON DELETE SET NULL;
ALTER TABLE torrent_files ADD COLUMN metadata_status TEXT NOT NULL DEFAULT 'pending' CHECK (metadata_status IN ('pending', 'matched', 'manual', 'unmatched', 'failed'));
ALTER TABLE torrent_files ADD COLUMN metadata_confidence REAL NOT NULL DEFAULT 0 CHECK (metadata_confidence >= 0 AND metadata_confidence <= 1);
ALTER TABLE torrent_files ADD COLUMN metadata_provider TEXT;
ALTER TABLE torrent_files ADD COLUMN metadata_error TEXT;

CREATE INDEX idx_torrent_files_catalog_item_id ON torrent_files(catalog_item_id);
CREATE INDEX idx_torrent_files_catalog_episode_id ON torrent_files(catalog_episode_id);
CREATE INDEX idx_torrent_files_metadata_status ON torrent_files(metadata_status);

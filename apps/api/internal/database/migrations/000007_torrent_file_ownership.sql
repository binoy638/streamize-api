-- Add owner_user_id to torrent_files and make torrent_id nullable with SET NULL.
-- SQLite does not support ALTER COLUMN, so we recreate the table.

CREATE TABLE torrent_files_new (
    id TEXT PRIMARY KEY,
    torrent_id TEXT REFERENCES torrents(id) ON DELETE SET NULL,
    owner_user_id TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    ext TEXT NOT NULL DEFAULT '',
    original_path TEXT,
    hls_path TEXT,
    size_bytes INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('downloading', 'queued', 'processing', 'done', 'error')),
    progress_preview INTEGER NOT NULL DEFAULT 0 CHECK (progress_preview IN (0, 1)),
    transcoding_percent REAL NOT NULL DEFAULT 0 CHECK (transcoding_percent >= 0 AND transcoding_percent <= 100),
    download_percent REAL NOT NULL DEFAULT 0 CHECK (download_percent >= 0 AND download_percent <= 100),
    container TEXT NOT NULL DEFAULT '',
    video_codec TEXT NOT NULL DEFAULT '',
    audio_codec TEXT NOT NULL DEFAULT '',
    duration_seconds REAL NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
    processing_mode TEXT NOT NULL DEFAULT '',
    thumbnail_sheet_path TEXT,
    thumbnail_vtt_path TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO torrent_files_new
SELECT
    tf.id,
    tf.torrent_id,
    COALESCE(t.owner_user_id, ''),
    tf.slug,
    tf.name,
    tf.ext,
    tf.original_path,
    tf.hls_path,
    tf.size_bytes,
    tf.status,
    tf.progress_preview,
    tf.transcoding_percent,
    tf.download_percent,
    tf.container,
    tf.video_codec,
    tf.audio_codec,
    tf.duration_seconds,
    tf.processing_mode,
    tf.thumbnail_sheet_path,
    tf.thumbnail_vtt_path,
    tf.error_message,
    tf.created_at,
    tf.updated_at
FROM torrent_files tf
LEFT JOIN torrents t ON t.id = tf.torrent_id;

DROP TABLE torrent_files;
ALTER TABLE torrent_files_new RENAME TO torrent_files;

CREATE INDEX idx_torrent_files_torrent_id ON torrent_files(torrent_id);
CREATE INDEX idx_torrent_files_status ON torrent_files(status);
CREATE INDEX idx_torrent_files_owner_user_id ON torrent_files(owner_user_id);

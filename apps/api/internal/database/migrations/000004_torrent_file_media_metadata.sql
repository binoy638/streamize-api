ALTER TABLE torrent_files ADD COLUMN container TEXT NOT NULL DEFAULT '';
ALTER TABLE torrent_files ADD COLUMN video_codec TEXT NOT NULL DEFAULT '';
ALTER TABLE torrent_files ADD COLUMN audio_codec TEXT NOT NULL DEFAULT '';
ALTER TABLE torrent_files ADD COLUMN duration_seconds REAL NOT NULL DEFAULT 0;
ALTER TABLE torrent_files ADD COLUMN processing_mode TEXT NOT NULL DEFAULT '';

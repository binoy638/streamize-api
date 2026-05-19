ALTER TABLE torrent_files ADD COLUMN thumbnail_sheet_path TEXT;
ALTER TABLE torrent_files ADD COLUMN thumbnail_vtt_path TEXT;

CREATE UNIQUE INDEX idx_subtitles_torrent_file_path ON subtitles(torrent_file_id, path);

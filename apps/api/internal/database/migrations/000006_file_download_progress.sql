ALTER TABLE torrent_files ADD COLUMN download_percent REAL NOT NULL DEFAULT 0 CHECK (download_percent >= 0 AND download_percent <= 100);

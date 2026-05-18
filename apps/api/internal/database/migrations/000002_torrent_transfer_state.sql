ALTER TABLE torrents ADD COLUMN progress_percent REAL NOT NULL DEFAULT 0 CHECK (progress_percent >= 0 AND progress_percent <= 100);
ALTER TABLE torrents ADD COLUMN download_speed_bytes INTEGER NOT NULL DEFAULT 0 CHECK (download_speed_bytes >= 0);
ALTER TABLE torrents ADD COLUMN upload_speed_bytes INTEGER NOT NULL DEFAULT 0 CHECK (upload_speed_bytes >= 0);
ALTER TABLE torrents ADD COLUMN eta_seconds INTEGER NOT NULL DEFAULT -1 CHECK (eta_seconds >= -1);
ALTER TABLE torrents ADD COLUMN peers INTEGER NOT NULL DEFAULT 0 CHECK (peers >= 0);
ALTER TABLE torrents ADD COLUMN ratio REAL NOT NULL DEFAULT 0 CHECK (ratio >= 0);

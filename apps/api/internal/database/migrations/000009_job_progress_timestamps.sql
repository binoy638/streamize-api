ALTER TABLE jobs ADD COLUMN progress_percent REAL NOT NULL DEFAULT 0 CHECK (progress_percent >= 0 AND progress_percent <= 100);
ALTER TABLE jobs ADD COLUMN started_at TEXT;
ALTER TABLE jobs ADD COLUMN finished_at TEXT;

CREATE INDEX idx_jobs_status_updated_at ON jobs(status, updated_at);

ALTER TABLE jobs ADD COLUMN dedupe_key TEXT;

CREATE UNIQUE INDEX idx_jobs_dedupe_key ON jobs(dedupe_key) WHERE dedupe_key IS NOT NULL;

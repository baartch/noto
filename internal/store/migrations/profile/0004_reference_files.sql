ALTER TABLE messages
    ADD COLUMN reference_influenced INTEGER NOT NULL DEFAULT 0
        CHECK (reference_influenced IN (0, 1));

CREATE TABLE reference_files (
    id                  TEXT PRIMARY KEY,
    profile_id          TEXT NOT NULL,
    relative_path       TEXT NOT NULL,
    media_type          TEXT NOT NULL DEFAULT '',
    fingerprint         TEXT NOT NULL DEFAULT '',
    summary_fingerprint TEXT NOT NULL DEFAULT '',
    summary             TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL CHECK (status IN ('pending', 'ready', 'failed')),
    size_bytes          INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
    mod_time_ns         INTEGER NOT NULL DEFAULT 0,
    last_error          TEXT NOT NULL DEFAULT '',
    indexed_at          DATETIME,
    UNIQUE (profile_id, relative_path)
);

CREATE INDEX idx_reference_files_profile_path
    ON reference_files(profile_id, relative_path);

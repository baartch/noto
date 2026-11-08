-- Restore the cache table for profiles whose initial migration was recorded
-- without the table. Existing cache entries are preserved.
CREATE TABLE IF NOT EXISTS context_cache (
    id              TEXT PRIMARY KEY,
    profile_id      TEXT NOT NULL,
    cache_key       TEXT NOT NULL,
    payload         TEXT NOT NULL DEFAULT '',
    source_note_ids TEXT NOT NULL DEFAULT '[]',
    prompt_version  TEXT NOT NULL DEFAULT '',
    state_version   TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    expires_at      DATETIME,
    UNIQUE (profile_id, cache_key)
);

CREATE INDEX IF NOT EXISTS idx_context_cache_profile_id ON context_cache(profile_id);

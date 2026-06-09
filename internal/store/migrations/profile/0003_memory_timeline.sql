-- profile/0003_memory_timeline.sql
-- Add timeline settings and weekly/monthly summary artifacts.

CREATE TABLE IF NOT EXISTS settings (
    profile_id   TEXT NOT NULL,
    key          TEXT NOT NULL,
    value        TEXT NOT NULL DEFAULT '',
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (profile_id, key)
);

CREATE INDEX IF NOT EXISTS idx_settings_profile_id
    ON settings (profile_id);

CREATE TABLE IF NOT EXISTS memory_summaries (
    id                   TEXT PRIMARY KEY,
    profile_id           TEXT NOT NULL,
    summary_type         TEXT NOT NULL CHECK (summary_type IN ('weekly','monthly')),
    period_key           TEXT NOT NULL,
    period_start         DATETIME NOT NULL,
    period_end           DATETIME NOT NULL,
    content              TEXT NOT NULL DEFAULT '',
    source_state_version TEXT NOT NULL DEFAULT '',
    freshness_state      TEXT NOT NULL DEFAULT 'fresh' CHECK (freshness_state IN ('fresh','stale','regenerating')),
    created_at           DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at           DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (profile_id, summary_type, period_key)
);

CREATE INDEX IF NOT EXISTS idx_memory_summaries_profile_type_start
    ON memory_summaries (profile_id, summary_type, period_start DESC);

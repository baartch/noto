-- ============================================================
-- system_prompts
-- ============================================================
CREATE TABLE IF NOT EXISTS system_prompts (
    id          TEXT PRIMARY KEY,
    profile_id  TEXT NOT NULL UNIQUE,
    prompt      TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_system_prompts_profile_id ON system_prompts(profile_id);

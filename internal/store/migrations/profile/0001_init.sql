-- profile/0001_init.sql
-- Per-profile DB: all profile-scoped data.

-- ============================================================
-- conversations
-- ============================================================
CREATE TABLE IF NOT EXISTS conversations (
    id          TEXT PRIMARY KEY,
    profile_id  TEXT NOT NULL,
    started_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    ended_at    DATETIME,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_conversations_profile_id ON conversations(profile_id);

-- ============================================================
-- messages
-- ============================================================
CREATE TABLE IF NOT EXISTS messages (
    id              TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content         TEXT NOT NULL,
    provider        TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);

-- ============================================================
-- memory_notes
-- ============================================================
CREATE TABLE IF NOT EXISTS memory_notes (
    id                  TEXT PRIMARY KEY,
    profile_id          TEXT NOT NULL,
    conversation_id     TEXT,
    category            TEXT NOT NULL DEFAULT 'fact'
                            CHECK (category IN ('fact','progress','blocker','action_item','other')),
    content             TEXT NOT NULL,
    importance          INTEGER NOT NULL DEFAULT 5 CHECK (importance BETWEEN 1 AND 10),
    source_message_ids  TEXT NOT NULL DEFAULT '[]',
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_memory_notes_profile_id ON memory_notes(profile_id);

-- ============================================================
-- session_summaries
-- ============================================================
CREATE TABLE IF NOT EXISTS session_summaries (
    id              TEXT PRIMARY KEY,
    profile_id      TEXT NOT NULL,
    conversation_id TEXT,
    summary_text    TEXT NOT NULL,
    open_loops      TEXT NOT NULL DEFAULT '[]',
    next_actions    TEXT NOT NULL DEFAULT '[]',
    created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_session_summaries_profile_id ON session_summaries(profile_id);

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

-- ============================================================
-- provider_config
-- ============================================================
CREATE TABLE IF NOT EXISTS provider_config (
    id             TEXT PRIMARY KEY,
    profile_id     TEXT NOT NULL,
    provider_type  TEXT NOT NULL,
    endpoint       TEXT NOT NULL DEFAULT '',
    model          TEXT NOT NULL DEFAULT '',
    active_model   TEXT NOT NULL DEFAULT '',
    credential_ref TEXT NOT NULL DEFAULT '',
    is_active      INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
    created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at     DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_provider_config_profile_id ON provider_config(profile_id);

-- ============================================================
-- context_cache
-- ============================================================
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

-- ============================================================
-- vector_index_entries
-- ============================================================
CREATE TABLE IF NOT EXISTS vector_index_entries (
    id              TEXT PRIMARY KEY,
    profile_id      TEXT NOT NULL,
    source_type     TEXT NOT NULL CHECK (source_type IN ('memory_note','session_summary','message')),
    source_id       TEXT NOT NULL,
    chunk_hash      TEXT NOT NULL,
    embedding_model TEXT NOT NULL DEFAULT '',
    embedding_dim   INTEGER NOT NULL DEFAULT 0,
    vector_ref      TEXT NOT NULL DEFAULT '',
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (profile_id, source_type, source_id, chunk_hash)
);

CREATE INDEX IF NOT EXISTS idx_vector_entries_profile_id ON vector_index_entries(profile_id);

-- ============================================================
-- vector_index_manifest
-- ============================================================
CREATE TABLE IF NOT EXISTS vector_index_manifest (
    id                   TEXT PRIMARY KEY,
    profile_id           TEXT NOT NULL UNIQUE,
    index_path           TEXT NOT NULL DEFAULT '',
    index_format_version TEXT NOT NULL DEFAULT '1',
    embedding_model      TEXT NOT NULL DEFAULT '',
    embedding_dim        INTEGER NOT NULL DEFAULT 0,
    last_rebuild_at      DATETIME,
    last_sync_at         DATETIME,
    source_state_version TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'ready'
                             CHECK (status IN ('ready','stale','rebuilding','failed'))
);

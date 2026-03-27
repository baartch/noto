# Data Model: Noto Profile Memory CLI

## Entity: Profile

- **Purpose**: Isolated consulting context.
- **Fields**:
  - `id` (UUID/string, PK)
  - `name` (string, unique)
  - `slug` (filesystem-safe unique key)
  - `system_prompt_path` (string)
  - `db_path` (string)
  - `is_default` (bool)
  - `created_at`, `updated_at` (timestamp)
- **Validation**:
  - Name required, trimmed, unique.
  - Slug deterministic and collision-safe.
- **Relationships**:
  - 1:N with Conversation, MemoryNote, SessionSummary, ContextCacheEntry,
    ProviderConfig, BackupSnapshot.

## Entity: Conversation

- **Purpose**: One chat session in one profile.
- **Fields**:
  - `id` (UUID/string, PK)
  - `profile_id` (FK)
  - `started_at`, `ended_at` (timestamp)
  - `status` (active, archived)
- **Validation**:
  - Must reference existing profile.
  - `ended_at >= started_at`.

## Entity: Message

- **Purpose**: Ordered turns inside a conversation.
- **Fields**:
  - `id` (UUID/string, PK)
  - `conversation_id` (FK)
  - `role` (user, assistant, system)
  - `content` (text)
  - `provider` (string)
  - `model` (string)
  - `created_at` (timestamp)
- **Validation**:
  - Role enumerated.
  - Content non-empty.

## Entity: MemoryNote

- **Purpose**: Durable extracted continuity knowledge.
- **Fields**:
  - `id` (UUID/string, PK)
  - `profile_id` (FK)
  - `conversation_id` (FK, nullable)
  - `category` (fact, progress, blocker, action_item)
  - `content` (text)
  - `importance` (int 1..5)
  - `source_message_ids` (json/text)
  - `created_at`, `updated_at` (timestamp)
- **Validation**:
  - Category enumerated.
  - Content required.
  - Importance bounded.
- **Indexes**:
  - FTS on content.
  - `(profile_id, category, updated_at)` composite.

## Entity: SessionSummary

- **Purpose**: Compact handoff context for next session bootstrap.
- **Fields**:
  - `id` (UUID/string, PK)
  - `profile_id` (FK)
  - `conversation_id` (FK)
  - `summary_text` (text)
  - `open_loops` (json/text)
  - `next_actions` (json/text)
  - `created_at` (timestamp)
- **Validation**:
  - One summary per conversation.
  - Bounded summary length.

## Entity: ContextCacheEntry

- **Purpose**: Reusable assembled context artifact (performance layer only).
- **Fields**:
  - `id` (UUID/string, PK)
  - `profile_id` (FK)
  - `cache_key` (string)
  - `payload` (json/text)
  - `source_note_ids` (json/text)
  - `prompt_version` (hash/string)
  - `state_version` (int)
  - `created_at`, `expires_at` (timestamp)
- **Validation**:
  - Valid only if prompt/version/source set matches current profile state.
- **Indexes**:
  - unique `(profile_id, cache_key)`.

## Entity: ProviderConfig

- **Purpose**: Per-profile provider configuration.
- **Fields**:
  - `id` (UUID/string, PK)
  - `profile_id` (FK)
  - `provider_type` (github_models, openrouter, local_compatible, openai_compatible)
  - `endpoint` (string)
  - `model` (string)
  - `credential_ref` (string)
  - `is_active` (bool)
  - `created_at`, `updated_at` (timestamp)
- **Validation**:
  - Exactly one active config/profile at runtime.
  - Endpoint/model required for active config.

## Entity: CredentialSecret

- **Purpose**: Encrypted provider credential material.
- **Fields**:
  - `id` (UUID/string, PK)
  - `profile_id` (FK)
  - `provider_type` (string)
  - `ciphertext` (blob/text)
  - `key_id` (string)
  - `created_at`, `rotated_at` (timestamp)
- **Validation**:
  - Never persisted in plaintext.
  - Must remain profile-scoped.

## Entity: BackupSnapshot

- **Purpose**: Recovery restore point for profile DB.
- **Fields**:
  - `id` (UUID/string, PK)
  - `profile_id` (FK)
  - `snapshot_path` (string)
  - `trigger` (periodic, session_end, manual_repair_pre)
  - `created_at` (timestamp)
  - `checksum` (string)
- **Validation**:
  - Snapshot path must remain inside profile-local storage root.
  - Checksum required for restore eligibility.

## Entity: ObservabilityEvent

- **Purpose**: Structured local record for key runtime outcomes.
- **Fields**:
  - `id` (UUID/string, PK)
  - `profile_id` (FK, nullable for startup pre-selection)
  - `event_type` (startup, retrieval, cache, provider_call, recovery, command)
  - `status` (success, failure)
  - `latency_ms` (int, nullable)
  - `metadata` (json/text)
  - `created_at` (timestamp)
- **Validation**:
  - Must not include decrypted secrets.
  - Metadata must be bounded in size.

## State Transitions

- **Profile**: created → selected/defaulted → renamed (optional) → deleted (confirmed).
- **Conversation**: active → archived.
- **ContextCacheEntry**: created → reused → invalidated (prompt/memory/profile change) → rebuilt.
- **BackupSnapshot**: created (periodic/session_end) → eligible for restore → retained/pruned.
- **Corruption recovery**: detect corruption → auto-repair attempt → if fail restore latest valid
  snapshot → continue with user notice.

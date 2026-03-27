# Data Model: Noto Profile Memory CLI

## Entity: Profile

- **Purpose**: Isolated consulting context.
- **Fields**: `id`, `name`, `slug`, `system_prompt_path`, `db_path`, `is_default`, timestamps.
- **Validation**: name unique + non-empty; slug collision-safe.

## Entity: Conversation

- **Purpose**: One chat session in one profile.
- **Fields**: `id`, `profile_id`, `started_at`, `ended_at`, `status`.

## Entity: Message

- **Purpose**: Ordered turns inside conversation.
- **Fields**: `id`, `conversation_id`, `role`, `content`, `provider`, `model`, `created_at`.

## Entity: MemoryNote

- **Purpose**: Durable continuity knowledge.
- **Fields**: `id`, `profile_id`, `conversation_id`, `category`, `content`, `importance`,
  `source_message_ids`, timestamps.

## Entity: SessionSummary

- **Purpose**: Compact handoff context for next session.
- **Fields**: `id`, `profile_id`, `conversation_id`, `summary_text`, `open_loops`,
  `next_actions`, `created_at`.

## Entity: ContextCacheEntry

- **Purpose**: Reusable assembled context artifact.
- **Fields**: `id`, `profile_id`, `cache_key`, `payload`, `source_note_ids`, `prompt_version`,
  `state_version`, `created_at`, `expires_at`.

## Entity: ProviderConfig

- **Purpose**: Per-profile provider configuration.
- **Fields**: `id`, `profile_id`, `provider_type`, `endpoint`, `model`, `credential_ref`,
  `is_active`, timestamps.

## Entity: SlashCommand

- **Purpose**: Canonical command metadata for CLI/slash parity.
- **Fields**:
  - `path` (string, unique canonical hierarchical form, e.g., `profile list`)
  - `usage` (string)
  - `description` (string)
  - `aliases` (json/text)
  - `requires_confirmation` (bool)
  - `scope` (profile/global)
- **Validation**:
  - Canonical path unique.
  - Alias conflicts prohibited.

## Entity: SlashSuggestion

- **Purpose**: In-memory suggestion candidate payload for active input.
- **Fields**:
  - `input_prefix` (string)
  - `candidate_path` (string)
  - `rank` (int)
  - `hint` (string)

## Entity: ObservabilityEvent

- **Purpose**: Structured local runtime event.
- **Fields**:
  - `event_type` (startup, retrieval, cache, provider_call, recovery, slash_parse,
    slash_suggest, slash_execute)
  - `profile_id` (nullable)
  - `status` (success/failure)
  - `latency_ms` (nullable)
  - `metadata` (json/text)
  - `created_at`

## State Transitions

- Profile: created → selected/defaulted → renamed → deleted (confirmed).
- Conversation: active → archived.
- ContextCacheEntry: created → reused → invalidated → rebuilt.
- Slash input: plain chat → slash mode (`/`) → suggest/update → explicit selection (if ambiguous)
  → execute or explicit unknown-command error.
